package state

import (
	"sync"
	"time"
)

type PIDTable struct {
	entries map[uint32]*PIDEntry
	mu      sync.RWMutex
}

type PIDEntry struct {
	PID         uint32
	ProcessName string
	GameID      string
	StartTime   time.Time
	BypassRules *BypassRuleSet
	KnownIPs    *KnownIPTable
}

func NewPIDTable() *PIDTable {
	return &PIDTable{entries: make(map[uint32]*PIDEntry)}
}

func (t *PIDTable) Add(entry *PIDEntry) {
	if entry == nil {
		return
	}
	cp := *entry
	if cp.StartTime.IsZero() {
		cp.StartTime = time.Now()
	}
	t.mu.Lock()
	t.entries[cp.PID] = &cp
	t.mu.Unlock()
}

func (t *PIDTable) Get(pid uint32) (*PIDEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, ok := t.entries[pid]
	if !ok {
		return nil, false
	}
	cp := *entry
	return &cp, true
}

func (t *PIDTable) Remove(pid uint32) {
	t.mu.Lock()
	delete(t.entries, pid)
	t.mu.Unlock()
}

func (t *PIDTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}
