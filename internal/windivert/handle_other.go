//go:build !windows

package windivert

import "context"

type noopHandle struct {
	options OpenOptions
}

func Open(options OpenOptions) (Handle, error) {
	return &noopHandle{options: options}, nil
}

func (h *noopHandle) Recv(ctx context.Context) (*Packet, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, ErrUnsupported
	}
}

func (h *noopHandle) Send(ctx context.Context, packet *Packet) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrUnsupported
	}
}

func (h *noopHandle) Shutdown(mode ShutdownMode) error { return nil }
func (h *noopHandle) Close() error                     { return nil }

func CalcChecksums(packet *Packet, flags uint64) error { return ErrUnsupported }
