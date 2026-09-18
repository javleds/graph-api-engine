package sqlite

import (
	"context"
	platformsqlite "github.com/javleds/api-engine/internal/platform/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func TestCooldownNeverShortensAndSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "engine.db")
	db, err := platformsqlite.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = platformsqlite.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	store := &SQLiteQuotaCooldownStore{DB: db}
	until := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	if err = store.Extend(ctx, "provider", until); err != nil {
		t.Fatal(err)
	}
	if err = store.Extend(ctx, "provider", until.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = platformsqlite.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store.DB = db
	got, err := store.Get(ctx, "provider")
	if err != nil || !got.Equal(until) {
		t.Fatalf("%v %v", got, err)
	}
}
