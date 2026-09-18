package application

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzRequestURLBoundary(f *testing.F) {
	for _, seed := range []string{"ordinary", "a/b?x#y", "ümlaut", "%2e%2e", "..", "."} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 8192 {
			t.Skip()
		}
		result, err := BuildRequestURL("https://example.com/api/v1", "/items/"+url.PathEscape(value), map[string]string{"q": value})
		if err != nil {
			return
		}
		parsed, err := url.Parse(result)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Scheme != "https" || parsed.Host != "example.com" || parsed.User != nil || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/api/v1/items/") || parsed.Query().Get("q") != value {
			t.Fatal("URL builder changed request boundary")
		}
	})
}
