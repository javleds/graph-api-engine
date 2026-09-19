package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	executiondb "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestV2LargeDurableResult(t *testing.T) {
	pages := 24
	if os.Getenv("ENGINE_V2_LARGE") == "1" {
		pages = 1024
	}
	// Leave room for chunk envelopes under the configured one-GiB published cap.
	const scalarBytes = 128*1024 - 512
	body := `["` + strings.Repeat("x", scalarBytes) + `"`
	body += strings.Repeat(`,"`+strings.Repeat("x", scalarBytes)+`"`, 7) + `]`
	ctx := context.Background()
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "engine.db")
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	open := func() (*sql.DB, *executiondb.ExecutionStore) {
		db, err := dbplatform.Open(ctx, databasePath, false)
		if err != nil {
			t.Fatal(err)
		}
		if err = dbplatform.Migrate(ctx, db, "engine"); err != nil {
			db.Close()
			t.Fatal(err)
		}
		return db, executiondb.NewExecutionStore(db, protector, 1)
	}
	db, store := open()
	defer func() { db.Close() }()
	root := call("root", "collection", "/root")
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{Count: int64(pages)}}
	compiledJSON, _ := json.Marshal(compiled(root))
	now := time.Now().UTC()
	_, _, err = store.Create(ctx, execution.ExecutionSnapshot{ID: "large", OwnerID: "owner", APIVersion: 2, Definition: compiledJSON, Fingerprint: "large", CreatedAt: now, MetadataExpiresAt: now.Add(48 * time.Hour), DeadlineAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(body) }}
	fault := &v2LargeFaultStore{V2Store: store, interruptAfter: int64(pages/2) * 8 * scalarBytes}
	runner := V2Runner{Store: store, V2Store: fault, Finalizer: executiondb.NewExecutionFinalizer(store), Engine: &Engine{Fetcher: fake, Options: Options{MaxPagesPerCall: pages + 1, DetectRepeatedPages: true}}}
	// Each page is distinct, exercising normal repeated-page detection as well.
	fake.handle = func(_ context.Context, request fetch.FetchRequest) (fetch.FetchResult, error) {
		page := request.Request.URL[strings.LastIndex(request.Request.URL, "=")+1:]
		return response(strings.Replace(body, `["`, `["`+page+":", 1))
	}
	var peak atomic.Uint64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				for old := peak.Load(); memory.HeapAlloc > old && !peak.CompareAndSwap(old, memory.HeapAlloc); old = peak.Load() {
				}
			}
		}
	}()
	defer func() { close(stop); <-sampled }()
	started := time.Now()
	if err = runner.Run(ctx, "large"); err == nil {
		t.Fatal("expected simulated interruption")
	}
	_, interrupted, err := store.Load(ctx, "large")
	if err != nil || interrupted.Status != execution.Running {
		t.Fatal(interrupted.Status, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, store = open()
	if err = store.RecoverV2(ctx); err != nil {
		t.Fatal(err)
	}
	runner.Store = store
	runner.V2Store = store
	runner.Finalizer = executiondb.NewExecutionFinalizer(store)
	if err = runner.Run(ctx, "large"); err != nil {
		t.Fatal(err)
	}
	_, completed, err := store.Load(ctx, "large")
	if err != nil || completed.Status != execution.Completed {
		t.Fatalf("completion=%+v err=%v", completed, err)
	}
	state, err := store.ReadV2State(ctx, "large")
	if err != nil {
		t.Fatal(err)
	}
	minimum := int64(pages) * 8 * scalarBytes
	if state.PublishedBytes < minimum || state.PublishedBytes > 1<<30 || !state.Terminal || state.Recoveries != 1 {
		t.Fatalf("state=%+v minimum=%d", state, minimum)
	}
	for page := 1; page <= pages; page++ {
		if count := fake.count("https://provider.test/root?page=" + strconv.Itoa(page)); count != 1 {
			t.Fatalf("page %d fetched %d times", page, count)
		}
	}
	if peak.Load() >= 512<<20 {
		t.Fatalf("heap exceeded acceptance ceiling: %d", peak.Load())
	}
	var inputs int
	if err = db.QueryRow("SELECT COUNT(*) FROM v2_inputs WHERE execution_id='large'").Scan(&inputs); err != nil || inputs != 0 {
		t.Fatalf("accepted inputs not released: %d %v", inputs, err)
	}
	var diskBytes int64
	_ = filepath.Walk(directory, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			diskBytes += info.Size()
		}
		return err
	})
	t.Logf("published=%d bytes heapPeak=%d bytes disk=%d bytes pages=%d elapsed=%s recoveries=%d", state.PublishedBytes, peak.Load(), diskBytes, pages, time.Since(started).Round(time.Millisecond), state.Recoveries)
}
