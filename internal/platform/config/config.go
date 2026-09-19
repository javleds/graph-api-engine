package config

import (
	"fmt"
	"github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/domain"
	"net/url"
	"path"
	"strings"
)

type Config struct {
	Version       int                 `json:"version"`
	Mode          string              `json:"mode"`
	HTTP          HTTPConfig          `json:"http"`
	Executions    ExecutionsConfig    `json:"executions"`
	Fetch         FetchConfig         `json:"fetch"`
	Throttle      ThrottleConfig      `json:"throttle"`
	Deduplication DeduplicationConfig `json:"deduplication"`
	Storage       StorageConfig       `json:"storage"`
	Cache         CacheConfig         `json:"cache"`
	Webhooks      WebhooksConfig      `json:"webhooks"`
	Security      SecurityConfig      `json:"security"`
	Maintenance   MaintenanceConfig   `json:"maintenance"`
	Shutdown      ShutdownConfig      `json:"shutdown"`
	Observability ObservabilityConfig `json:"observability"`
}

func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported configuration version")
	}
	if c.Mode != "local" || c.Storage.Driver != "sqlite" || c.Cache.Driver != "sqlite" || c.Throttle.Driver != "local_pacing" || c.Deduplication.Driver != "local_singleflight" {
		return &domain.ValidationError{Code: "UNSUPPORTED_DRIVER", Message: "unsupported service mode or driver"}
	}
	if err := positiveValues(*c); err != nil {
		return err
	}
	if err := validateHTTP(c.HTTP, c.Security); err != nil {
		return err
	}
	if err := validateSecurity(c.Security); err != nil {
		return err
	}
	if err := validateStorage(c.Storage, c.Cache); err != nil {
		return err
	}
	e := c.Executions
	if e.V2.MaxPublishedBytes < 262144 || e.V2.MaxStoredBytes < e.V2.MaxPublishedBytes {
		return fmt.Errorf("invalid v2 storage budgets")
	}
	if e.MaxRootCalls > 128 || e.Errors.MaxDetails < 1 || e.Errors.MaxReportBytes < 32768 || e.Errors.MaxMessageBytes < 64 || e.Errors.MaxMessageBytes > 2048 || e.MaxResultBytes <= e.Errors.MaxReportBytes || e.MaxResultBytes-e.Errors.MaxReportBytes <= 65536 {
		return fmt.Errorf("invalid execution or error report budgets")
	}
	if c.Throttle.Utilization <= 0 || c.Throttle.Utilization > 1 || c.Throttle.ColdStart != "wait_one_window" {
		return fmt.Errorf("invalid throttle utilization or cold start policy")
	}
	if err := validateQuotaGroups(c.Throttle.QuotaGroups); err != nil {
		return err
	}
	if c.Webhooks.MaxPayloadBytes < 16384 || c.Webhooks.ClaimLease <= c.Webhooks.AttemptTimeout+c.Shutdown.PersistenceTimeout {
		return fmt.Errorf("webhook payload or claim lease is insufficient")
	}
	if c.Storage.MetadataRetention <= c.Storage.ResultRetention || c.Storage.MetadataRetention <= c.Webhooks.DeliveryMaxAge {
		return fmt.Errorf("metadata retention must exceed result and delivery retention")
	}
	if c.Fetch.Retry.MaxBackoff < c.Fetch.Retry.BaseDelay || c.Webhooks.MaxBackoff < c.Webhooks.BaseDelay {
		return fmt.Errorf("maximum retry backoff is below base delay")
	}
	switch c.Observability.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level")
	}
	if c.Observability.MetricsPath == "" || c.Observability.MetricsPath[0] != '/' {
		return fmt.Errorf("metrics path must be absolute")
	}
	decodedPath, err := url.PathUnescape(c.Observability.MetricsPath)
	if err != nil {
		return fmt.Errorf("metrics path has invalid escaping")
	}
	metricsPath := path.Clean(decodedPath)
	if metricsPath == "/playground" || strings.HasPrefix(metricsPath, "/playground/") {
		return fmt.Errorf("metrics path conflicts with playground routes")
	}
	return nil
}
func (c Config) DefinitionLimits() application.ValidationLimits {
	return application.ValidationLimits{MaxDefinitionDepth: c.Executions.MaxDefinitionDepth, MaxDefinitionNodes: c.Executions.MaxDefinitionNodes, MaxChildrenPerNode: c.Executions.MaxChildrenPerNode, MaxRootCalls: c.Executions.MaxRootCalls, HTTPConcurrency: c.Fetch.MaxConcurrentHTTPRequests, MaxTTL: c.Cache.MaxTTL.Value(), AllowHTTPForDevelopment: c.Security.Egress.AllowHTTPForDevelopment, AllowAnyOriginForDevelopment: c.Security.Egress.AllowAnyOriginForDevelopment, UpstreamAllowedOrigins: append([]string(nil), c.Security.Egress.UpstreamAllowedOrigins...), WebhookAllowedOrigins: append([]string(nil), c.Security.Egress.WebhookAllowedOrigins...)}
}
