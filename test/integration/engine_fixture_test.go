package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/platform/config"
)

type engineFixture struct {
	app        *bootstrap.Application
	server     *httptest.Server
	config     *config.Config
	token      string
	otherToken string
}

func fixtureConfig(t *testing.T, upstream, webhook string) *config.Config {
	t.Helper()
	directory := t.TempDir()
	cfg := config.Default()
	cfg.HTTP.PublicBaseURL = "http://engine.example.test"
	cfg.Security.Egress.AllowHTTPForDevelopment = true
	cfg.Security.Egress.UpstreamAllowedOrigins = []string{upstream}
	cfg.Security.Egress.WebhookAllowedOrigins = []string{webhook}
	cfg.Security.Egress.AllowedPrivateCIDRs = []string{"127.0.0.0/8"}
	cfg.Storage.Path = filepath.Join(directory, "engine.db")
	cfg.Cache.Path = filepath.Join(directory, "cache.db")
	cfg.Storage.MinimumFreeDiskBytes = 1
	cfg.Security.IdentityKeyFile = filepath.Join(directory, "identity.key")
	keyPath := filepath.Join(directory, "encryption.key")
	cfg.Security.Encryption.ActiveKeyID = "test"
	cfg.Security.Encryption.KeyFiles = map[string]string{"test": keyPath}
	for _, path := range []string{cfg.Security.IdentityKeyFile, keyPath} {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, key, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Security.PrincipalsFile = filepath.Join(directory, "principals.json")
	principals := []any{}
	for _, owner := range []string{"owner-one", "owner-two"} {
		sum := sha256.Sum256([]byte(owner + "-a-long-random-looking-integration-token"))
		principals = append(principals, map[string]any{"ownerId": owner, "tokenSha256": hex.EncodeToString(sum[:]), "enabled": true, "permissions": []string{"executions:create", "executions:read"}})
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "principals": principals})
	if err := os.WriteFile(cfg.Security.PrincipalsFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Fetch.AttemptTimeout = config.Duration(500 * time.Millisecond)
	cfg.Throttle.SafetyMargin = config.Duration(time.Millisecond)
	cfg.Fetch.Retry.BaseDelay = config.Duration(time.Millisecond)
	cfg.Webhooks.BaseDelay = config.Duration(time.Millisecond)
	return &cfg
}
func openFixture(t *testing.T, cfg *config.Config) *engineFixture {
	t.Helper()
	app, err := bootstrap.Build(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	app.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	fixture := &engineFixture{app: app, server: httptest.NewServer(app.HTTP.Handler), config: cfg, token: "owner-one-a-long-random-looking-integration-token", otherToken: "owner-two-a-long-random-looking-integration-token"}
	t.Cleanup(func() { fixture.close(t) })
	return fixture
}
func (f *engineFixture) close(t *testing.T) {
	t.Helper()
	if f.app == nil {
		return
	}
	f.server.Close()
	f.app.Dispatcher.StopAdmission()
	f.app.Dispatcher.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.app.Dispatcher.Wait(ctx); err != nil {
		t.Error(err)
	}
	f.app.Close()
	f.app = nil
}
func (f *engineFixture) request(t *testing.T, method, path, token, key string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}
func (f *engineFixture) create(t *testing.T, body []byte, key string) string {
	t.Helper()
	status, data := f.request(t, "POST", "/v1/executions", f.token, key, body)
	if status != 202 {
		t.Fatalf("create %d: %s", status, data)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	return response["id"].(string)
}
func (f *engineFixture) terminal(t *testing.T, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, data := f.request(t, "GET", "/v1/executions/"+id, f.token, "", nil)
		if status != 200 {
			t.Fatalf("status %d: %s", status, data)
		}
		var response map[string]any
		if err := json.Unmarshal(data, &response); err != nil {
			t.Fatal(err)
		}
		if response["status"] != "pending" && response["status"] != "running" {
			return response
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("execution did not finish")
	return nil
}
func definitionBody(upstream string, webhook any) []byte {
	data, _ := json.Marshal(map[string]any{"definition": map[string]any{"version": 1, "api": map[string]any{"baseUrl": upstream, "limits": map[string]any{"requests": 1000, "per": "1ms"}, "cache": map[string]any{"ttl": "1h"}}, "calls": map[string]any{"name": "items", "type": "collection", "request": map[string]any{"method": "GET", "path": "/items"}, "children": []any{map[string]any{"name": "detail", "type": "single", "request": map[string]any{"method": "GET", "path": "/detail/{parent.id}"}}}}}, "webhook": webhook})
	return data
}
