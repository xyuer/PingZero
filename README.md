# 完整架构方案

## 整体架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           整体架构                                           │
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │
│  │  Handle B   │    │  Handle A   │    │  Handle C   │    │  Handle D   │ │
│  │  DNS劫持    │    │  TCP/UDP    │    │  ICMP       │    │  入方向回包  │ │
│  │  优先级最高  │    │  出方向     │    │  出方向     │    │             │ │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘ │
│         │                  │                  │                  │        │
│         └──────────────────┴──────────────────┴──────────────────┘        │
│                                    │                                       │
│                          ┌─────────▼─────────┐                            │
│                          │   核心状态管理      │                            │
│                          │                   │                            │
│                          │  - PID监控表       │                            │
│                          │  - DNS映射表       │                            │
│                          │  - 连接跟踪表      │                            │
│                          │  - 已知IP表        │  ← ICMP动态维护            │
│                          │  - 排除规则集      │                            │
│                          └─────────┬─────────┘                            │
│                                    │                                       │
│                          ┌─────────▼─────────┐                            │
│                          │     隧道管理        │                            │
│                          │  本地代理端口       │                            │
│                          └─────────┬─────────┘                            │
│                                    │                                       │
│                          ┌─────────▼─────────┐                            │
│                          │   加速节点          │                            │
│                          └───────────────────┘                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 核心状态管理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           核心数据结构                                       │
└─────────────────────────────────────────────────────────────────────────────┘

// ─────────────────────────────────────────────────────────
// 1. PID 监控表
// ─────────────────────────────────────────────────────────
PIDTable {
    entries map[uint32]*PIDEntry    // PID → Entry
    mu      sync.RWMutex
}

PIDEntry {
    PID         uint32
    ProcessName string              // "League of Legends.exe"
    GameID      string              // "lol"
    StartTime   time.Time
    BypassRules *BypassRuleSet      // 该游戏的排除规则
    KnownIPs    *KnownIPTable       // 该进程已知连接IP表 ← ICMP用
}


// ─────────────────────────────────────────────────────────
// 2. DNS 映射表
// ─────────────────────────────────────────────────────────
DNSNATTable {
    fakeToReal map[string]string    // "198.18.1.1" → "119.28.1.1"
    realToFake map[string]string    // "119.28.1.1" → "198.18.1.1"
    domainToFake map[string]string  // "game.server.com" → "198.18.1.1"
    mu         sync.RWMutex
    nextFakeIP uint32               // 分配游标
}


// ─────────────────────────────────────────────────────────
// 3. 连接跟踪表
// ─────────────────────────────────────────────────────────
ConnTrackTable {
    entries map[ConnKey]*ConnEntry
    mu      sync.RWMutex
}

ConnKey {
    Protocol uint8
    SrcIP    [4]byte
    SrcPort  uint16
    DstIP    [4]byte    // 可能是虚假IP或真实IP
    DstPort  uint16
}

ConnEntry {
    Key         ConnKey
    RealDstIP   net.IP   // 真实目标IP（隧道转发用）
    RealDstPort uint16
    PID         uint32
    TunnelPort  uint16   // 本地隧道分配的端口
    State       ConnState
    LastSeen    time.Time
    IsFakeIP    bool     // 是否经过DNS劫持
}


// ─────────────────────────────────────────────────────────
// 4. 已知IP表（ICMP动态维护核心）
// ─────────────────────────────────────────────────────────
KnownIPTable {
    ips  map[string]*KnownIPEntry   // IP字符串 → Entry
    mu   sync.RWMutex
}

KnownIPEntry {
    IP          net.IP
    FirstSeen   time.Time
    LastSeen    time.Time
    ConnCount   int         // 该IP的连接数量
    IsFakeIP    bool        // 是否是虚假IP（DNS劫持）
    RealIP      net.IP      // 对应的真实IP（IsFakeIP=true时有效）
}

方法:
    Add(ip net.IP, realIP net.IP)   // 新增已知IP
    Contains(ip net.IP) bool        // 检查IP是否已知
    Remove(ip net.IP)               // 移除（连接全部断开后）
    BuildWinDivertFilter() string   // 动态生成WinDivert过滤规则


// ─────────────────────────────────────────────────────────
// 5. 排除规则集
// ─────────────────────────────────────────────────────────
BypassRuleSet {
    rules []BypassRule
}

BypassRule {
    Protocol  uint8       // 0=任意  6=TCP  17=UDP
    DstIPNets []*net.IPNet
    DstPorts  []uint16
    Comment   string
}

func (r *BypassRule) Match(protocol uint8, dstIP net.IP, dstPort uint16) bool {
    // 协议检查
    if r.Protocol != 0 && r.Protocol != protocol {
        return false
    }
    // IP检查
    if len(r.DstIPNets) > 0 {
        matched := false
        for _, ipnet := range r.DstIPNets {
            if ipnet.Contains(dstIP) {
                matched = true
                break
            }
        }
        if !matched { return false }
    }
    // 端口检查
    if len(r.DstPorts) > 0 {
        matched := false
        for _, p := range r.DstPorts {
            if p == dstPort {
                matched = true
                break
            }
        }
        if !matched { return false }
    }
    return true
}
```

---

## Handle B：DNS 劫持

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Handle B：DNS 劫持                                 │
└─────────────────────────────────────────────────────────────────────────────┘

WinDivert 过滤规则:
─────────────────────────────────────────────────────────────
  "udp.DstPort == 53 and outbound"

处理流程:
─────────────────────────────────────────────────────────────

  收到 DNS 查询包
        │
        ▼
  解析 DNS 报文
  获取查询域名: "game.server.com"
        │
        ▼
  检查 ProcessId 是否是目标游戏进程
        │
        ├── 否 ──────────────────────────────────────────► WinDivertSend 放行
        │
        ▼ 是目标进程
  检查域名是否需要加速
  （查游戏配置的加速域名列表 / 全局加速规则）
        │
        ├── 不需要加速 ──────────────────────────────────► WinDivertSend 放行
        │
        ▼ 需要加速
  检查 DNSNATTable 是否已有映射
        │
        ├── 已有 → 直接使用已有虚假IP
        │
        ▼ 没有
  分配新虚假IP: 198.18.x.x
  写入 DNSNATTable:
    fakeToReal["198.18.1.1"] = "119.28.1.1"（DNS解析后）
    domainToFake["game.server.com"] = "198.18.1.1"
        │
        ▼
  构造 DNS 响应包
  将真实IP替换为虚假IP 198.18.1.1
  直接回复给游戏进程（不转发给真实DNS服务器）
        │
        ▼
  WinDivertSend 注入响应包

注意:
  真实DNS解析在后台异步完成:
  后台协程 → 查询真实DNS → 得到真实IP
  → 更新 DNSNATTable fakeToReal 映射
  → 如果已有连接在等待 → 通知更新
```

