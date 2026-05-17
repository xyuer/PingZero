//go:build windows

package windivert

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const defaultPacketBufferSize = 0xffff

var (
	winDivertDLL = windows.NewLazyDLL("WinDivert.dll")

	procOpen                = winDivertDLL.NewProc("WinDivertOpen")
	procRecv                = winDivertDLL.NewProc("WinDivertRecv")
	procSend                = winDivertDLL.NewProc("WinDivertSend")
	procShutdown            = winDivertDLL.NewProc("WinDivertShutdown")
	procClose               = winDivertDLL.NewProc("WinDivertClose")
	procHelperCalcChecksums = winDivertDLL.NewProc("WinDivertHelperCalcChecksums")
)

type dllHandle struct {
	handle windows.Handle
	bufLen int
}

func Open(options OpenOptions) (Handle, error) {
	if options.Filter == "" {
		return nil, errors.New("windivert filter is required")
	}
	if err := winDivertDLL.Load(); err != nil {
		return nil, fmt.Errorf("%w: load WinDivert.dll: %v", ErrUnsupported, err)
	}
	filter, err := syscall.BytePtrFromString(options.Filter)
	if err != nil {
		return nil, fmt.Errorf("encode windivert filter: %w", err)
	}
	ret, _, callErr := procOpen.Call(
		uintptr(unsafe.Pointer(filter)),
		uintptr(options.Layer),
		uintptr(int16(options.Priority)),
		uintptr(options.Flags),
	)
	if ret == uintptr(windows.InvalidHandle) || ret == 0 {
		return nil, wrapLastError("WinDivertOpen", callErr)
	}
	bufLen := options.QueueLen
	if bufLen <= 0 {
		bufLen = defaultPacketBufferSize
	}
	return &dllHandle{handle: windows.Handle(ret), bufLen: bufLen}, nil
}

func (h *dllHandle) Recv(ctx context.Context) (*Packet, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	buf := make([]byte, h.bufLen)
	var recvLen uint32
	var addr Address
	ret, _, callErr := procRecv.Call(
		uintptr(h.handle),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(uint32(len(buf))),
		uintptr(unsafe.Pointer(&recvLen)),
		uintptr(unsafe.Pointer(&addr)),
	)
	runtime.KeepAlive(buf)
	if ret == 0 {
		return nil, wrapLastError("WinDivertRecv", callErr)
	}
	return &Packet{Data: buf[:recvLen], Address: addr}, nil
}

func (h *dllHandle) Send(ctx context.Context, packet *Packet) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if packet == nil || len(packet.Data) == 0 {
		return errors.New("packet data is required")
	}
	var sendLen uint32
	ret, _, callErr := procSend.Call(
		uintptr(h.handle),
		uintptr(unsafe.Pointer(&packet.Data[0])),
		uintptr(uint32(len(packet.Data))),
		uintptr(unsafe.Pointer(&sendLen)),
		uintptr(unsafe.Pointer(&packet.Address)),
	)
	runtime.KeepAlive(packet)
	if ret == 0 {
		return wrapLastError("WinDivertSend", callErr)
	}
	if sendLen != uint32(len(packet.Data)) {
		return fmt.Errorf("WinDivertSend sent %d of %d bytes", sendLen, len(packet.Data))
	}
	return nil
}

func (h *dllHandle) Shutdown(mode ShutdownMode) error {
	ret, _, callErr := procShutdown.Call(uintptr(h.handle), uintptr(mode))
	if ret == 0 {
		return wrapLastError("WinDivertShutdown", callErr)
	}
	return nil
}

func (h *dllHandle) Close() error {
	ret, _, callErr := procClose.Call(uintptr(h.handle))
	if ret == 0 {
		return wrapLastError("WinDivertClose", callErr)
	}
	return nil
}

func CalcChecksums(packet *Packet, flags uint64) error {
	if packet == nil || len(packet.Data) == 0 {
		return errors.New("packet data is required")
	}
	ret, _, callErr := procHelperCalcChecksums.Call(
		uintptr(unsafe.Pointer(&packet.Data[0])),
		uintptr(uint32(len(packet.Data))),
		uintptr(unsafe.Pointer(&packet.Address)),
		uintptr(flags),
	)
	runtime.KeepAlive(packet)
	if ret == 0 {
		return wrapLastError("WinDivertHelperCalcChecksums", callErr)
	}
	return nil
}

func wrapLastError(name string, err error) error {
	if err != nil && !errors.Is(err, syscall.Errno(0)) {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s failed", name)
}
