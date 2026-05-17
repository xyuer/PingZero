package process

import (
	"context"
	"errors"
)

var ErrMonitorNotImplemented = errors.New("process monitor is not implemented in this skeleton")

type EventType string

const (
	EventStarted EventType = "started"
	EventStopped EventType = "stopped"
)

type Event struct {
	Type        EventType
	ProcessID   uint32
	ProcessName string
}

type Monitor struct {
	processes []string
}

func NewMonitor(processes []string) *Monitor {
	return &Monitor{processes: append([]string(nil), processes...)}
}

func (m *Monitor) Watch(ctx context.Context) (<-chan Event, <-chan error) {
	events := make(chan Event)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		select {
		case <-ctx.Done():
			errs <- ctx.Err()
		default:
			errs <- ErrMonitorNotImplemented
		}
	}()
	return events, errs
}
