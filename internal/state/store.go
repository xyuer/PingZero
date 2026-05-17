package state

import "net"

type Store struct {
	PIDs      *PIDTable
	DNS       *DNSNATTable
	Conns     *ConnTrackTable
	KnownIPs  *KnownIPTable
	ICMPDirty chan struct{}
	Bypass    *BypassRuleSet
}

func NewStore() *Store {
	ch := make(chan struct{}, 1)
	return &Store{
		PIDs:      NewPIDTable(),
		DNS:       NewDNSNATTable(),
		Conns:     NewConnTrackTable(),
		KnownIPs:  NewKnownIPTable(ch),
		ICMPDirty: ch,
		Bypass:    NewBypassRuleSet(nil),
	}
}

func BuildICMPFilter(ips []net.IP) string {
	if len(ips) == 0 {
		return "icmp and outbound and false"
	}
	filter := "icmp and outbound and ("
	for i, ip := range ips {
		if i > 0 {
			filter += " or "
		}
		filter += "ip.DstAddr == " + ip.String()
	}
	filter += ")"
	return filter
}
