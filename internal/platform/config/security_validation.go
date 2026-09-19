package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"github.com/javleds/api-engine/internal/platform/egresspattern"
)

func validateHTTP(http HTTPConfig, security SecurityConfig) error {
	host, port, err := net.SplitHostPort(http.Address)
	if err != nil || port == "" {
		return fmt.Errorf("invalid HTTP listening address")
	}
	if !security.AuthenticationEnabled {
		address, err := netip.ParseAddr(host)
		if !security.Egress.AllowHTTPForDevelopment || err != nil || !address.IsLoopback() {
			return fmt.Errorf("disabled authentication requires development loopback binding")
		}
	}
	if err := validatePublicURL(http.PublicBaseURL, security.Egress.AllowHTTPForDevelopment); err != nil {
		return fmt.Errorf("invalid public base URL: %w", err)
	}
	return nil
}
func validatePublicURL(raw string, allowHTTP bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		return fmt.Errorf("absolute HTTPS URL without credentials, query or fragment required")
	}
	return nil
}
func validateSecurity(security SecurityConfig) error {
	if security.IdentityKeyFile == "" || security.Encryption.ActiveKeyID == "" || security.Encryption.KeyFiles[security.Encryption.ActiveKeyID] == "" {
		return fmt.Errorf("identity and active encryption key paths are required")
	}
	for id, path := range security.Encryption.KeyFiles {
		if id == "" || path == "" {
			return fmt.Errorf("encryption key IDs and paths cannot be empty")
		}
	}
	if security.AuthenticationEnabled && security.PrincipalsFile == "" {
		return fmt.Errorf("principals file is required")
	}
	if security.Playground.Username == "" || security.Playground.Password == "" {
		return fmt.Errorf("playground username and password are required")
	}
	for _, origins := range [][]string{security.Egress.UpstreamAllowedOrigins, security.Egress.WebhookAllowedOrigins} {
		if len(origins) == 0 {
			return fmt.Errorf("explicit upstream and webhook origin allowlists are required")
		}
		if err := validateOrigins(origins, security.Egress.AllowHTTPForDevelopment); err != nil {
			return err
		}
	}
	for _, cidr := range security.Egress.AllowedPrivateCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("invalid private CIDR")
		}
	}
	return nil
}
func validateOrigins(origins []string, allowHTTP bool) error {
	if _, err := egresspattern.ParseAll(origins, allowHTTP); err != nil {
		return fmt.Errorf("invalid origin allowlist")
	}
	return nil
}
