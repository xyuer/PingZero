package windivert

import (
	"testing"
	"unsafe"
)

func TestAddressSizeAndFlags(t *testing.T) {
	if unsafe.Sizeof(Address{}) != 80 {
		t.Fatalf("Address size = %d", unsafe.Sizeof(Address{}))
	}
	addr := Address{Flags0: 1 | (2 << 8) | (1 << 17) | (1 << 18)}
	if addr.Layer() != 1 || addr.Event() != 2 || !addr.Outbound() || !addr.Loopback() {
		t.Fatalf("unexpected flags: %#v", addr)
	}
}

func TestShutdownModesMatchWinDivert(t *testing.T) {
	if ShutdownRecv != 0x1 || ShutdownSend != 0x2 || ShutdownBoth != 0x3 {
		t.Fatalf("unexpected shutdown constants: recv=%d send=%d both=%d", ShutdownRecv, ShutdownSend, ShutdownBoth)
	}
}
