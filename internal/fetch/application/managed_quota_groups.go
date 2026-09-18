package application

import (
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

func NewConfiguredQuotaScopeResolver(secret []byte, groups []domain.QuotaGroup) (*QuotaScopeResolver, error) {
	resolver := NewQuotaScopeResolver(secret)
	origins := map[string]bool{}
	names := map[string]bool{}
	for _, group := range groups {
		if group.Name == "" || names[group.Name] || len(group.Origins) == 0 || group.Limits.Requests <= 0 || group.Limits.Period <= 0 || group.Limits.Concurrency < 0 {
			return nil, errors.New("invalid quota group")
		}
		if group.CredentialMode == "" {
			group.CredentialMode = "separate"
		}
		if group.CredentialMode != "shared" && group.CredentialMode != "separate" {
			return nil, errors.New("invalid quota credential mode")
		}
		names[group.Name] = true
		normalized := make([]string, 0, len(group.Origins))
		for _, raw := range group.Origins {
			origin, err := canonicalOrigin(raw)
			if err != nil {
				return nil, err
			}
			if origins[origin] {
				return nil, errors.New("overlapping quota group origins")
			}
			origins[origin] = true
			normalized = append(normalized, origin)
		}
		group.Origins = normalized
		resolver.groups = append(resolver.groups, group)
	}
	return resolver, nil
}
