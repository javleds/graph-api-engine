package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

func testStore(t *testing.T) (*ExecutionStore, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := dbplatform.Open(ctx, filepath.Join(t.TempDir(), "engine.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := dbplatform.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	return NewExecutionStore(db, protector, 1), db
}

func testSnapshot(id string, webhook bool) d.ExecutionSnapshot {
	now := time.Now().UTC()
	return d.ExecutionSnapshot{ID: d.ExecutionID(id), OwnerID: "owner", Fingerprint: "fingerprint", IdempotencyKeyHash: "key", Definition: []byte(`{"secret":"sensitive-token"}`), WebhookEnabled: webhook, CreatedAt: now, MetadataExpiresAt: now.Add(48 * time.Hour)}
}

func TestCreateReplayIsolationEncryptionAndCAS(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	snapshot := testSnapshot("first", false)
	execution, replay, err := store.Create(ctx, snapshot)
	if err != nil || replay {
		t.Fatalf("create: %v", err)
	}
	snapshot.ID = "other"
	again, replay, err := store.Create(ctx, snapshot)
	if err != nil || !replay || again.ID != execution.ID {
		t.Fatalf("replay: %v", err)
	}
	snapshot.Fingerprint = "changed"
	if _, _, err := store.Create(ctx, snapshot); !errors.Is(err, d.ErrIdempotencyConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if _, err := store.Read(ctx, "different-owner", execution.ID); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("owner isolation: %v", err)
	}
	var encrypted []byte
	if err := db.QueryRow("SELECT snapshot_blob FROM executions").Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, []byte("sensitive-token")) {
		t.Fatal("plaintext secret stored")
	}
	loaded, _, err := store.Load(ctx, execution.ID)
	if err != nil || !bytes.Equal(loaded.Definition, snapshot.Definition) {
		t.Fatalf("load: %v", err)
	}
	claimed, err := store.Claim(ctx, execution.ID, "instance", 0, time.Now())
	if err != nil || !claimed {
		t.Fatalf("claim: %v", err)
	}
	claimed, err = store.Claim(ctx, execution.ID, "instance", 0, time.Now())
	if err != nil || claimed {
		t.Fatalf("duplicate claim: %v", err)
	}
}

func TestWebhookBacklogAdmission(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	snapshot := testSnapshot("one", true)
	if _, _, err := store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.ID = "two"
	snapshot.IdempotencyKeyHash = "other"
	if _, _, err := store.Create(ctx, snapshot); !errors.Is(err, d.ErrEngineBusy) {
		t.Fatalf("backlog: %v", err)
	}
	snapshot.WebhookEnabled = false
	if _, _, err := store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
}
