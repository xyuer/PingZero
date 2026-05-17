package tunnel

import (
	"errors"
	"net"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	frame := &Frame{
		Proto:   17,
		SrcIP:   IPv4(net.ParseIP("192.0.2.10")),
		SrcPort: 5000,
		DstIP:   IPv4(net.ParseIP("203.0.113.10")),
		DstPort: 7000,
		Payload: []byte("hello"),
	}
	got, err := DecodeFrame(EncodeFrame(frame))
	if err != nil {
		t.Fatal(err)
	}
	if got.Proto != frame.Proto || got.SrcPort != frame.SrcPort || got.DstPort != frame.DstPort || string(got.Payload) != "hello" {
		t.Fatalf("decoded frame mismatch: %#v", got)
	}
}

func TestDecodeFrameRejectsBadInput(t *testing.T) {
	if _, err := DecodeFrame([]byte{0, 1}); !errors.Is(err, ErrFrameTooShort) {
		t.Fatalf("expected short frame error, got %v", err)
	}
	buf := EncodeFrame(&Frame{})
	buf[3]++
	if _, err := DecodeFrame(buf); !errors.Is(err, ErrFrameLength) {
		t.Fatalf("expected length error, got %v", err)
	}
}
