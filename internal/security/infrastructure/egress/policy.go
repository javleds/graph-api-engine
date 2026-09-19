package egress

import (
	"errors"
	"net/netip"
	"net/url"

	"github.com/javleds/api-engine/internal/platform/egresspattern"
)

type Policy struct {
	patterns     []egresspattern.Pattern
	privateCIDRs []netip.Prefix
	allowHTTP    bool
}

func NewPolicy(origins, privateCIDRs []string, allowHTTP bool) (*Policy, error) {
	patterns, err := egresspattern.ParseAll(origins, allowHTTP)
	if err != nil {
		return nil, errors.New("invalid egress origin")
	}
	p := &Policy{patterns: patterns, allowHTTP: allowHTTP}
	for _, raw := range privateCIDRs {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, errors.New("invalid allowed private CIDR")
		}
		p.privateCIDRs = append(p.privateCIDRs, prefix)
	}
	return p, nil
}

func (p *Policy) ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(p.allowHTTP && u.Scheme == "http")) {
		return errors.New("EGRESS_DENIED")
	}
	if !egresspattern.Matches(p.patterns, u) {
		return errors.New("EGRESS_DENIED")
	}
	return nil
}

func (p *Policy) ValidateAddresses(addresses []netip.Addr) error {
	if len(addresses) == 0 {
		return errors.New("EGRESS_DENIED")
	}
	for _, address := range addresses {
		ip := address.Unmap()
		if !ip.IsValid() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return errors.New("EGRESS_DENIED")
		}
		blocked := ip.IsPrivate() || ip.IsLoopback() || !ip.IsGlobalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
		if !blocked {
			continue
		}
		allowed := false
		for _, prefix := range p.privateCIDRs {
			if prefix.Contains(ip) {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("EGRESS_DENIED")
		}
	}
	return nil
}
