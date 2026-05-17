package tunnel

import (
	"errors"
	"fmt"
	"sync"

	"github.com/xyuer/PingZero/internal/state"
)

type ConnKey = state.ConnKey

type Packet struct {
	Key     ConnKey
	Payload []byte
}

type Tunnel interface {
	Connect(serverAddr string) error
	SendPacket(key ConnKey, payload []byte) error
	RecvPacket() (Packet, error)
	Close() error
}

type TunnelFactory func(cfg map[string]any) (Tunnel, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]TunnelFactory)
)

func Register(name string, factory TunnelFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

func New(name string, cfg map[string]any) (Tunnel, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown tunnel %q", name)
	}
	return factory(cfg)
}

var ErrClosed = errors.New("tunnel is closed")
