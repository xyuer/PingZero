package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/xyuer/PingZero/internal/state"
	"github.com/xyuer/PingZero/internal/windivert"
)

var ErrAlreadyRunning = errors.New("engine is already running")

type Engine struct {
	store *state.Store

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	stats   Stats
}

type Stats struct {
	PacketsSent     uint64
	PacketsReceived uint64
	StartedAt       time.Time
	Filters         map[string]string
}

func New(store *state.Store) *Engine {
	if store == nil {
		store = state.NewStore()
	}
	return &Engine{store: store}
}

func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return ErrAlreadyRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true
	e.stats = Stats{
		StartedAt: time.Now(),
		Filters: map[string]string{
			"B": windivert.DNSFilter,
			"A": windivert.OutboundFilter,
			"D": windivert.InboundFilter,
			"C": e.store.KnownIPs.BuildWinDivertFilter(),
		},
	}
	go e.handleCRebuilder(ctx)
	return nil
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return nil
	}
	e.cancel()
	e.running = false
	return nil
}

func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) Stats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := e.stats
	cp.Filters = make(map[string]string, len(e.stats.Filters))
	for k, v := range e.stats.Filters {
		cp.Filters[k] = v
	}
	return cp
}

func (e *Engine) Store() *state.Store {
	return e.store
}
