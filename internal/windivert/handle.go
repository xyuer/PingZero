package windivert

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("windivert is only supported on windows with WinDivert.dll available")

type Layer uint8

const (
	LayerNetwork Layer = iota
)

type ShutdownMode int

const (
	ShutdownRecv ShutdownMode = 0x1
	ShutdownSend ShutdownMode = 0x2
	ShutdownBoth ShutdownMode = ShutdownRecv | ShutdownSend
)

type Handle interface {
	Recv(ctx context.Context) (*Packet, error)
	Send(ctx context.Context, packet *Packet) error
	Shutdown(mode ShutdownMode) error
	Close() error
}

type OpenOptions struct {
	Filter   string
	Layer    Layer
	Priority int16
	Flags    uint64
	QueueLen int
}
