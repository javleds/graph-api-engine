package mapper

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
)

const input = `{"definition":{"version":1,"api":{"baseUrl":"https://example.com/api/v1","limits":{"requests":100,"per":"1m"},"cache":{"ttl":"1h"}},"calls":[{"name":"items","type":"collection","request":{"method":"GET","path":"/items","query":{"size":10}},"pagination":{"page":"page","pageSize":"size"},"children":[{"name":"details","type":"single","request":{"method":"GET","path":"/items/{parent.id}"},"cache":false}]}]}}`

func TestNormalization(t *testing.T) {
	parsed, err := schema.Parse([]byte(input), 64)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(parsed, application.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	root := compiled.Roots[0]
	if root.Pointer != "/definition/calls/0" || root.RootIndex != 0 || root.Cache.TTL != time.Hour || root.Children[0].Cache.Enabled || root.Pagination.Start != 1 || !root.Pagination.Pages.All {
		t.Fatalf("wrong defaults: %#v", root)
	}
	if compiled.API.Auth.Type != "none" || compiled.API.Limits.Concurrency != 20 {
		t.Fatalf("wrong API defaults: %#v", compiled.API)
	}
	encoded, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "References") {
		t.Fatal("compiled references not persistable")
	}
}
func TestSemanticFailures(t *testing.T) {
	invalids := []string{
		strings.Replace(input, `"/items/{parent.id}"`, `"/items/{missing.id}"`, 1),
		strings.Replace(input, `"/items"`, `"/items/{parent.id}"`, 1),
		strings.Replace(input, `"size":10`, `"size":"10"`, 1),
		strings.Replace(input, `"size":10`, `"size":10,"page":1`, 1),
		strings.Replace(input, `"size":10`, `"size":1.5`, 1),
		strings.Replace(input, `"1h"`, `"31d"`, 1),
		strings.Replace(input, `https://example.com/api/v1`, `https://example.com/api/v1?x=1`, 1),
		strings.Replace(input, `"/items"`, `"/../items"`, 1),
		strings.Replace(input, `"method":"GET"`, `"headers":{"Host":"evil.example"},"method":"GET"`, 1),
	}
	for _, raw := range invalids {
		parsed, err := schema.Parse([]byte(raw), 64)
		if err != nil {
			continue
		}
		if _, err := Compile(parsed, application.DefaultValidationLimits()); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestArrayRootPointerAndLimits(t *testing.T) {
	parsed, err := schema.Parse([]byte(input), 64)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(parsed, application.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Roots[0].Pointer != "/definition/calls/0" {
		t.Fatal(compiled.Roots[0].Pointer)
	}
	parsed.Definition.Calls.Calls = append(parsed.Definition.Calls.Calls, parsed.Definition.Calls.Calls[0])
	if _, err := Compile(parsed, application.DefaultValidationLimits()); err == nil {
		t.Fatal("duplicate root accepted")
	}
}
func TestHeaderAuthCollision(t *testing.T) {
	parsed, err := schema.Parse([]byte(input), 64)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Definition.API.Auth.Type = "bearer"
	parsed.Definition.API.Auth.Token = "${SECRET}"
	parsed.Definition.Calls.Calls[0].Request.Headers = map[string]string{"aUtHoRiZaTiOn": "other"}
	if _, err := Compile(parsed, application.DefaultValidationLimits()); err == nil {
		t.Fatal("header collision accepted")
	}
	parsed.Definition.Calls.Calls[0].Request.Headers = nil
	compiled, err := Compile(parsed, application.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	if compiled.API.Auth.Token != "${SECRET}" {
		t.Fatal("secret unexpectedly interpolated")
	}
}
