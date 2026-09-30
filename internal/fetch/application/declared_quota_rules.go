package application

import (
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"sort"
)

// DeclaredQuotaRules uses the same credential and endpoint identities as fetches.
// Registration of these rules validates conflicts without consuming HTTP quota.
func DeclaredQuotaRules(compiled definition.CompiledDefinition, resolver *QuotaScopeResolver) ([]domain.QuotaRule, error) {
	if resolver == nil {
		return nil, errors.New("quota scope resolver required")
	}
	identity, err := json.Marshal(compiled.API.Auth)
	if err != nil {
		return nil, err
	}
	providerRule, err := resolver.ProviderRule(compiled.API.BaseURL, string(identity), compiled.API.Limits)
	if err != nil {
		return nil, err
	}
	provider := providerRule.Scope
	rules := map[string]domain.QuotaRule{provider: providerRule}
	managed, err := resolver.ManagedRules(compiled.API.BaseURL, string(identity))
	if err != nil {
		return nil, err
	}
	for _, rule := range managed {
		rules[rule.Scope] = rule
	}
	if err = collectCallQuotas(compiled.Roots, compiled.API.BaseURL, provider, resolver, rules); err != nil {
		return nil, err
	}
	scopes := make([]string, 0, len(rules))
	for scope := range rules {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	result := make([]domain.QuotaRule, 0, len(scopes))
	for _, scope := range scopes {
		result = append(result, rules[scope])
	}
	return result, nil
}

func collectCallQuotas(calls []definition.CompiledCall, base, provider string, resolver *QuotaScopeResolver, rules map[string]domain.QuotaRule) error {
	for _, call := range calls {
		scope, err := resolver.EndpointScopeForBase(base, provider, call.Request.Method, call.Request.Path.Source)
		if err != nil {
			return err
		}
		rule := domain.QuotaRule{Scope: scope}
		if call.Limits != nil {
			rule.Requests = int(call.Limits.Requests)
			rule.Period = call.Limits.Per
			rule.Concurrency = call.Limits.Concurrency
		}
		existing, exists := rules[scope]
		if exists && quotaDeclared(existing) && quotaDeclared(rule) && existing != rule {
			return &domain.UpstreamFailure{Code: "QUOTA_POLICY_CONFLICT"}
		}
		if !exists || quotaDeclared(rule) {
			rules[scope] = rule
		}
		if err = collectCallQuotas(call.Children, base, provider, resolver, rules); err != nil {
			return err
		}
	}
	return nil
}

func quotaDeclared(rule domain.QuotaRule) bool { return rule.Requests > 0 || rule.Concurrency > 0 }
