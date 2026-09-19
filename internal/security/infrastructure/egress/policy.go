package egress

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

type Policy struct {
	allowAnyOrigin bool
	origins        map[string]bool
	privateCIDRs   []netip.Prefix
	allowHTTP      bool
}

func NewPolicy(origins, privateCIDRs []string, allowHTTP, allowAnyOrigin bool) (*Policy, error) {
	if allowAnyOrigin && !allowHTTP {
		return nil, errors.New("open origins require development mode")
	}
	p := &Policy{allowAnyOrigin: allowAnyOrigin, origins: make(map[string]bool), allowHTTP: allowHTTP}
	for _, raw := range origins {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return nil, errors.New("invalid egress origin")
		}
		if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
			return nil, errors.New("invalid egress scheme")
		}
		p.origins[origin(u)] = true
	}
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
	if !p.allowAnyOrigin && !p.origins[origin(u)] {
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

func origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