---

## Handle A：TCP/UDP 出方向

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Handle A：TCP/UDP 出方向处理                            │
└─────────────────────────────────────────────────────────────────────────────┘

WinDivert 过滤规则:
─────────────────────────────────────────────────────────────
  "(tcp or udp) and outbound and !loopback"

处理流程:
─────────────────────────────────────────────────────────────

  收到出方向 TCP/UDP 包
        │
        ▼
  解析包头
  Protocol, SrcIP, SrcPort, DstIP, DstPort, ProcessId
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │  Step 1: PID 检查                                   │
  └─────────────────────────────────────────────────────┘
        │
        ├── 非目标进程 ───────────────────────────────► WinDivertSend 放行
        │
        ▼ 是目标进程，获取 PIDEntry
  ┌─────────────────────────────────────────────────────┐
  │  Step 2: 排除规则检查                               │
  │                                                     │
  │  检查顺序:                                          │
  │  1. 强制放行规则（内置）                             │
  │  2. 游戏专属排除规则（PIDEntry.BypassRules）         │
  │  3. 全局用户排除规则                                 │
  └─────────────────────────────────────────────────────┘
        │
        ├── 命中排除规则 ─────────────────────────────► WinDivertSend 放行
        │
        ▼ 未命中，需要加速
  ┌─────────────────────────────────────────────────────┐
  │  Step 3: 解析真实目标IP                             │
  │                                                     │
  │  if isFakeIP(DstIP):                                │
  │      RealDstIP = DNSNATTable.FakeToReal(DstIP)      │
  │      IsFakeIP  = true                               │
  │  else:                                              │
  │      RealDstIP = DstIP   // 直接IP连接              │
  │      IsFakeIP  = false                              │
  └─────────────────────────────────────────────────────┘
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │  Step 4: 更新 KnownIPTable（ICMP动态维护）           │
  │                                                     │
  │  pidEntry.KnownIPs.Add(DstIP, RealDstIP)            │
  │                                                     │
  │  DstIP 可能是:                                      │
  │    - 198.18.1.1（虚假IP）→ 记录虚假IP+真实IP映射    │
  │    - 119.28.1.1（真实IP）→ 记录真实IP               │
  │                                                     │
  │  → 触发 Handle C 过滤规则更新                       │
  └─────────────────────────────────────────────────────┘
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │  Step 5: 连接跟踪                                   │
  │                                                     │
  │  Key = {Protocol, SrcIP, SrcPort, DstIP, DstPort}  │
  │  查 ConnTrackTable                                  │
  └─────────────────────────────────────────────────────┘
        │
        ├── 已存在连接 → 使用已有 TunnelPort
        │
        ▼ 新连接
  分配 TunnelPort
  写入 ConnTrackTable:
    ConnEntry {
        RealDstIP:   RealDstIP
        RealDstPort: DstPort
        PID:         ProcessId
        TunnelPort:  分配的本地端口
        IsFakeIP:    IsFakeIP
    }
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │  Step 6: 修改包并转发到隧道                          │
  │                                                     │
  │  修改 DstIP   → 127.0.0.1（本地隧道监听地址）        │
  │  修改 DstPort → TunnelPort                          │
  │  重新计算校验和                                      │
  │  WinDivertSend 注入                                 │
  └─────────────────────────────────────────────────────┘
        │
        ▼
  本地隧道进程收到包
  读取 ConnTrackTable 获取 RealDstIP
  封装后发往加速节点
