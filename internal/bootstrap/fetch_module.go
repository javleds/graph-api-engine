package bootstrap

import (
	"context"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetchapp "github.com/javleds/api-engine/internal/fetch/application"
	fetchdomain "github.com/javleds/api-engine/internal/fetch/domain"
	fetchhttp "github.com/javleds/api-engine/internal/fetch/infrastructure/http"
	"github.com/javleds/api-engine/internal/fetch/infrastructure/local"
	fetchsql "github.com/javleds/api-engine/internal/fetch/infrastructure/sqlite"
	"github.com/javleds/api-engine/internal/platform/clock"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"github.com/javleds/api-engine/internal/security/infrastructure/egress"
	"time"
)

func (a *Application) buildFetch(protector securityports.SecretProtector, identity []byte) (*fetchapp.CachingFetchService, error) {
	cfg := a.Config
	policy, err := egress.NewPolicy(cfg.Security.Egress.UpstreamAllowedOrigins, cfg.Security.Egress.AllowedPrivateCIDRs, cfg.Security.Egress.AllowHTTPForDevelopment, cfg.Security.Egress.AllowAnyOriginForDevelopment)
	if err != nil {
		return nil, err
	}
	options := fetchhttp.TransportOptions{
		ConnectTimeout:            cfg.Fetch.ConnectTimeout.Value(),
		TLSHandshakeTimeout:       cfg.Fetch.TlsHandshakeTimeout.Value(),
		ResponseHeaderTimeout:     cfg.Fetch.ResponseHeaderTimeout.Value(),
		MaxResponseBytes:          int64(cfg.Fetch.MaxResponseBytes),
		MaxIdleConnections:        cfg.Fetch.MaxIdleConnections,
		MaxIdleConnectionsPerHost: cfg.Fetch.MaxIdleConnectionsPerHost,
		IdleConnectionTimeout:     cfg.Fetch.IdleConnectionTimeout.Value(),
	}
	a.Transport, err = fetchhttp.NewNetHTTPTransport(policy, options)
	if err != nil {
		return nil, err
	}
	raw := fetchapp.NewFetchService(measuredTransport{transport: a.Transport, metrics: a.Metrics})
	gate := local.NewLocalAdmissionGate(local.GateOptions{
		MaxConcurrent:    cfg.Fetch.MaxConcurrentHTTPRequests,
		MaxScopes:        cfg.Throttle.MaxQuotaScopes,
		Utilization:      cfg.Throttle.Utilization,
		ProcessStartedAt: time.Now(),
		AttemptTimeout:   cfg.Fetch.AttemptTimeout.Value(),
		SafetyMargin:     cfg.Throttle.SafetyMargin.Value(),
	}, &fetchsql.SQLiteQuotaCooldownStore{DB: a.EngineDB}, clock.RealClock{})
	raw.Gate = measuredGate{gate: gate, metrics: a.Metrics}
	raw.MaxJSONDepth = cfg.Fetch.MaxJSONDepth
	raw.MaxJSONValues = cfg.Fetch.MaxJSONValuesPerResponse
	raw.OnAttempt = func() { a.Metrics.Add("http_attempts_total", 1) }
	raw.OnRetry = func() { a.Metrics.Add("retries_total", 1) }
	a.Cache = fetchsql.NewSQLiteResponseCache(a.CacheDB, protector, fetchsql.CacheLimits{
		MaxEntryBytes:   int64(cfg.Cache.MaxEntryBytes),
		MaxEntries:      int64(cfg.Cache.MaxEntries),
		MaxLogicalBytes: int64(cfg.Cache.MaxLogicalBytes),
		MaxTTL:          cfg.Cache.MaxTTL.Value(),
	})
	service := fetchapp.NewCachingFetchService(raw, measuredCache{cache: a.Cache, app: a}, identity)
	service.MaxTTL = cfg.Cache.MaxTTL.Value()
	service.OnCacheError = func(error) { a.Metrics.Add("cache_errors_total", 1) }
	flights := local.NewLocalFlightCoordinator(cfg.Deduplication.MaxActiveFlights, cfg.Deduplication.MaxOperationTime.Value())
	service.Flights = flights
	a.activeFlights = flights.ActiveFlights
	groups := []fetchdomain.QuotaGroup{}
	for _, group := range cfg.Throttle.QuotaGroups {
		groups = append(groups, fetchdomain.QuotaGroup{
			Name:           group.Name,
			Origins:        group.Origins,
			CredentialMode: group.CredentialMode,
			Limits:         fetchdomain.QuotaRule{Requests: group.Limits.Requests, Period: group.Limits.Per.Value(), Concurrency: group.Limits.Concurrency},
		})
	}
	resolver, err := fetchapp.NewConfiguredQuotaScopeResolver(identity, groups)
	if err != nil {
		return nil, err
	}
	service.QuotaResolver = resolver
	a.registerQuotas = func(ctx context.Context, compiled definition.CompiledDefinition) error {
		rules, err := fetchapp.DeclaredQuotaRules(compiled, resolver)
		if err == nil {
			err = gate.Register(ctx, rules)
		}
		if err != nil {
			var failure *fetchdomain.UpstreamFailure
			if errors.As(err, &failure) && failure.Code == "QUOTA_POLICY_CONFLICT" {
				return &definition.ValidationError{Code: failure.Code, Path: "/definition", Message: "Conflicting active quota policies."}
			}
			return execution.StorageError("ENGINE_BUSY")
		}
		return nil
	}
	return service, nil
}
