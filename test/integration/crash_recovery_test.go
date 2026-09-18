package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/api/http/mapper"
	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/config"
)

// Simulates abrupt process loss after durable acceptance/claim, without graceful shutdown.
func TestCrashRecoveryDoesNotReexecuteAcceptedTrees(t *testing.T) {
	if path := os.Getenv("ENGINE_CRASH_FIXTURE"); path != "" {
		writeInterruptedJobs(t, path)
		os.Exit(0)
	}
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`[]`)) }))
	defer upstream.Close()
	cfg := fixtureConfig(t, upstream.URL, upstream.URL)
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCrashRecoveryDoesNotReexecuteAcceptedTrees$")
	child.Env = append(os.Environ(), "ENGINE_CRASH_FIXTURE="+path)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("crash fixture: %v: %s", err, output)
	}
	fixture := openFixture(t, cfg)
	for _, id := range []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"} {
		response := fixture.terminal(t, id)
		if response["status"] != "failed" {
			t.Fatalf("interrupted job not failed: %#v", response)
		}
		failure, ok := response["error"].(map[string]any)
		if !ok || failure["code"] != "PROCESS_INTERRUPTED" {
			t.Fatalf("wrong recovery error: %#v", response)
		}
		if status, _ := fixture.request(t, "GET", "/v1/executions/"+id+"/result", fixture.token, "", nil); status != 409 {
			t.Fatalf("interrupted result status %d", status)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("recovery performed an upstream request")
	}
}
func writeInterruptedJobs(t *testing.T, path string) {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err := bootstrap.Build(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := schema.Parse(definitionBody(cfg.Security.Egress.UpstreamAllowedOrigins[0], nil), 64)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := mapper.Compile(parsed, cfg.DefinitionLimits())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []execution.ExecutionID{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"} {
		job, _, err := app.Store.Create(context.Background(), execution.ExecutionSnapshot{ID: id, OwnerID: "owner-one", Fingerprint: string(id), Definition: definition, CreatedAt: time.Now(), MetadataExpiresAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			claimed, err := app.Store.Claim(context.Background(), id, "dead-process", job.Version, time.Now())
			if err != nil || !claimed {
				t.Fatalf("claim: %v %v", claimed, err)
			}
		}
	}
	// Deliberately leave pools and the advisory lock open: process exit must release them.
}
