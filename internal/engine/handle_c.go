package engine

import (
	"context"
	"time"
)

func (e *Engine) handleCRebuilder(ctx context.Context) {
	var timer *time.Timer
	var timerCh <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-e.store.ICMPDirty:
			if timer == nil {
				timer = time.NewTimer(100 * time.Millisecond)
				timerCh = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(100 * time.Millisecond)
			}
		case <-timerCh:
			e.mu.Lock()
			if e.stats.Filters != nil {
				e.stats.Filters["C"] = e.store.KnownIPs.BuildWinDivertFilter()
			}
			e.mu.Unlock()
			timerCh = nil
			timer = nil
		}
	}
}
