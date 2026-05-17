package process

import "errors"

var ErrPIDLookupNotImplemented = errors.New("PID lookup is not implemented in this skeleton")

type LookupKey struct {
	Protocol uint8
	SrcPort  uint16
	DstPort  uint16
}

type PIDLookup struct{}

func NewPIDLookup() *PIDLookup {
	return &PIDLookup{}
}

func (l *PIDLookup) FindPID(key LookupKey) (uint32, error) {
	return 0, ErrPIDLookupNotImplemented
}
