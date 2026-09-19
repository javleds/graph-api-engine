package egresspattern

import (
	"net/url"
	"testing"
)

func TestMatchesExactWildcardAndAny(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"https://api.example.com", "https://api.example.com/v1", true},
		{"https://api.example.com", "https://api.example.com:8443/v1", false},
		{"https://*.example.com", "https://api.example.com/v1", true},
		{"https://*.example.com", "https://example.com/v1", false},
		{"https://*.example.com", "https://nested.api.example.com/v1", true},
		{"*", "https://any.example/v1", true},
	}
	for _, test := range cases {
		t.Run(test.pattern+test.target, func(t *testing.T) {
			pattern, err := Parse(test.pattern, false)
			if err != nil {
				t.Fatal(err)
			}
			target, _ := url.Parse(test.target)
			if got := pattern.Matches(target); got != test.want {
				t.Fatalf("matches=%v", got)
			}
		})
	}
}

func TestRejectsUnsafePatterns(t *testing.T) {
	for _, raw := range []string{"https://foo.*.example.com", "https://*.127.0.0.1", "https://example.com/path", "http://example.com"} {
		if _, err := Parse(raw, false); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
