package domain

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func FuzzDuration(f *testing.F) {
	for _, seed := range []string{"7d", "500ms", "9223372036854775807h", "0s"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 128 {
			t.Skip()
		}
		duration, err := ParseDuration(input)
		if err == nil && duration <= 0 {
			t.Fatal("accepted nonpositive duration")
		}
	})
}
func FuzzSelector(f *testing.F) {
	for _, seed := range []string{"data.0.id", "data.1", "missing", "*", "data..id"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 2048 {
			t.Skip()
		}
		path, err := ParseDotPath(input)
		if err != nil {
			return
		}
		_, _ = Select(map[string]any{"data": []any{map[string]any{"id": json.Number("9007199254740993")}, nil}}, path)
	})
}
func FuzzPathBinding(f *testing.F) {
	for _, seed := range []string{"a/b?x#y", "..", "", "9007199254740993", "ümlaut"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 8192 {
			t.Skip()
		}
		template, err := CompileTemplate("/items/{parent.id}", []string{"items"})
		if err != nil {
			t.Fatal(err)
		}
		path, err := template.Resolve(map[string]any{"parent": map[string]any{"id": input}}, true)
		if err != nil {
			return
		}
		u, err := url.Parse("https://example.com" + path)
		if err != nil {
			t.Fatal(err)
		}
		if u.Host != "example.com" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.EscapedPath(), "/items/") {
			t.Fatal("binding escaped path boundary")
		}
	})
}
func TestScalarAndBindingMissingBoundaries(t *testing.T) {
	for _, value := range []any{true, 1, int64(2), float64(3), "text", json.Number("9007199254740993")} {
		if _, err := ScalarString(value); err != nil {
			t.Fatal(err)
		}
	}
	template, _ := CompileTemplate("{parent.id}", []string{"items"})
	for _, context := range []map[string]any{{}, {"parent": map[string]any{}}} {
		if _, err := template.Resolve(context, false); err == nil {
			t.Fatal("missing binding accepted")
		}
	}
	for _, path := range [][]string{{"a"}, {"0"}, {"-1"}, {"999999999999999999999999"}} {
		_, _ = Select([]any{nil}, path)
	}
	if _, err := Select(nil, []string{"id"}); err == nil {
		t.Fatal("scalar path accepted")
	}
	if _, err := ParseDotPath(""); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDuration("9999999999999999999999999999999d"); err == nil {
		t.Fatal("integer overflow accepted")
	}
}
