package state

import (
	"net"
	"sort"
	"sync"
	"time"
)

type KnownIPTable struct {
	ips      map[string]*KnownIPEntry
	mu       sync.RWMutex
	updateCh chan struct{}
}

type KnownIPEntry struct {
	IP        net.IP
	FirstSeen time.Time
	LastSeen  time.Time
	ConnCount int
	IsFakeIP  bool
	RealIP    net.IP
}

func NewKnownIPTable(updateCh chan struct{}) *KnownIPTable {
	return &KnownIPTable{ips: make(map[string]*KnownIPEntry), updateCh: updateCh}
}

func (t *KnownIPTable) Add(ip net.IP, realIP net.IP) {
	if ip == nil {
		return
	}
	key := ip.String()
	now := time.Now()
	t.mu.Lock()
	if entry, ok := t.ips[key]; ok {
		entry.LastSeen = now
		entry.ConnCount++
		if realIP != nil {
			entry.RealIP = cloneIP(realIP)
			entry.IsFakeIP = true
		}
		t.mu.Unlock()
		t.notify()
		return
	}
	t.ips[key] = &KnownIPEntry{
		IP:        cloneIP(ip),
		FirstSeen: now,
		LastSeen:  now,
		ConnCount: 1,
		IsFakeIP:  realIP != nil,
		RealIP:    cloneIP(realIP),
	}
	t.mu.Unlock()
	t.notify()
}

func (t *KnownIPTable) Contains(ip net.IP) bool {
	if ip == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.ips[ip.String()]
	return ok
}

func (t *KnownIPTable) Remove(ip net.IP) {
	if ip == nil {
		return
	}
	t.mu.Lock()
	_, existed := t.ips[ip.String()]
	delete(t.ips, ip.String())
	t.mu.Unlock()
	if existed {
		t.notify()
	}
}

func (t *KnownIPTable) All() []net.IP {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]net.IP, 0, len(t.ips))
	for _, entry := range t.ips {
		out = append(out, cloneIP(entry.IP))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func (t *KnownIPTable) BuildWinDivertFilter() string {
	return BuildICMPFilter(t.All())
}

func (t *KnownIPTable) notify() {
	if t.updateCh == nil {
		return
	}
	select {
	case t.updateCh <- struct{}{}:
	default:
	}
}

func cloneIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	cp := make(net.IP, len(ip))
	copy(cp, ip)
	return cp
}
