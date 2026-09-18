package application

import (
	"github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
	"time"
)

func TestQuotaScopesShareOriginAndNormalizeAliases(t *testing.T) {
	resolver := NewQuotaScopeResolver([]byte("stable-key"))
	first, _ := resolver.ProviderScope("https://EXAMPLE.com/v1", "token")
	second, _ := resolver.ProviderScope("https://example.com:443/v2", "token")
	if first != second {
		t.Fatal("provider scope split by path")
	}
	if resolver.EndpointScope(first, "GET", "/games/{parent.id}") != resolver.EndpointScope(first, "GET", "/games/{games.id}") {
		t.Fatal("aliases created new quotas")
	}
	other, _ := resolver.ProviderScope("https://example.com", "other-token")
	if first == other {
		t.Fatal("credentials collapsed")
	}
}

func TestManagedQuotaGroupsShareCredentialsWithoutRemovingDeclaredRules(t *testing.T) {
	resolver, err := NewConfiguredQuotaScopeResolver([]byte("key"), []domain.QuotaGroup{{Name: "account", Origins: []string{"https://a.example.com", "https://b.example.com"}, CredentialMode: "shared", Limits: domain.QuotaRule{Requests: 100, Period: time.Minute}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := resolver.ManagedRules("https://a.example.com/v1", "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolver.ManagedRules("https://b.example.com/v2", "two")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].Scope != second[0].Scope {
		t.Fatal("managed group not shared")
	}
}
