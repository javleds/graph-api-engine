package worker

import (
	"bytes"
	"context"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	executiondb "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func durableResultRunner(t *testing.T, fake *fakeFetcher, roots ...definition.CompiledCall) (*V2Runner, *executiondb.ExecutionStore) {
	t.Helper()
	ctx := context.Background()
	db, err := dbplatform.Open(ctx, filepath.Join(t.TempDir(), "engine.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = dbplatform.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	store := executiondb.NewExecutionStore(db, protector, 10)
	raw, _ := json.Marshal(compiled(roots...))
	now := time.Now()
	_, _, err = store.Create(ctx, execution.ExecutionSnapshot{ID: "results", OwnerID: "owner", APIVersion: 2, Definition: raw, Fingerprint: "results", CreatedAt: now, DeadlineAt: now.Add(time.Minute), MetadataExpiresAt: now.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return &V2Runner{Store: store, V2Store: store, Finalizer: executiondb.NewExecutionFinalizer(store), Engine: &Engine{Fetcher: fake}}, store
}

func TestV2ResultsCloseWholeChainsBeforeNextFetch(t *testing.T) {
	var store *executiondb.ExecutionStore
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		switch {
		case strings.HasSuffix(r.Request.URL, "/language"):
			return response(`{"name":"es"}`)
		case strings.HasSuffix(r.Request.URL, "/pokemon"):
			page, err := store.ReadV2Results(ctx, "owner", "results", 0, 0, 100, time.Now())
			if err != nil || len(page.Results) != 1 || page.Results[0].Root != "language" {
				t.Fatalf("single not committed before collection fetch: %+v %v", page, err)
			}
			return response(`[{"id":1},{"id":2}]`)
		case strings.HasSuffix(r.Request.URL, "/species/1"):
			page, err := store.ReadV2Results(ctx, "owner", "results", 0, 0, 100, time.Now())
			if err != nil || len(page.Results) != 1 {
				t.Fatalf("unfinished pokemon published: %+v %v", page, err)
			}
			return response(`{"number":9007199254740993,"nested":[null,{"":false}]}`)
		case strings.HasSuffix(r.Request.URL, "/species/2"):
			page, err := store.ReadV2Results(ctx, "owner", "results", 0, 0, 100, time.Now())
			if err != nil || len(page.Results) != 2 {
				t.Fatalf("closed pokemon not committed before next child: %+v %v", page, err)
			}
			var result bytes.Buffer
			if err = store.WriteV2Result(ctx, "owner", "results", page.Results[1].NodeID, time.Now(), &result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.String(), `"data":{"id":1,"species":{"nested":[null,{"":false}],"number":9007199254740993}}`) {
				t.Fatal(result.String())
			}
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_HTTP_ERROR"}
		default:
			t.Fatalf("unexpected request %s", r.Request.URL)
			return fetch.FetchResult{}, nil
		}
	}}
	runner, s := durableResultRunner(t, fake, call("language", "single", "/language"), call("pokemon", "collection", "/pokemon", call("species", "single", "/species/{parent.id}")))
	store = s
	if err := runner.Run(context.Background(), "results"); err != nil {
		t.Fatal(err)
	}
	page, err := store.ReadV2Results(context.Background(), "owner", "results", 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 3 || page.Results[2].Status != "completed_with_errors" || page.Results[2].ErrorSummary.Total != 1 {
		t.Fatalf("results %+v %v", page, err)
	}
	var result bytes.Buffer
	if err = store.WriteV2Result(context.Background(), "owner", "results", page.Results[2].NodeID, time.Now(), &result); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(result.Bytes()) || !strings.Contains(result.String(), `"species":null`) || !strings.Contains(result.String(), `UPSTREAM_HTTP_ERROR`) {
		t.Fatal(result.String())
	}
}

func TestV2ResultFailuresAndEmptyCollections(t *testing.T) {
	for _, tc := range []v2ResultCase{
		{"null", "null", "completed", false, false},
		{"scalar", "7", "completed", true, false},
		{"not_enrichable", "null", "failed", true, false},
		{"collision", `{"child":1}`, "failed", true, false},
		{"upstream", "", "failed", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := call("root", "single", "/root")
			if tc.child {
				root.Children = []definition.CompiledCall{call("child", "single", "/child")}
			}
			fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
				if strings.HasSuffix(r.Request.URL, "/child") {
					return response(`true`)
				}
				if tc.fetchFailure {
					return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_HTTP_ERROR"}
				}
				if strings.HasSuffix(r.Request.URL, "/empty") {
					return response(`[]`)
				}
				return response(tc.body)
			}}
			runner, store := durableResultRunner(t, fake, root, call("empty", "collection", "/empty"))
			if err := runner.Run(context.Background(), "results"); err != nil {
				t.Fatal(err)
			}
			page, err := store.ReadV2Results(context.Background(), "owner", "results", 0, 0, 100, time.Now())
			if err != nil || len(page.Results) != 1 || page.Results[0].Status != tc.status {
				t.Fatalf("result %+v %v", page, err)
			}
			var body bytes.Buffer
			if err = store.WriteV2Result(context.Background(), "owner", "results", page.Results[0].NodeID, time.Now(), &body); err != nil || !json.Valid(body.Bytes()) {
				t.Fatalf("%s %v", body.String(), err)
			}
		})
	}
}
