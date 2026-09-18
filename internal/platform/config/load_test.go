package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func example(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../config/config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestExampleDefaultsAndDurationRoundTrip(t *testing.T) {
	config, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	if config.Executions.Timeout.Value() != 6*time.Hour || config.Cache.MaxTTL.Value() != 30*24*time.Hour {
		t.Fatal("duration defaults incorrect")
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err != nil {
		t.Fatal(err)
	}
	limits := config.DefinitionLimits()
	if limits.MaxDefinitionDepth != 8 || limits.MaxRootCalls != 16 {
		t.Fatal("definition limits do not reflect config")
	}
	limits.UpstreamAllowedOrigins[0] = "changed"
	if config.Security.Egress.UpstreamAllowedOrigins[0] == "changed" {
		t.Fatal("definition limits alias configuration")
	}
}
func TestDefaultsRequireDeploymentSettings(t *testing.T) {
	if _, err := Parse([]byte(`{}`)); err == nil {
		t.Fatal("missing deployment settings accepted")
	}
	raw := `{"http":{"publicBaseUrl":"https://engine.example.com"},"security":{"principalsFile":"/absent/principals","identityKeyFile":"/absent/identity","encryption":{"activeKeyId":"one","keyFiles":{"one":"/absent/key"}},"egress":{"upstreamAllowedOrigins":["https://example.com"],"webhookAllowedOrigins":["https://consumer.example.com"]}}}`
	config, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if config.Executions.MaxConcurrent != 4 || config.HTTP.Address != ":8080" {
		t.Fatal("defaults missing")
	}
}
func TestRejectInvalidConfiguration(t *testing.T) {
	raw := string(example(t))
	for _, value := range []string{
		strings.Replace(raw, `"mode": "local"`, `"mode": "redis"`, 1),
		strings.Replace(raw, `"maxConcurrent": 4`, `"maxConcurrent": 0`, 1),
		strings.Replace(raw, `"maxMessageBytes": 512`, `"maxMessageBytes": 63`, 1),
		strings.Replace(raw, `"maxResultBytes": 16777216`, `"maxResultBytes": 1048576`, 1),
		strings.Replace(raw, `"claimLease": "30s"`, `"claimLease": "10s"`, 1),
		strings.Replace(raw, `"metadataRetention": "30d"`, `"metadataRetention": "1d"`, 1),
		strings.Replace(raw, `"utilization": 0.95`, `"utilization": 1.1`, 1),
		strings.Replace(raw, `"maxRootCalls": 16`, `"maxRootCalls": 129`, 1),
		strings.Replace(raw, `"version": 1`, `"version": 1,"unknown": true`, 1),
		strings.Replace(raw, `"version": 1`, `"version": 1,"version": 1`, 1),
		strings.Replace(raw, `"maxTTL": "30d"`, `"maxTTL": "0d"`, 1),
		strings.Replace(raw, `"mode": "local"`, `"mode": null`, 1),
	} {
		if _, err := Parse([]byte(value)); err == nil {
			t.Errorf("accepted invalid configuration: %s", value)
		}
	}
}
func TestExplicitEnvironmentAndFilePriority(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, example(t), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGINE_CONFIG", "/not/a/config.json")
	t.Setenv("ENGINE_HTTP_ADDRESS", "127.0.0.1:9000")
	t.Setenv("ENGINE_HTTP_PUBLIC_BASE_URL", "https://new.example.com")
	t.Setenv("ENGINE_LOG_LEVEL", "debug")
	t.Setenv("ENGINE_MAX_ROOT_CALLS", "999")
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTP.Address != "127.0.0.1:9000" || config.HTTP.PublicBaseURL != "https://new.example.com" || config.Observability.LogLevel != "debug" || config.Executions.MaxRootCalls != 16 {
		t.Fatal("incorrect environment overrides")
	}
	t.Setenv("ENGINE_CONFIG", path)
	if _, err := Load(""); err != nil {
		t.Fatal(err)
	}
}
func TestDisabledAuthenticationRestrictedToDevelopmentLoopback(t *testing.T) {
	config, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	config.Security.AuthenticationEnabled = false
	if err := config.Validate(); err == nil {
		t.Fatal("production anonymous listener accepted")
	}
	config.Security.Egress.AllowHTTPForDevelopment = true
	config.HTTP.Address = "127.0.0.1:8080"
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	config.HTTP.Address = "0.0.0.0:8080"
	if err := config.Validate(); err == nil {
		t.Fatal("public anonymous listener accepted")
	}
}
func TestQuotaGroupDefaultsAndValidation(t *testing.T) {
	config, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	config.Throttle.QuotaGroups = []QuotaGroup{{Name: "shared", Origins: []string{"https://example.com"}, Limits: QuotaLimits{Requests: 100, Per: Duration(time.Minute)}}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	if config.Throttle.QuotaGroups[0].CredentialMode != "separate" {
		t.Fatal("quota mode default missing")
	}
	config.Throttle.QuotaGroups = append(config.Throttle.QuotaGroups, config.Throttle.QuotaGroups[0])
	if err := config.Validate(); err == nil {
		t.Fatal("duplicate quota name accepted")
	}
}
