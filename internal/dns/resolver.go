package dns

import (
	"context"
	"net"
)

type Resolver struct {
	resolver *net.Resolver
}

func NewResolver() *Resolver {
	return &Resolver{resolver: net.DefaultResolver}
}

func (r *Resolver) LookupHost(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := r.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}
	return ips, nil
}
