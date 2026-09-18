package application

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"time"
)

type CachingFetchService struct {
	QuotaResolver *QuotaScopeResolver
	Fetcher       ports.Fetcher
	Cache         ports.ResponseCache
	Flights       ports.FlightCoordinator
	Clock         clock.Clock
	IdentityKey   []byte
	MaxTTL        time.Duration
	OnCacheError  func(error)
}

func NewCachingFetchService(fetcher ports.Fetcher, cache ports.ResponseCache, identityKey []byte) *CachingFetchService {
	return &CachingFetchService{Fetcher: fetcher, Cache: cache, Clock: clock.RealClock{}, IdentityKey: append([]byte(nil), identityKey...), MaxTTL: 30 * 24 * time.Hour}
}

func (s *CachingFetchService) Fetch(ctx context.Context, input domain.FetchRequest) (domain.FetchResult, error) {
	input.Policy = input.Policy.WithDefaults()
	if s.QuotaResolver != nil {
		rules, err := s.QuotaResolver.ManagedRules(input.Request.URL, input.CredentialIdentity)
		if err != nil {
			return domain.FetchResult{}, err
		}
		input.Admission.Rules = append(append([]domain.QuotaRule(nil), input.Admission.Rules...), rules...)
		input.Admission.CooldownScopes = append([]string(nil), input.Admission.CooldownScopes...)
		for _, rule := range rules {
			input.Admission.CooldownScopes = append(input.Admission.CooldownScopes, rule.Scope)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, input.Policy.OperationTimeout)
	defer cancel()
	key, err := RequestKey(s.IdentityKey, input)
	if err != nil {
		return domain.FetchResult{}, err
	}
	if response, ok := s.lookup(ctx, key, input.Policy); ok {
		return domain.FetchResult{Response: response, CacheHit: true}, nil
	}
	work := func(workCtx context.Context) (domain.RawResponse, error) {
		if response, ok := s.lookup(workCtx, key, input.Policy); ok {
			return response, nil
		}
		result, err := s.Fetcher.Fetch(workCtx, input)
		if err != nil {
			return domain.RawResponse{}, err
		}
		if input.Policy.CacheEnabled && s.Cache != nil {
			if entry, ok := CacheEntryFor(key, result.Response, input.Policy.CacheTTL, s.MaxTTL, s.Clock.Now()); ok {
				s.cacheError(s.Cache.Put(workCtx, entry))
			}
		}
		return result.Response, nil
	}
	if s.Flights != nil {
		result, err := s.Flights.Do(ctx, FlightKey(s.IdentityKey, key, input), work)
		return domain.FetchResult{Response: result.Response, ReusedInFlight: result.ReusedInFlight}, err
	}
	response, err := work(ctx)
	return domain.FetchResult{Response: response}, err
}

func (s *CachingFetchService) lookup(ctx context.Context, key string, policy domain.FetchPolicy) (domain.RawResponse, bool) {
	if !policy.CacheEnabled || s.Cache == nil {
		return domain.RawResponse{}, false
	}
	entry, found, err := s.Cache.Get(ctx, key)
	s.cacheError(err)
	if err != nil || !found || !cacheFresh(entry, policy.CacheTTL, s.Clock.Now()) {
		return domain.RawResponse{}, false
	}
	response := entry.Response.Clone()
	response.Attempts = 0
	return response, true
}

func (s *CachingFetchService) cacheError(err error) {
	if err != nil && s.OnCacheError != nil {
		s.OnCacheError(err)
	}
}
