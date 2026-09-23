package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func example(t *testing.T) []byte {
	t.Helper()
	t.Setenv("APP_PORT", "8080")
	t.Setenv("APP_URL", "http://localhost:8080")
	t.Setenv("PLAYGROUND_USERNAME", "test-user")
	t.Setenv("PLAYGROUND_PASSWORD", "test-password")
	data, err := os.ReadFile("../../../config/config.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExampleDefaults(t *testing.T) {
	config, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	if config.Executions.Timeout.Value() != 6*time.Hour || config.Cache.MaxTTL.Value() != 30*24*time.Hour {
		t.Fatal("duration defaults incorrect")
	}
	if !config.UI.Enabled || config.UI.Language != "es" {
		t.Fatalf("unexpected UI defaults: %+v", config.UI)
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

func TestUILanguageEnglishIsAccepted(t *testing.T) {
	raw := strings.Replace(string(example(t)), "language: es", "language: en", 1)
	config, err := Parse([]byte(raw))
	if err != nil || config.UI.Language != "en" {
		t.Fatalf("english UI language: %+v %v", config.UI, err)
	}
}

func TestResultRetentionCoversNodeDeliveryWindow(t *testing.T) {
	config, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	config.Storage.ResultRetention = config.Webhooks.DeliveryMaxAge - 1
	if err := config.Validate(); err == nil {
		t.Fatal("result retention shorter than delivery window accepted")
	}
	config.Storage.ResultRetention = config.Webhooks.DeliveryMaxAge
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsRequireDeploymentSettings(t *testing.T) {
	if _, err := Parse([]byte("{}")); err == nil {
		t.Fatal("missing deployment settings accepted")
	}
	raw := "http:\n  publicBaseUrl: https://engine.example.com\nsecurity:\n  principalsFile: /absent/principals\n  identityKeyFile: /absent/identity\n  encryption:\n    activeKeyId: one\n    keyFiles:\n      one: /absent/key\n  playground:\n    username: test\n    password: password\n  egress:\n    upstreamAllowedOrigins: [https://example.com]\n    webhookAllowedOrigins: [https://consumer.example.com]\n"
	config, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if config.Executions.MaxConcurrent != 4 || config.Fetch.MaxConcurrentHTTPRequests != 10 || config.HTTP.Address != ":8080" {
		t.Fatal("defaults missing")
	}
}

func TestRejectInvalidConfiguration(t *testing.T) {
	raw := string(example(t))
	for _, value := range []string{
		strings.Replace(raw, "mode: local", "mode: redis", 1),
		strings.Replace(raw, "maxConcurrent: 4", "maxConcurrent: 0", 1),
		strings.Replace(raw, "maxMessageBytes: 512", "maxMessageBytes: 63", 1),
		strings.Replace(raw, "maxResultBytes: 16777216", "maxResultBytes: 1048576", 1),
		strings.Replace(raw, "claimLease: \"30s\"", "claimLease: \"10s\"", 1),
		strings.Replace(raw, "metadataRetention: \"30d\"", "metadataRetention: \"1d\"", 1),
		strings.Replace(raw, "utilization: 0.95", "utilization: 1.1", 1),
		strings.Replace(raw, "maxRootCalls: 16", "maxRootCalls: 129", 1),
		raw + "\nunknown: true\n",
		raw + "\nversion: 1\n",
		strings.Replace(raw, "maxTTL: \"30d\"", "maxTTL: \"0d\"", 1),
		strings.Replace(raw, "mode: local", "mode: null", 1),
		strings.Replace(raw, "language: es", "language: fr", 1),
	} {
		if _, err := Parse([]byte(value)); err == nil {
			t.Errorf("accepted invalid configuration: %s", value)
		}
	}
}

func TestExplicitEnvironmentAndFilePriority(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yml")
	if err := os.WriteFile(path, example(t), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGINE_CONFIG", "/not/a/config.yml")
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

func TestYAMLRejectsAmbiguousSyntax(t *testing.T) {
	for name, raw := range map[string]string{
		"alias":      "value: &value test\nother: *value\n",
		"merge":      "value: &value {one: 1}\n<<: *value\n",
		"multiple":   "version: 1\n---\nversion: 1\n",
		"non-string": "1: value\n",
		"null":       "value: null\n",
		"unknown":    "unknown: true\n",
		"duplicate":  "version: 1\nversion: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeYAML([]byte(raw), func(string) (string, bool) { return "", false }); err == nil {
				t.Fatal("accepted ambiguous YAML")
			}
		})
	}
}

func TestYAMLInterpolationPreservesScalarTypes(t *testing.T) {
	raw := "http:\n  address: $" + "{ADDRESS}\n  publicBaseUrl: https://$" + "{HOST}/api\nexecutions:\n  maxConcurrent: $" + "{CONCURRENCY}\nsecurity:\n  authenticationEnabled: $" + "{AUTHENTICATION}\n"
	values := map[string]string{
		"ADDRESS":        "127.0.0.1:8080",
		"HOST":           "engine.example.test",
		"CONCURRENCY":    "7",
		"AUTHENTICATION": "true",
	}
	config, err := decodeYAML([]byte(raw), func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTP.Address != "127.0.0.1:8080" || config.HTTP.PublicBaseURL != "https://engine.example.test/api" || config.Executions.MaxConcurrent != 7 || !config.Security.AuthenticationEnabled {
		t.Fatalf("interpolation lost scalar values: %#v", config)
	}
	if _, err := decodeYAML([]byte("http:\n  address: $"+"{MISSING}\n"), func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("missing environment variable accepted")
	}
	if _, err := decodeYAML([]byte("http:\n  address: $"+"{EMPTY}\n"), func(string) (string, bool) { return "", true }); err == nil {
		t.Fatal("empty environment variable accepted")
	}
}

func TestLoadRejectsJSONConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, example(t), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("JSON configuration accepted")
	}
}
func TestLoadAcceptsYAMLExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, example(t), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
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
