package application

import (
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
	"time"
)

func TestDeclaredQuotaRulesMatchRuntimeScopesAndMergeOmissions(t *testing.T) {
	resolver, err := NewConfiguredQuotaScopeResolver([]byte("key"), []domain.QuotaGroup{{Name: "group", Origins: []string{"https://example.com"}, Limits: domain.QuotaRule{Requests: 100, Period: time.Minute}}})
	if err != nil {
		t.Fatal(err)
	}
	limited := definition.CompiledCall{Request: definition.RequestTemplate{Method: "GET", Path: definition.Template{Source: "/items/{parent.id}"}}, Limits: &definition.RatePolicy{Requests: 10, Per: time.Minute}}
	omitted := limited
	omitted.Limits = nil
	omitted.Request.Path.Source = "/items/{items.id}"
	compiled := definition.CompiledDefinition{API: definition.APIConnection{BaseURL: "https://example.com/v2", Auth: definition.Authentication{Type: "bearer", Token: "secret"}, Limits: definition.RatePolicy{Requests: 100, Per: time.Minute}}, Roots: []definition.CompiledCall{omitted, limited}}
	rules, err := DeclaredQuotaRules(compiled, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected provider, endpoint and group; got %d", len(rules))
	}
	identity, _ := json.Marshal(compiled.API.Auth)
	provider, _ := resolver.ProviderScope(compiled.API.BaseURL, string(identity))
	endpoint, _ := resolver.EndpointScopeForBase(compiled.API.BaseURL, provider, "GET", limited.Request.Path.Source)
	found := false
	for _, rule := range rules {
		if rule.Scope == endpoint {
			found = true
			if rule.Requests != 10 {
				t.Fatal("omission erased restriction")
			}
		}
	}
	if !found {
		t.Fatal("runtime endpoint scope missing")
	}
	conflicting := limited
	conflicting.Limits = &definition.RatePolicy{Requests: 20, Per: time.Minute}
	compiled.Roots = append(compiled.Roots, conflicting)
	if _, err = DeclaredQuotaRules(compiled, resolver); err == nil || err.Error() != "QUOTA_POLICY_CONFLICT" {
		t.Fatalf("%v", err)
	}
}

func TestDeclaredQuotaRulesPreserveConcurrencyOnlyPolicies(t *testing.T) {
	resolver := NewQuotaScopeResolver([]byte("key"))
	call := definition.CompiledCall{Request: definition.RequestTemplate{Method: "GET", Path: definition.Template{Source: "/items"}}}
	limited := call
	limited.Limits = &definition.RatePolicy{Concurrency: 2}
	compiled := definition.CompiledDefinition{API: definition.APIConnection{BaseURL: "https://example.com", Limits: definition.RatePolicy{Requests: 100, Per: time.Minute}}, Roots: []definition.CompiledCall{call, limited}}
	rules, err := DeclaredQuotaRules(compiled, resolver)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range rules {
		if rule.Concurrency == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("concurrency-only restriction erased")
	}
	conflict := limited
	conflict.Limits = &definition.RatePolicy{Concurrency: 3}
	compiled.Roots = append(compiled.Roots, conflict)
	if _, err = DeclaredQuotaRules(compiled, resolver); err == nil {
		t.Fatal("concurrency conflict accepted")
	}
}
