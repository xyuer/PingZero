package engine

import "github.com/xyuer/PingZero/internal/windivert"

func outboundHandleOptions() windivert.OpenOptions {
	return windivert.OpenOptions{Filter: windivert.OutboundFilter, Layer: windivert.LayerNetwork, Priority: 0}
}
