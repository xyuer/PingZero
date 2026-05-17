package windivert

import "unsafe"

type Address struct {
	Timestamp int64
	Flags0    uint32
	Reserved2 uint32
	Data      [64]byte
}

func init() {
	if unsafe.Sizeof(Address{}) != 80 {
		panic("WINDIVERT_ADDRESS size mismatch")
	}
}

func (a *Address) Layer() uint8   { return uint8(a.Flags0 & 0xff) }
func (a *Address) Event() uint8   { return uint8((a.Flags0 >> 8) & 0xff) }
func (a *Address) Outbound() bool { return (a.Flags0>>17)&1 == 1 }
func (a *Address) Loopback() bool { return (a.Flags0>>18)&1 == 1 }

type Packet struct {
	Data    []byte
	Address Address
}

type Header struct {
	Protocol uint8
	SrcIP    [4]byte
	DstIP    [4]byte
	SrcPort  uint16
	DstPort  uint16
}
