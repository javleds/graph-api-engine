package egress

import (
	"net/netip"
	"testing"
)

func TestPolicyRejectsMixedDNSAndMappedPrivateIP(t *testing.T) {
	policy, err := NewPolicy([]string{"https://example.com"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::ffff:127.0.0.1", "::", "ff02::1", "100.64.1.1"} {
		if policy.ValidateAddresses([]netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(raw)}) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if policy.ValidateURL("https://example.com:443/path?q=secret") != nil {
		t.Fatal("canonical origin rejected")
	}
	if policy.ValidateURL("https://evil.example.com") == nil {
		t.Fatal("unlisted origin accepted")
	}
	if policy.ValidateURL("http://example.com") == nil {
		t.Fatal("HTTP accepted")
	}
}

func TestWildcardOriginPreservesNetworkRestrictions(t *testing.T) {
	policy, err := NewPolicy([]string{"*"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://unlisted.example/v1", "http://other.example:8080/hook"} {
		if err := policy.ValidateURL(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"ftp://other.example", "https://user:secret@other.example", "https://other.example/#fragment"} {
		if policy.ValidateURL(raw) == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::ffff:127.0.0.1"} {
		if policy.ValidateAddresses([]netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(ip)}) == nil {
			t.Fatalf("private DNS allowed: %s", ip)
		}
	}
	if err := policy.ValidateAddresses([]netip.Addr{netip.MustParseAddr("8.8.8.8")}); err != nil {
		t.Fatal(err)
	}
}
