package relay

import (
	"context"
	"errors"
	"net"
	"sync"
)

type Server struct {
	addr string
	mu   sync.Mutex
	conn net.PacketConn
}

func NewServer(addr string) *Server {
	if addr == "" {
		addr = ":51820"
	}
	return &Server{addr: addr}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", s.addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	defer conn.Close()

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				errCh <- err
				return
			}
			_ = NewSession(addr).HandleDatagram(buf[:n])
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}
