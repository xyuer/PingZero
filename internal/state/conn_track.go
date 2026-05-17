package state

import (
	"net"
	"sync"
	"time"
)

type ConnTrackTable struct {
	entries map[ConnKey]*ConnEntry
	mu      sync.RWMutex
}

type ConnKey struct {
	Protocol uint8
	SrcIP    [4]byte
	SrcPort  uint16
	DstIP    [4]byte
	DstPort  uint16
}

type ConnState string

const (
	ConnStateNew    ConnState = "new"
	ConnStateActive ConnState = "active"
	ConnStateClosed ConnState = "closed"
)

type ConnEntry struct {
	Key         ConnKey
	RealDstIP   net.IP
	RealDstPort uint16
	PID         uint32
	TunnelPort  uint16
	State       ConnState
	LastSeen    time.Time
	IsFakeIP    bool
}

func NewConnTrackTable() *ConnTrackTable {
	return &ConnTrackTable{entries: make(map[ConnKey]*ConnEntry)}
}

func (t *ConnTrackTable) Upsert(entry *ConnEntry) {
	if entry == nil {
		return
	}
	cp := *entry
	cp.RealDstIP = cloneIP(entry.RealDstIP)
	if cp.LastSeen.IsZero() {
		cp.LastSeen = time.Now()
	}
	t.mu.Lock()
	t.entries[cp.Key] = &cp
	t.mu.Unlock()
}

func (t *ConnTrackTable) Get(key ConnKey) (*ConnEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, ok := t.entries[key]
	if !ok {
		return nil, false
	}
	cp := *entry
	cp.RealDstIP = cloneIP(entry.RealDstIP)
	return &cp, true
}

func (t *ConnTrackTable) Delete(key ConnKey) {
	t.mu.Lock()
	delete(t.entries, key)
	t.mu.Unlock()
}

func (t *ConnTrackTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}