```

---

## Handle C：ICMP 出方向（动态过滤规则）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Handle C：ICMP 出方向（动态维护）                        │
└─────────────────────────────────────────────────────────────────────────────┘

核心设计: WinDivert 过滤规则动态更新
─────────────────────────────────────────────────────────────

  KnownIPTable 变化时 → 重新生成过滤规则 → 重启 Handle C

  过滤规则生成逻辑:
  ┌──────────────────────────────────────────────────────────────────────┐
  │                                                                      │
  │  func BuildICMPFilter(knownIPs []net.IP) string {                    │
  │                                                                      │
  │      if len(knownIPs) == 0 {                                         │
  │          return "false"  // 没有已知IP，不拦截任何ICMP               │
  │      }                                                               │
  │                                                                      │
  │      conditions := []string{}                                        │
  │      for _, ip := range knownIPs {                                   │
  │          conditions = append(conditions,                             │
  │              fmt.Sprintf("ip.DstAddr == %s", ip.String()))           │
  │      }                                                               │
  │                                                                      │
  │      return fmt.Sprintf(                                             │
  │          "icmp and outbound and (%s)",                               │
  │          strings.Join(conditions, " or "),                           │
  │      )                                                               │
  │  }                                                                   │
  │                                                                      │
  │  生成结果示例:                                                        │
  │  "icmp and outbound and (                                            │
  │      ip.DstAddr == 119.28.1.1 or                                     │
  │      ip.DstAddr == 47.52.88.100 or                                   │
  │      ip.DstAddr == 198.18.1.1                                        │
  │  )"                                                                  │
  │                                                                      │
  └──────────────────────────────────────────────────────────────────────┘


Handle C 处理流程:
─────────────────────────────────────────────────────────────

  收到 ICMP 包（已被过滤规则筛选，DstIP 一定在 KnownIPTable 中）
        │
        ▼
  解析 ICMP 包
  获取 DstIP, ProcessId
        │
        ▼
  检查 ProcessId 是否是目标进程
        │
        ├── 否 ──────────────────────────────────────────► WinDivertSend 放行
        │
        ▼ 是目标进程
  ┌─────────────────────────────────────────────────────┐
  │  解析真实目标IP                                      │
  │                                                     │
  │  if isFakeIP(DstIP):                                │
  │      RealDstIP = DNSNATTable.FakeToReal(DstIP)      │
  │  else:                                              │
  │      RealDstIP = DstIP                              │
  └─────────────────────────────────────────────────────┘
        │
        ▼
  封装 ICMP 包发往隧道
  隧道节点 → 向 RealDstIP 发送 ICMP
  收到响应 → 通过 Handle D 回传给游戏进程


Handle C 重启机制:
─────────────────────────────────────────────────────────────

  KnownIPTable 变化事件:
    - 新增IP → 触发规则更新
    - 移除IP → 触发规则更新

  更新流程:
  ┌──────────────────────────────────────────────────────────────────────┐
  │                                                                      │
  │  func UpdateHandleC() {                                              │
  │      newFilter := BuildICMPFilter(knownIPs.All())                   │
  │                                                                      │
  │      if newFilter == currentFilter {                                 │
  │          return  // 规则未变化，不需要重启                            │
  │      }                                                               │
  │                                                                      │
  │      // 关闭旧 Handle C                                              │
  │      oldHandle.Close()                                               │
  │                                                                      │
  │      // 用新规则打开新 Handle C                                       │
  │      newHandle = WinDivertOpen(newFilter, ...)                       │
  │      currentFilter = newFilter                                       │
  │                                                                      │
  │      // 启动新的处理 goroutine                                        │
  │      go handleCLoop(newHandle)                                       │
  │  }                                                                   │
  │                                                                      │
  │  注意: 更新操作需要加锁，防止并发重启                                  │
  │        使用 sync.Mutex 保护 Handle C 的生命周期                       │
  └──────────────────────────────────────────────────────────────────────┘


防抖动机制:
─────────────────────────────────────────────────────────────
  游戏启动时可能短时间内建立大量连接
  → 频繁触发 Handle C 重启 → 性能问题

  解决方案: 防抖动 (Debounce)

  KnownIPTable.Add() 被调用
        │
        ▼
  发送信号到 updateChan
        │
        ▼
  updateWorker goroutine:
    等待 100ms 内没有新信号
        │
        ▼
    执行 UpdateHandleC()

  效果: 批量新增IP时只触发一次重启
```

---

## Handle D：入方向回包处理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Handle D：入方向回包处理                                 │
└─────────────────────────────────────────────────────────────────────────────┘

WinDivert 过滤规则:
─────────────────────────────────────────────────────────────
  "(tcp or udp or icmp) and inbound and !loopback"

处理流程:
─────────────────────────────────────────────────────────────

  收到入方向包
        │
        ▼
  解析包头
  Protocol, SrcIP, SrcPort, DstIP, DstPort
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │  查连接跟踪表                                        │
  │                                                     │
  │  反向Key = {Protocol, DstIP, DstPort, SrcIP, SrcPort}│
  │  (入方向: Src和Dst对调查找出方向的记录)              │
  └─────────────────────────────────────────────────────┘
        │
        ├── 未找到 ───────────────────────────────────► WinDivertSend 放行
        │
        ▼ 找到 ConnEntry
  ┌─────────────────────────────────────────────────────┐
  │  判断是否需要还原IP                                  │
  │                                                     │
  │  if ConnEntry.IsFakeIP:                             │
  │      // 出方向用了虚假IP，回包SrcIP是真实IP          │
  │      // 需要将 SrcIP 替换回虚假IP                    │
  │      // 让游戏认为是从虚假IP收到的回包               │
  │      SrcIP = DNSNATTable.RealToFake(SrcIP)          │
  │  else:                                              │
  │      // 直接IP连接，不需要替换                       │
  │      // SrcIP 保持真实IP不变                         │
  └─────────────────────────────────────────────────────┘
        │
        ▼
  更新 ConnEntry.LastSeen
        │
        ▼
  重新计算校验和
  WinDivertSend 注入给游戏进程
