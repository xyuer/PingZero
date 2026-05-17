package engine

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/xyuer/PingZero/internal/state"
)

func TestEngineStartStop(t *testing.T) {
	e := New(state.NewStore())
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	if !e.Running() {
		t.Fatal("engine should be running")
	}
	if err := e.Start(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if e.Running() {
		t.Fatal("engine should be stopped")
	}
}

func TestHandleCDebounceUpdatesFilter(t *testing.T) {
	store := state.NewStore()
	e := New(store)
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	store.KnownIPs.Add(net.ParseIP("203.0.113.30"), nil)
	time.Sleep(150 * time.Millisecond)
	if got := e.Stats().Filters["C"]; got != "icmp and outbound and (ip.DstAddr == 203.0.113.30)" {
		t.Fatalf("filter = %q", got)
	}
}
