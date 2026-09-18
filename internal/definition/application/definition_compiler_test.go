package application

import (
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/definition/domain"
)

func validDefinition() domain.CompiledDefinition {
	return domain.CompiledDefinition{Version: 1, API: domain.APIConnection{BaseURL: "https://example.com/api", Limits: domain.RatePolicy{Requests: 1, Per: time.Second}}, Roots: []domain.CompiledCall{{Name: "items", Type: "single", Pointer: "/definition/calls", Request: domain.RequestTemplate{Method: "GET", Path: domain.Template{Source: "/items"}}}}}
}
func TestCompilerRejectsUnsafeOrIncoherentDefinitions(t *testing.T) {
	cases := map[string]func(*domain.CompiledDefinition){
		"version":          func(d *domain.CompiledDefinition) { d.Version = 2 },
		"empty roots":      func(d *domain.CompiledDefinition) { d.Roots = nil },
		"missing rate":     func(d *domain.CompiledDefinition) { d.API.Limits = domain.RatePolicy{} },
		"incomplete rate":  func(d *domain.CompiledDefinition) { d.API.Limits.Per = 0 },
		"negative rate":    func(d *domain.CompiledDefinition) { d.API.Limits.Concurrency = -1 },
		"api TTL":          func(d *domain.CompiledDefinition) { d.API.Cache.TTL = 31 * 24 * time.Hour },
		"empty node limit": func(d *domain.CompiledDefinition) { d.Roots[0].Limits = &domain.RatePolicy{} },
		"node TTL":         func(d *domain.CompiledDefinition) { d.Roots[0].Cache.TTL = 31 * 24 * time.Hour },
		"name":             func(d *domain.CompiledDefinition) { d.Roots[0].Name = "parent" },
		"type":             func(d *domain.CompiledDefinition) { d.Roots[0].Type = "other" },
		"method":           func(d *domain.CompiledDefinition) { d.Roots[0].Request.Method = "POST" },
		"query ancestor":   func(d *domain.CompiledDefinition) { d.Roots[0].Request.Query = map[string]any{"q": "{missing.id}"} },
		"bad escape":       func(d *domain.CompiledDefinition) { d.Roots[0].Request.Path.Source = "/%zz" },
		"scheme":           func(d *domain.CompiledDefinition) { d.API.BaseURL = "http://example.com" },
		"fragment":         func(d *domain.CompiledDefinition) { d.API.BaseURL = "https://example.com/#" },
		"credentials":      func(d *domain.CompiledDefinition) { d.API.BaseURL = "https://user:secret@example.com" },
		"header casing": func(d *domain.CompiledDefinition) {
			d.API.Headers = map[string]string{"Accept": "one", "accept": "two"}
		},
		"auth header": func(d *domain.CompiledDefinition) {
			d.API.Auth = domain.Authentication{Type: "header", Name: "Host", Value: "other"}
		},
		"auth controls": func(d *domain.CompiledDefinition) {
			d.API.Auth = domain.Authentication{Type: "bearer", Token: "secret\r\n"}
		},
		"pagination single": func(d *domain.CompiledDefinition) {
			d.Roots[0].Pagination = &domain.Pagination{Page: "p", Pages: domain.PageSelection{All: true}}
		},
		"webhook reserved": func(d *domain.CompiledDefinition) {
			d.Webhook = &domain.Webhook{URL: "https://receiver.example.com", Headers: map[string]string{"X-Engine-Signature": "fake"}}
		},
		"webhook URL": func(d *domain.CompiledDefinition) { d.Webhook = &domain.Webhook{URL: "file:///etc/passwd"} },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			d := validDefinition()
			change(&d)
			if _, err := Compile(d, DefaultValidationLimits()); err == nil {
				t.Fatal("invalid definition accepted")
			}
		})
	}
}
func TestCompilerOperatorLimitsAndWebhookDefaults(t *testing.T) {
	limits := DefaultValidationLimits()
	limits.UpstreamAllowedOrigins = []string{"https://other.example.com"}
	if _, err := Compile(validDefinition(), limits); err == nil {
		t.Fatal("allowlist ignored")
	}
	limits.UpstreamAllowedOrigins = []string{"https://example.com"}
	limits.WebhookAllowedOrigins = []string{"https://receiver.example.com"}
	d := validDefinition()
	d.Webhook = &domain.Webhook{URL: "https://receiver.example.com/path"}
	result, err := Compile(d, limits)
	if err != nil {
		t.Fatal(err)
	}
	if result.Webhook.Method != "POST" || result.Webhook.Payload != "status" || result.Webhook.Auth.Type != "none" {
		t.Fatal("webhook defaults missing")
	}
	d.Roots[0].Children = []domain.CompiledCall{d.Roots[0]}
	limits.MaxDefinitionDepth = 1
	if _, err := Compile(d, limits); err == nil {
		t.Fatal("depth limit ignored")
	}
	limits = DefaultValidationLimits()
	limits.AllowHTTPForDevelopment = true
	d = validDefinition()
	d.API.BaseURL = "http://example.com"
	d.Roots[0].Limits = &domain.RatePolicy{Concurrency: 1}
	if _, err := Compile(d, limits); err != nil {
		t.Fatal(err)
	}
}
