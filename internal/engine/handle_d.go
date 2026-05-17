package engine

import "github.com/xyuer/PingZero/internal/windivert"

func inboundHandleOptions() windivert.OpenOptions {
	return windivert.OpenOptions{Filter: windivert.InboundFilter, Layer: windivert.LayerNetwork, Priority: 0}
}
