package windivert

import (
	"net"
	"strings"
)

const (
	DNSFilter      = "udp.DstPort == 53 and outbound"
	OutboundFilter = "(tcp or udp) and outbound and !loopback"
	InboundFilter  = "(tcp or udp or icmp) and inbound and !loopback"
)

func BuildICMPFilter(ips []net.IP) string {
	if len(ips) == 0 {
		return "icmp and outbound and false"
	}
	parts := make([]string, 0, len(ips))
	for _, ip := range ips {
		parts = append(parts, "ip.DstAddr == "+ip.String())
	}
	return "icmp and outbound and (" + strings.Join(parts, " or ") + ")"
}
