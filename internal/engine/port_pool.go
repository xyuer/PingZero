package engine

import "sync"

type PortPool struct {
	mu   sync.Mutex
	min  uint16
	next uint16
	max  uint16
	used map[uint16]struct{}
}

func NewPortPool(min, max uint16) *PortPool {
	return &PortPool{min: min, next: min, max: max, used: make(map[uint16]struct{})}
}

func (p *PortPool) Allocate() (uint16, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	start := p.next
	for {
		if _, ok := p.used[p.next]; !ok {
			port := p.next
			p.used[port] = struct{}{}
			p.advance()
			return port, true
		}
		p.advance()
		if p.next == start {
			return 0, false
		}
	}
}

func (p *PortPool) Release(port uint16) {
	p.mu.Lock()
	delete(p.used, port)
	p.mu.Unlock()
}

func (p *PortPool) advance() {
	if p.next >= p.max {
		p.next = p.min
		return
	}
	p.next++
}