```

---

## KnownIPTable 生命周期管理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      KnownIPTable 生命周期管理                               │
└─────────────────────────────────────────────────────────────────────────────┘

IP 新增时机:
─────────────────────────────────────────────────────────────
  Handle A 处理出方向包时:
    → 目标进程 + 未命中排除规则
    → pidEntry.KnownIPs.Add(DstIP, RealDstIP)
    → 触发 Handle C 规则更新


IP 移除时机:
─────────────────────────────────────────────────────────────
  ConnTrackTable 清理时:
    → 某个IP的所有连接都超时/关闭
    → 检查 KnownIPTable 中该IP的 ConnCount
    → ConnCount == 0 → 移除该IP
    → 触发 Handle C 规则更新

  游戏进程退出时:
    → PID 从 PIDTable 移除
    → 清空该 PID 的所有 ConnTrack 条目
    → 清空该 PID 的 KnownIPTable
    → 触发 Handle C 规则更新


KnownIPTable 与 ConnTrackTable 的关联:
─────────────────────────────────────────────────────────────

  ConnTrackTable 清理 goroutine（每30秒运行一次）:

  for each ConnEntry in ConnTrackTable:
      if time.Since(entry.LastSeen) > 5*time.Minute:
          ConnTrackTable.Remove(entry.Key)
          
          // 检查该IP是否还有其他连接
          remainingConns := ConnTrackTable.CountByDstIP(entry.DstIP)
          if remainingConns == 0:
              pidEntry.KnownIPs.Remove(entry.DstIP)
              // 触发 Handle C 规则更新


KnownIPTable 数据示意:
─────────────────────────────────────────────────────────────

  游戏进程 PID=1234 的 KnownIPTable:

  ┌──────────────────┬──────────────────┬──────────┬──────────────────────┐
  │  记录IP           │  真实IP           │ ConnCount│  备注                │
  ├──────────────────┼──────────────────┼──────────┼──────────────────────┤
  │  198.18.1.1      │  119.28.1.1      │    3     │  DNS劫持的虚假IP      │
  │  198.18.1.2      │  47.52.88.100    │    1     │  DNS劫持的虚假IP      │
  │  103.24.77.1     │  103.24.77.1     │    2     │  直接IP连接           │
  └──────────────────┴──────────────────┴──────────┴──────────────────────┘

  BuildWinDivertFilter() 生成:
  "icmp and outbound and (
      ip.DstAddr == 198.18.1.1 or
      ip.DstAddr == 198.18.1.2 or
      ip.DstAddr == 103.24.77.1
  )"
```

---

## 完整数据流示例

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    场景1: DNS劫持 + 游戏战斗连接                              │
└─────────────────────────────────────────────────────────────────────────────┘

1. 游戏查询 DNS: "game.server.com"
   Handle B 拦截
   → 分配虚假IP 198.18.1.1
   → DNSNATTable: 198.18.1.1 → 119.28.1.1
   → 返回 198.18.1.1 给游戏

2. 游戏建立连接: UDP 198.18.1.1:7000
   Handle A 拦截
   → PID✓ → 排除规则✗
   → isFakeIP(198.18.1.1) = true
   → RealDstIP = 119.28.1.1
   → KnownIPs.Add(198.18.1.1, 119.28.1.1)  ← 触发Handle C更新
   → ConnTrack: {UDP,本地IP,54321,198.18.1.1,7000} → {119.28.1.1,7000}
   → 修改DstIP→127.0.0.1:TunnelPort → 发往本地隧道

3. 本地隧道收到包
   → 查ConnTrack → RealDstIP=119.28.1.1
   → 封装发往加速节点
   → 节点转发到 119.28.1.1:7000

4. Handle C 收到更新信号（防抖100ms后）
   → 新过滤规则: "icmp and outbound and ip.DstAddr == 198.18.1.1"
   → 重启 Handle C

5. 游戏发送 ICMP ping 到 198.18.1.1
   Handle C 拦截（新规则命中）
   → PID✓
   → isFakeIP(198.18.1.1) = true → RealDstIP = 119.28.1.1
   → 封装ICMP发往隧道 → 节点ping 119.28.1.1
   → 响应回来 → Handle D 处理 → SrcIP还原为198.18.1.1 → 返回游戏


┌─────────────────────────────────────────────────────────────────────────────┐
│                    场景2: 直接IP连接（无DNS）                                 │
└─────────────────────────────────────────────────────────────────────────────┘

1. 游戏直接连接: UDP 119.28.1.1:7000
   Handle A 拦截
   → PID✓ → 排除规则✗
   → isFakeIP(119.28.1.1) = false
   → RealDstIP = 119.28.1.1（直接使用）
   → KnownIPs.Add(119.28.1.1, nil)  ← 触发Handle C更新
   → ConnTrack: {UDP,本地IP,54321,119.28.1.1,7000} → {119.28.1.1,7000}
   → 修改DstIP→127.0.0.1:TunnelPort → 发往本地隧道

2. Handle C 更新
   → 新过滤规则: "icmp and outbound and ip.DstAddr == 119.28.1.1"

3. 游戏发送 ICMP ping 到 119.28.1.1
   Handle C 拦截
   → PID✓
   → isFakeIP(119.28.1.1) = false → RealDstIP = 119.28.1.1
   → 封装ICMP发往隧道 → 节点ping 119.28.1.1
   → 响应回来 → Handle D 处理 → SrcIP不需要替换 → 返回游戏


┌─────────────────────────────────────────────────────────────────────────────┐
│                    场景3: 命中排除规则（不加速）                               │
└─────────────────────────────────────────────────────────────────────────────┘

1. 游戏更新请求: TCP 47.52.1.1:443
   Handle A 拦截
   → PID✓
   → 排除规则检查: TCP:443 命中游戏专属排除规则 ✓
   → WinDivertSend 放行
   → KnownIPs 不记录该IP  ← 不触发Handle C更新
   → 走原始网络直连
```

---

## 配置文件结构

```yaml
# config/games/lol.yaml
game:
  id: "lol"
  name: "英雄联盟"
  process:
    - "League of Legends.exe"
    - "LeagueClient.exe"

  bypass:
    # 协议级排除
    - protocol: tcp
      dst_port: [80, 443]
      comment: "客户端更新/HTTP"

    # IP段排除
    - dst_ip: ["101.71.0.0/16", "203.107.0.0/16"]
      comment: "国内CDN"

    # 组合排除
    - protocol: udp
      dst_ip: "47.52.0.0/16"
      dst_port: [8080, 8443]
      comment: "统计上报"


# config/global_bypass.yaml
global_bypass:
  # 所有游戏通用，内置强制放行
  forced:
    - dst_ip: ["127.0.0.0/8"]
      comment: "本地回环"
    - dst_ip: ["192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"]
      comment: "局域网"
    - dst_ip: ["224.0.0.0/4", "255.255.255.255/32"]
      comment: "组播/广播"

  # 用户自定义
  custom:
    - dst_ip: "1.2.3.4"
      comment: "用户手动添加"
