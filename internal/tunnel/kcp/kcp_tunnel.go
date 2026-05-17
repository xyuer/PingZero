package kcp

import (
	"errors"
	"sync"

	kcpgo "github.com/xtaci/kcp-go/v5"
	"github.com/xyuer/PingZero/internal/state"
	"github.com/xyuer/PingZero/internal/tunnel"
)

var _ *kcpgo.UDPSession

func init() {
	tunnel.Register("kcp", NewKCPTunnel)
}

type KCPTunnel struct {
	mu        sync.Mutex
	connected bool
	closed    bool
	server    string
}

func NewKCPTunnel(cfg map[string]any) (tunnel.Tunnel, error) {
	return &KCPTunnel{}, nil
}

func (t *KCPTunnel) Connect(serverAddr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tunnel.ErrClosed
	}
	if serverAddr == "" {
		return errors.New("server address is required")
	}
	t.server = serverAddr
	t.connected = true
	return nil
}

func (t *KCPTunnel) SendPacket(key state.ConnKey, payload []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tunnel.ErrClosed
	}
	if !t.connected {
		return errors.New("tunnel is not connected")
	}
	return nil
}

func (t *KCPTunnel) RecvPacket() (tunnel.Packet, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tunnel.Packet{}, tunnel.ErrClosed
	}
	return tunnel.Packet{}, errors.New("kcp receive is not implemented in skeleton")
}

func (t *KCPTunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.connected = false
	return nil
}
