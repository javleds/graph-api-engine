package http

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"net"
	"net/netip"
	"time"
)

type SafeDialer struct {
	Policy   securityports.EgressPolicy
	Resolver *net.Resolver
	Timeout  time.Duration
}

func (d *SafeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	resolver := d.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	ips := make([]netip.Addr, len(addresses))
	for i, address := range addresses {
		ips[i] = address.Unmap()
	}
	if err = d.Policy.ValidateAddresses(ips); err != nil {
		return nil, &domain.UpstreamFailure{Code: "EGRESS_DENIED"}
	}
	dialer := net.Dialer{Timeout: d.Timeout}
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, err
}