```

---

## 模块依赖关系

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           模块依赖关系                                       │
└─────────────────────────────────────────────────────────────────────────────┘

  ProcessMonitor
      │ 监控进程启动/退出
      ▼
  PIDTable ──────────────────────────────────────────────────────────────────┐
      │                                                                      │
      │ 查询PID归属                                                          │
      ▼                                                                      │
  Handle A (TCP/UDP出方向)                                                   │
      │                                                                      │
      ├── 读取 BypassRuleSet ← PIDEntry                                      │
      │                                                                      │
      ├── 读写 DNSNATTable                                                   │
      │       │                                                              │
      │       └── Handle B (DNS) 也读写                                     │
      │                                                                      │
      ├── 读写 ConnTrackTable                                                │
      │       │                                                              │
      │       └── Handle D (入方向) 也读写                                   │
      │                                                                      │
      └── 写入 KnownIPTable ──────────────────────────────────────────────┐  │
                                                                          │  │
  ConnTrackTable 清理协程                                                  │  │
      │                                                                   │  │
      └── 移除过期条目 → 更新 KnownIPTable ──────────────────────────────┤  │
                                                                          │  │
  PIDTable 进程退出事件                                                    │  │
      │                                                                   │  │
      └── 清空 KnownIPTable ──────────────────────────────────────────────┤  │
                                                                          │  │
  KnownIPTable 变化事件 ──────────────────────────────────────────────────┘  │
      │                                                                      │
      ▼ (防抖100ms)                                                          │
  HandleC Manager                                                            │
      │                                                                      │
      ├── BuildWinDivertFilter()                                             │
      │                                                                      │
      └── 重启 Handle C (ICMP出方向) ← 读取 DNSNATTable / KnownIPTable      │
                                                                            │
  PIDTable 进程退出 ──────────────────────────────────────────────────────────┘
      │
      └── 清理所有相关状态
```

---

## 实现指南

### 项目架构总览

PingZero 是基于 WinDivert 的 Windows 网络包拦截加速器。通过 4 个并发 WinDivert Handle 分别处理不同的网络层级和方向，最终通过 KCP 隧道转发流量到 Linux 服务端。

### ⚠️ 重要使用约束：先启动加速，再启动游戏

```
错误的顺序（游戏先启动）：
  游戏 DNS 查询 (Handle B 未开启) ← 获得真实 IP
  游戏连接真实 IP (Handle A 未开启) ← 走原始网络
  启动加速器 ← 只能拦截新连接，已建立连接无法重定向

✅ 正确的顺序：
  1. 启动 PingZero 加速器
     → Engine.Start()
     → 4 个 WinDivert Handle 全部打开并准备接收
  2. 选择游戏和加速节点
  3. 点击"启动加速"
  4. 【这时才】启动游戏进程
     → WMI ProcessStartTrace 事件立即触发
     → PIDTable 添加该 PID 的游戏记录
     → 游戏所有流量立即被 Handle A/B/C/D 拦截
```

**为什么必须这样**：
- WinDivert 是包级过滤器，只能拦截新包和新连接
- 游戏启动时的初始 DNS 查询和连接建立无法"回溯"
- 必须在游戏发包之前就准备好拦截器

---

### 完整项目结构

```
PingZero/
├── cmd/
│   ├── client/
│   │   ├── main.go              # Wails app.Run()，管理员权限清单
│   │   ├── app.go               # App struct，所有 Wails 绑定方法
│   │   └── main.manifest        # 清单：requireAdministrator
│   └── server/
│       └── main.go              # 服务端入口，flag 解析
│
├── internal/
│   ├── windivert/               # WinDivert DLL 绑定层
│   │   ├── dll.go               # LoadDLL，所有 Proc 句柄，全局 *WinDivert
│   │   ├── types.go             # WINDIVERT_ADDRESS (80字节) 及所有包头结构
│   │   ├── handle.go            # Handle: Open/Recv/Send/Close/Shutdown
│   │   ├── packet.go            # ParsePacket（纯 Go），CalcChecksums（调 DLL）
│   │   └── filter.go            # BuildICMPFilter，过滤器字符串工具
│   │
│   ├── state/                   # 核心状态管理（并发安全）
│   │   ├── store.go             # Store 聚合所有表，ICMPUpdateCh 信号
│   │   ├── pid_table.go         # PIDTable → PID → PIDEntry
│   │   ├── dns_nat_table.go     # DNSNATTable：虚假IP ↔ 真实IP ↔ 域名
│   │   ├── conn_track.go        # ConnTrackTable + ConnKey + ConnEntry + 清理协程
│   │   ├── known_ip.go          # KnownIPTable（per PIDEntry，防抖通知）
│   │   └── bypass.go            # BypassRuleSet + BypassRule.Match()
│   │
│   ├── engine/                  # 4 个 Handle 引擎
│   │   ├── engine.go            # Engine: Start/Stop，编排 4 个 Handle + 后台协程
│   │   ├── handle_a.go          # Handle A：TCP/UDP 出方向（核心拦截）
│   │   ├── handle_b.go          # Handle B：DNS 劫持（优先级 1000）
│   │   ├── handle_c.go          # Handle C：ICMP 出方向，动态过滤器 + 防抖重启
│   │   ├── handle_d.go          # Handle D：入方向回包，还原虚假 IP
│   │   └── port_pool.go         # TunnelPort 分配器（40000–59999）
│   │
│   ├── tunnel/                  # 可插拔隧道实现
│   │   ├── interface.go         # Tunnel 接口 + TunnelFactory + Registry
│   │   ├── manager.go           # TunnelManager：本地代理监听
│   │   ├── proto.go             # 帧格式：[4B len][proto][IPs][ports][payload]
│   │   └── kcp/
│   │       └── kcp_tunnel.go    # KCP 实现，init() 注册 "kcp"
│   │
│   ├── process/                 # 进程监控和 PID 查询
│   │   ├── monitor.go           # WMI 事件订阅（Win32_ProcessStartTrace/StopTrace）
│   │   └── pid_lookup.go        # GetExtendedTcpTable/UdpTable 查 PID（100ms 缓存）
│   │
│   ├── config/                  # 配置加载
│   │   ├── game.go              # GameConfig + BypassRuleConfig 结构
│   │   ├── global.go            # GlobalConfig（forced + custom bypass）
│   │   └── loader.go            # LoadAll()：读所有 YAML 配置
│   │
│   ├── dns/                     # DNS 后台处理
│   │   └── resolver.go          # 异步 DNS 解析，更新 DNSNATTable
│   │
│   └── relay/                   # 服务端中继（Linux）
│       ├── relay.go             # RelayServer：KCP 监听 + accept session
│       └── session.go           # Session：解帧 → 上游转发 → 回帧
│
├── frontend/                    # Wails Vue3 前端
│   ├── src/
│   │   ├── App.vue
│   │   ├── components/          # GameList.vue, NodeSelector.vue, StatusBar.vue, LatencyDisplay.vue
│   │   └── stores/              # Pinia store：Wails 绑定方法包装
│   └── package.json
│
├── config/                      # 配置文件示例
│   ├── games/
│   │   ├── lol.yaml             # 英雄联盟配置
│   │   ├── csgo.yaml
│   │   └── ...
│   └── global_bypass.yaml       # 全局排除规则
│
├── wails.json                   # Wails 项目配置
└── go.mod                       # module github.com/xyuer/PingZero
```

