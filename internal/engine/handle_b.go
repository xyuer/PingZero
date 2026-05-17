package engine

import "github.com/xyuer/PingZero/internal/windivert"

func dnsHandleOptions() windivert.OpenOptions {
	return windivert.OpenOptions{Filter: windivert.DNSFilter, Layer: windivert.LayerNetwork, Priority: 1000}
}
