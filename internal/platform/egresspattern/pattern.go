package egresspattern

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Pattern is a normalized allowed destination origin. A wildcard applies only
// to a complete hostname label and never includes the apex domain.
type Pattern struct {
	any      bool
	scheme   string
	host     string
	port     string
	wildcard bool
}

func ParseAll(values []string, allowHTTP bool) ([]Pattern, error) {
	patterns := make([]Pattern, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		pattern, err := Parse(value, allowHTTP)
		if err != nil {
			return nil, err
		}
		key := pattern.key()
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate destination pattern")
		}
		seen[key] = struct{}{}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

func Parse(value string, allowHTTP bool) (Pattern, error) {
	if value == "*" {
		return Pattern{any: true}, nil
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Hostname() == "" {
		return Pattern{}, fmt.Errorf("invalid destination pattern")
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return Pattern{}, fmt.Errorf("invalid destination pattern scheme")
	}
	host := strings.ToLower(u.Hostname())
	wildcard := strings.HasPrefix(host, "*.")
	if wildcard {
		host = strings.TrimPrefix(host, "*.")
		if host == "" || strings.Contains(host, "*") || net.ParseIP(host) != nil {
			return Pattern{}, fmt.Errorf("invalid destination wildcard")
		}
	} else if strings.Contains(host, "*") {
		return Pattern{}, fmt.Errorf("invalid destination wildcard")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return Pattern{scheme: strings.ToLower(u.Scheme), host: host, port: port, wildcard: wildcard}, nil
}

func Matches(patterns []Pattern, target *url.URL) bool {
	for _, pattern := range patterns {
		if pattern.Matches(target) {
			return true
		}
	}
	return false
}

func (p Pattern) Matches(target *url.URL) bool {
	if p.any {
		return true
	}
	if strings.ToLower(target.Scheme) != p.scheme || canonicalPort(target) != p.port {
		return false
	}
	host := strings.ToLower(target.Hostname())
	if p.wildcard {
		return host != p.host && strings.HasSuffix(host, "."+p.host)
	}
	return host == p.host
}

func (p Pattern) key() string {
	if p.any {
		return "*"
	}
	prefix := ""
	if p.wildcard {
		prefix = "*."
	}
	return p.scheme + "://" + prefix + p.host + ":" + p.port
}

func canonicalPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}