---

### WinDivert 绑定设计

由于 WinDivert v2.2.0 没有官方 Go bindings，使用 `syscall.LoadDLL` 直接调用 DLL。

**优点**：
- 无需 CGo，避免编译复杂度
- 运行时动态加载 DLL，exe 与 DLL 打包在一起
- 调试友好，完全 Go 代码路径

**关键类型**：
```go
// WINDIVERT_ADDRESS（80 字节，已验证）
type Address struct {
    Timestamp int64    // offset 0
    Flags0    uint32   // offset 8: Layer[7:0] Event[15:8] Flags[31:16]
    Reserved2 uint32   // offset 12
    Data      [64]byte // offset 16: union (Network/Flow/Socket)
}

// init() 中验证大小
func init() {
    if unsafe.Sizeof(Address{}) != 80 {
        panic("WINDIVERT_ADDRESS size mismatch")
    }
}

// 位字段访问器
func (a *Address) Outbound() bool  { return (a.Flags0>>17)&1 == 1 }
func (a *Address) Loopback() bool  { return (a.Flags0>>18)&1 == 1 }
```

---

### Handle 优先级与过滤规则

| Handle | 过滤规则 | 优先级 | 功能 |
|--------|---------|-------|------|
| **B** | `udp.DstPort == 53 and outbound` | **1000** | DNS 劫持，分配虚假 IP (198.18.x.x)，伪造响应 |
| **A** | `(tcp or udp) and outbound and !loopback` | 0 | 核心：拦截游戏流量，本地隧道重定向 |
| **C** | `icmp and outbound and (ip.DstAddr == X or ...)` | 0 | ICMP 转发，动态 IP 过滤（防抖重启） |
| **D** | `(tcp or udp or icmp) and inbound and !loopback` | 0 | 回包处理，还原虚假 IP，返回游戏 |

> Handle B 优先级最高（1000），确保 DNS 包优先被拦截，不被 A 处理。

---

### Handle C 动态重启机制

当 KnownIPTable 变化时（新增/移除 IP），Handle C 需要重新生成过滤规则并重启：

```
流程：
  1. Handle A 每次拦截新连接时 → KnownIPTable.Add(dstIP, realIP)
  2. KnownIPTable.Add() 发送信号到 debounceWorker
  3. debounceWorker 等待 100ms 内无新信号 → 防抖
  4. 调用 rebuild():
     a) lock manager.mu（防止并发重启）
     b) oldHandle.Shutdown(ShutdownBoth) ← 立即解除 Recv 阻塞
     c) 等待 oldDone chan 关闭（old goroutine 完全退出）
     d) newFilter = BuildICMPFilter(knownIPs.All())
     e) WinDivertOpen(newFilter, layer, priority, flags) ← 新 Handle
     f) go icmpLoop(newHandle, newDone)
     g) unlock

防抖效果：游戏启动时可能在 100ms 内建立 100+ 连接 → 防抖
         保证 Handle C 只重启 1 次而不是 100 次
```

**Handle C 的 Recv 被 Shutdown 中断时**：
- `icmpLoop` 收到错误，检查 `ctx.Done()`
- 若未取消（仅是 shutdown），直接退出，关闭 `done` chan
- rebuild() 等待 `done` 关闭后才打开新 Handle

---

### 进程监控方案（WMI 事件订阅)

使用 WMI 事件订阅而非轮询：

```go
// 依赖：github.com/go-ole/go-ole

// 订阅进程启动/退出事件（底层由 ETW 驱动）
query := "SELECT * FROM Win32_ProcessStartTrace"  // 进程启动
query := "SELECT * FROM Win32_ProcessStopTrace"   // 进程退出

// 事件延迟：<50ms，完全满足游戏加速器需求
// 无轮询开销，内存占用极低

for {
    event := source.NextEvent(timeout)
    processName := event.Properties_("ProcessName")
    processID := event.Properties_("ProcessID")
    if isTargetGame(processName) {
        pidTable.Add(processID, gameID, ...)
    }
}
```

**对比轮询方案（EnumProcesses）**：
- WMI 订阅：延迟 <50ms，无轮询开销 ✅
- EnumProcesses 轮询：延迟 ~2s，每 2s 扫描全系统进程 ❌

---

### 隧道插件化架构

隧道实现完全可插拔：

