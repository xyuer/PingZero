package tunnel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const frameHeaderLen = 1 + 4 + 2 + 4 + 2

var (
	ErrFrameTooShort = errors.New("frame too short")
	ErrFrameLength   = errors.New("frame length mismatch")
)

type Frame struct {
	Proto   uint8
	SrcIP   [4]byte
	SrcPort uint16
	DstIP   [4]byte
	DstPort uint16
	Payload []byte
}

func EncodeFrame(f *Frame) []byte {
	if f == nil {
		f = &Frame{}
	}
	size := 4 + frameHeaderLen + len(f.Payload)
	out := make([]byte, size)
	binary.BigEndian.PutUint32(out[0:4], uint32(size-4))
	out[4] = f.Proto
	copy(out[5:9], f.SrcIP[:])
	binary.BigEndian.PutUint16(out[9:11], f.SrcPort)
	copy(out[11:15], f.DstIP[:])
	binary.BigEndian.PutUint16(out[15:17], f.DstPort)
	copy(out[17:], f.Payload)
	return out
}

func DecodeFrame(buf []byte) (*Frame, error) {
	if len(buf) < 4+frameHeaderLen {
		return nil, ErrFrameTooShort
	}
	want := int(binary.BigEndian.Uint32(buf[0:4]))
	if want != len(buf)-4 {
		return nil, fmt.Errorf("%w: want %d got %d", ErrFrameLength, want, len(buf)-4)
	}
	f := &Frame{Proto: buf[4], SrcPort: binary.BigEndian.Uint16(buf[9:11]), DstPort: binary.BigEndian.Uint16(buf[15:17])}
	copy(f.SrcIP[:], buf[5:9])
	copy(f.DstIP[:], buf[11:15])
	f.Payload = append([]byte(nil), buf[17:]...)
	return f, nil
}

func IPv4(ip net.IP) [4]byte {
	var out [4]byte
	copy(out[:], ip.To4())
	return out
}
