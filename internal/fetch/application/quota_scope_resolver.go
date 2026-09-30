package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type QuotaScopeResolver struct {
	secret []byte
	groups []domain.QuotaGroup
}

func NewQuotaScopeResolver(secret []byte) *QuotaScopeResolver {
	return &QuotaScopeResolver{secret: append([]byte(nil), secret...)}
}

func (r *QuotaScopeResolver) ProviderScope(baseURL, credentialIdentity string) (string, error) {
	origin, err := canonicalOrigin(baseURL)
	if err != nil {
		return "", err
	}
	return r.digest([]string{"provider:v1", origin, credentialIdentity}), nil
}

// ProviderRule selects the effective provider quota for a definition. A managed
// group owns the rate when one covers the origin; otherwise each declared rate
// has its own scope so concurrent definitions can use different limits.
func (r *QuotaScopeResolver) ProviderRule(baseURL, credentialIdentity string, limits definition.RatePolicy) (domain.QuotaRule, error) {
	provider, err := r.ProviderScope(baseURL, credentialIdentity)
	if err != nil {
		return domain.QuotaRule{}, err
	}
	managed, err := r.ManagedRules(baseURL, credentialIdentity)
	if err != nil {
		return domain.QuotaRule{}, err
	}
	if len(managed) > 0 {
		return domain.QuotaRule{Scope: provider}, nil
	}
	scope := r.digest([]string{"provider-policy:v1", provider, strconv.FormatInt(limits.Requests, 10), limits.Per.String(), strconv.Itoa(limits.Concurrency)})
	return domain.QuotaRule{Scope: scope, Requests: int(limits.Requests), Period: limits.Per, Concurrency: limits.Concurrency}, nil
}

func canonicalOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("invalid provider origin")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port), nil
}

func (r *QuotaScopeResolver) EndpointScope(providerScope, method, template string) string {
	normalized := regexp.MustCompile(`\{[^{}]+\}`).ReplaceAllString(template, "{}")
	return r.digest([]string{"endpoint:v1", providerScope, strings.ToUpper(method), normalized})
}

func (r *QuotaScopeResolver) digest(fields []string) string {
	encoded, _ := json.Marshal(fields)
	mac := hmac.New(sha256.New, r.secret)
	_, _ = mac.Write(encoded)
	return hex.EncodeToString(mac.Sum(nil))
}

func (r *QuotaScopeResolver) ManagedRules(baseURL, credentialIdentity string) ([]domain.QuotaRule, error) {
	origin, err := canonicalOrigin(baseURL)
	if err != nil {
		return nil, err
	}
	var rules []domain.QuotaRule
	for _, group := range r.groups {
		for _, candidate := range group.Origins {
			if candidate != origin {
				continue
			}
			credential := credentialIdentity
			if group.CredentialMode == "shared" {
				credential = ""
			}
			rule := group.Limits
			rule.Scope = r.digest([]string{"quota-group:v1", group.Name, credential})
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func (r *QuotaScopeResolver) EndpointScopeForBase(baseURL, providerScope, method, template string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", errors.New("invalid provider base URL")
	}
	return r.EndpointScope(providerScope, method, strings.TrimSuffix(base.EscapedPath(), "/")+template), nil
}