```go
// tunnel/interface.go
type Tunnel interface {
    Connect(serverAddr string) error
    SendPacket(key ConnKey, payload []byte) error
    RecvPacket() (Packet, error)
    Close() error
}

type TunnelFactory func(cfg map[string]any) (Tunnel, error)

// 注册（tunnel/kcp/kcp_tunnel.go）
func init() {
    tunnel.Register("kcp", NewKCPTunnel)
}

// 使用
t, _ := tunnel.New("kcp", kcpConfig)
t.Connect(serverAddr)
```

**添加新隧道实现**（如 QUIC）：
1. 创建 `internal/tunnel/quic/quic_tunnel.go`
2. 实现 `Tunnel` 接口
3. 在 `init()` 调用 `tunnel.Register("quic", ...)`
4. 在 `cmd/client/main.go` 添加 `import _ "github.com/xyuer/PingZero/internal/tunnel/quic"`

---

### KCP 帧格式

KCP 是可靠流协议，需要自己实现帧分界（4 字节长度前缀）：

```
[4B big-endian length]           # 整帧长度（不含本 4B）
[1B proto]                       # 6=TCP, 17=UDP, 1=ICMP
[4B srcIP]                       # 客户端 IP（网络字节序）
[2B srcPort]                     # 客户端端口（网络字节序）
[4B dstIP]                       # 真实服务器 IP（网络字节序）
[2B dstPort]                     # 真实服务器端口（网络字节序）
[payload...]                     # TCP/UDP/ICMP 包体（原始字节）
```

**编码示例**：
```go
func EncodeFrame(f *Frame) []byte {
    buf := &bytes.Buffer{}
    binary.Write(buf, binary.BigEndian, uint32(0)) // 占位符，稍后填充长度
    binary.Write(buf, binary.BigEndian, f.Proto)
    buf.Write(f.SrcIP[:])
    binary.Write(buf, binary.BigEndian, f.SrcPort)
    buf.Write(f.DstIP[:])
    binary.Write(buf, binary.BigEndian, f.DstPort)
    buf.Write(f.Payload)
    
    // 回头填充长度
    result := buf.Bytes()
    binary.BigEndian.PutUint32(result[0:4], uint32(len(result)-4))
    return result
}
```

---

### 服务端中继逻辑

Linux 服务端通过 KCP 接收帧，解析后转发到真实游戏服务器：

```
RelayServer 监听 :51820/udp (KCP)
  ↓
accept(session)
  ├─ readLoop:
  │   DecodeFrame(buf) → Frame{proto, srcIP, srcPort, dstIP, dstPort, payload}
  │   ↓
  │   upstreamKey = (dstIP, dstPort, proto)
  │   upstream = upstreams.LoadOrStore(key, dial())
  │   ↓
  │   upstream.WriteToUDP(payload, {dstIP, dstPort})  // UDP
  │   或 upstream.Write(payload)                       // TCP
  │
  └─ upstreamReadLoop(upstream):
      recv(payload) from upstream
      ↓
      EncodeFrame(reply) → session.Write(encodedFrame)
```

**特点**：
- UDP 上游：`net.DialUDP` 绑定随机本地端口，OS 自动路由回包
- TCP 上游：`net.DialTCP`，按 5 元组键控
- 上游连接 5 分钟无活动自动关闭和移除
- 支持多个并发客户端，各 session 独立转发

---

### Wails 前端 API 绑定

所有 Go 方法通过 Wails 自动导出为 TypeScript，前端可直接调用：

```go
// cmd/client/app.go
type App struct {
    ctx    context.Context
    engine *engine.Engine
    store  *state.Store
}

// GetGames() 返回所有可用游戏
func (a *App) GetGames() []GameInfo {
    return []GameInfo{
        {ID: "lol", Name: "英雄联盟", Running: false, Processes: ["League of Legends.exe"]},
        ...
    }
}

// StartAcceleration(gameID, nodeAddr) 启动加速
// ⚠️ 必须在游戏启动前调用
func (a *App) StartAcceleration(gameID string, nodeAddr string) error {
    cfg, _ := config.GetGame(gameID)
    rules := state.BuildBypassRules(cfg.Bypass)
    ...
    return a.engine.Start()
}

// StopAcceleration() 停止加速
func (a *App) StopAcceleration() error {
    return a.engine.Stop()
}

// GetStatus() 返回实时状态
func (a *App) GetStatus() AccelStatus {
    return AccelStatus{
        Running: a.engine.Running(),
        GameID: currentGame,
        NodeAddr: currentNode,
        UptimeSecs: ...,
        PacketsSent: ...,
    }
}

// GetLatency() 返回延迟统计
func (a *App) GetLatency() LatencyInfo {
    return LatencyInfo{
        CurrentMS: ...,
        MinMS: ...,
        MaxMS: ...,
        AvgMS: ...,
    }
}
```

---

### 依赖库

```
github.com/xtaci/kcp-go/v5      # KCP 协议实现
github.com/wailsapp/wails/v2    # 客户端 UI 框架（Go + Vue3）
github.com/go-ole/go-ole        # WMI COM 调用（进程监控）
golang.org/x/net                # DNS 消息构造
golang.org/x/sys/windows        # Windows 系统调用
gopkg.in/yaml.v3                # YAML 配置文件解析
```

---

### 关键陷阱与注意事项

1. **ProcessId 获取限制**
   - NETWORK 层 WinDivert Handle 不提供 ProcessId
   - Handle A 需要调用 `GetExtendedTcpTable`/`GetExtendedUdpTable` (iphlpapi.dll)
   - 按源端口查询 PID，结果缓存 100ms 降低开销

2. **WinDivert DLL 部署**
   - `WinDivert.dll` 和 `WinDivert64.sys` 必须与 `.exe` 同目录
   - DLL 首次调用 `WinDivertOpen` 时自动安装内核驱动
   - 需要管理员权限

