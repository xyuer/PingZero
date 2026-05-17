package relay

import (
	"net"

	"github.com/xyuer/PingZero/internal/tunnel"
)

type Session struct {
	addr net.Addr
}

func NewSession(addr net.Addr) *Session {
	return &Session{addr: addr}
}

func (s *Session) HandleDatagram(data []byte) error {
	_, err := tunnel.DecodeFrame(data)
	return err
}