3. **管理员权限**
   - `cmd/client/main.manifest` 设置 `requestedExecutionLevel = requireAdministrator`
   - 用 `rsrc` 工具嵌入可执行文件

4. **KCP 帧分界**
   - KCP 是流协议，必须使用 4 字节长度前缀
   - 否则接收端无法正确分帧

5. **Handle D 无限循环防护**
   - 过滤器 `!loopback` 防止隧道管理器回注的包再次被拦截
   - 至关重要

6. **ConnTrackTable 反向查找**
   - Handle D 按反向键（交换 src/dst）在 ConnTrackTable 中查找
   - 判断是否需要还原虚假 IP

7. **虚假 IP 范围**
   - 使用 `198.18.0.0/15`（198.18.0.1 - 198.19.255.254）
   - IANA 为基准测试保留，不会出现为真实服务器 IP
   - 可分配 131,070 个虚假 IP

8. **⚠️ 先启动加速，再启动游戏（关键）**
   - 游戏必须在加速器启动后启动
   - 否则游戏初始 DNS 查询和连接走原始网络，后续无法修复

---

### WinDivert 运行时构建与测试

#### 运行时文件位置

PingZero 通过 `internal/windivert` 动态加载原生 `WinDivert.dll`，并由 DLL 加载同目录下的驱动文件。

当前项目内置的 x64 运行时文件位于：

```
third_party/windivert/bin/amd64/WinDivert.dll
third_party/windivert/bin/amd64/WinDivert64.sys
```

本地运行或打包客户端时，需要把这两个文件复制到客户端 `.exe` 同目录。例如：

```powershell
Copy-Item third_party\windivert\bin\amd64\WinDivert.dll  .\build\bin\
Copy-Item third_party\windivert\bin\amd64\WinDivert64.sys .\build\bin\
```

> 注意：客户端必须以管理员权限运行，否则 `WinDivertOpen` 无法加载内核驱动。

#### 从 WinDivert 源码重新编译

如果 WinDivert 源码位于仓库同级目录 `..\WinDivert`，直接运行：

```powershell
third_party\windivert\build-windivert.cmd
```

如果源码在其它位置：

```powershell
third_party\windivert\build-windivert.cmd -SourceDir E:\workspace\WinDivert
```

脚本会构建：

- `WinDivert.dll`
- `WinDivert64.sys`

然后自动复制到：

```
third_party/windivert/bin/amd64/
```

当前脚本默认使用：

- Visual Studio: `C:\Program Files\Microsoft Visual Studio\18\Community`
- MSVC: `14.51.36231`
- Windows Kit: `10.0.26100.0`
- KMDF: `1.35`
- 驱动签名：关闭（`SignMode=Off`）

#### 最小加载测试

1. 先确认运行时文件存在：

```powershell
Test-Path third_party\windivert\bin\amd64\WinDivert.dll
Test-Path third_party\windivert\bin\amd64\WinDivert64.sys
```

2. 构建或运行客户端前，把 `WinDivert.dll` 和 `WinDivert64.sys` 放到客户端 `.exe` 同目录。

3. 使用管理员权限启动客户端。第一次调用 `WinDivertOpen` 时，WinDivert 会尝试加载驱动。

4. 如果加载失败，优先检查：
   - `.exe` 同目录是否同时存在 `WinDivert.dll` 和 `WinDivert64.sys`
   - 是否以管理员权限运行
   - 驱动是否未签名导致 Windows 拒绝加载
   - Windows 事件查看器中是否有驱动加载错误

#### 驱动签名说明

当前构建脚本生成的是未签名开发构建，可用于本地开发验证。正式发布给普通用户前，需要对 `WinDivert64.sys` 做合法驱动签名，否则 Windows 10/11 默认安全策略可能拒绝加载。

本地测试未正式签名驱动时，请使用测试签名和 Windows Test Mode，详细步骤见：

```
third_party/windivert/TEST_SIGNING.md
```

---

### 实现顺序（推荐）

1. **基础层**
   - `internal/windivert`（DLL 绑定 + 类型定义）
   - 单元测试：验证 `unsafe.Sizeof(Address{}) == 80`

2. **状态层**
   - `internal/state`（5 个表 + 并发安全）
   - 竞态测试：`go test -race`

3. **进程层**
   - `internal/process`（WMI monitor + GetExtendedTcpTable lookup）

4. **配置层**
   - `internal/config` + `config/games/*.yaml`

5. **隧道层**
   - `internal/tunnel/interface.go` + `kcp/` + `proto.go`

6. **引擎层**
   - `internal/engine`（4 个 Handle，顺序：B → A → D → C）

7. **服务端**
   - `internal/relay` + `cmd/server/main.go`

8. **前端**
   - `cmd/client/app.go` + Wails Vue3 UI

---

### 验证计划

#### 单元测试（无需 WinDivert 驱动）

- `windivert/types_test.go`：验证 Address 大小 (80B)，bit accessor 逻辑
- `state/*_test.go`：表的并发操作，`go test -race` 竞态检测
- `engine/handle_b_test.go`：DNS query 构造，forged response 验证
- `engine/handle_a_test.go`：IP 包头改写逻辑（mock WinDivert backend）
- `tunnel/proto_test.go`：帧编解码往返
- `engine/handle_c_test.go`：100 goroutine 并发 Add IP，防抖只触发 1 次重启，`goleak` 验证无泄漏

#### 系统测试（需管理员 + WinDivert）

1. 启动本地 relay server（本机）
2. 启动客户端，选择 lol.yaml，连接 relay
3. `ping 198.18.0.1`（测试虚假 IP）→ ICMP 回包（Handle C/D 链路）
4. `netcat -u 198.18.0.1 7000` 发 UDP → relay 端收到并回包
5. 观察 ConnTrackTable 清理协程（5 分钟超时 → KnownIPTable 变化 → Handle C 重启）
6. 进程监控验证：游戏启动立即在 PIDTable 出现，游戏退出自动移除

---
