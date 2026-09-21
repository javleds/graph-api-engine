package sqlite

import (
	"bytes"
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	platformsqlite "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteCacheContract(t *testing.T) {
	ctx := context.Background()
	db, err := platformsqlite.Open(ctx, filepath.Join(t.TempDir(), "cache.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = platformsqlite.Migrate(ctx, db, "cache"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{1}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	cache := NewSQLiteResponseCache(db, protector, CacheLimits{MaxEntries: 2, MaxEntryBytes: 4096})
	clock := &fixedClock{now: time.Now().Truncate(time.Millisecond)}
	cache.Clock = clock
	entry := domain.CacheEntry{Key: "key", Response: domain.RawResponse{StatusCode: 200, Body: []byte(`{"secret":"private-payload"}`)}, FetchedAt: clock.now, ExpiresAt: clock.now.Add(time.Hour), SchemaVersion: 1}
	if _, found, err := cache.Get(ctx, "missing"); err != nil || found {
		t.Fatal("miss contract")
	}
	if err = cache.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}
	var blob []byte
	if err = db.QueryRow("SELECT response_blob FROM http_cache").Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("private-payload")) {
		t.Fatal("plaintext in cache")
	}
	older := entry
	older.FetchedAt = entry.FetchedAt.Add(-time.Second)
	older.Response.Body = []byte(`[]`)
	if err = cache.Put(ctx, older); err != nil {
		t.Fatal(err)
	}
	hit, found, err := cache.Get(ctx, "key")
	if err != nil || !found || !bytes.Equal(hit.Response.Body, entry.Response.Body) {
		t.Fatalf("%+v %v %v", hit, found, err)
	}
	hit.Response.Body[0] = '!'
	again, _, _ := cache.Get(ctx, "key")
	if !bytes.Equal(again.Response.Body, entry.Response.Body) {
		t.Fatal("mutable cache alias")
	}
	if !again.ExpiresAt.Equal(entry.ExpiresAt) {
		t.Fatal("read renewed expiration")
	}
	clock.now = entry.ExpiresAt
	if _, found, err = cache.Get(ctx, "key"); err != nil || found {
		t.Fatal("exact expiration not respected")
	}
	if count, err := cache.DeleteExpired(ctx, 10); err != nil || count != 1 {
		t.Fatalf("cleanup %d %v", count, err)
	}
	entry.Key = "clear"
	entry.ExpiresAt = clock.now.Add(time.Hour)
	if err = cache.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if count, err := cache.Clear(ctx); err != nil || count != 1 {
		t.Fatalf("clear %d %v", count, err)
	}
	if _, found, err := cache.Get(ctx, entry.Key); err != nil || found {
		t.Fatalf("cleared entry found=%t err=%v", found, err)
	}
}

func TestSQLiteCacheEvictsOldestAndRejectsFormats(t *testing.T) {
	ctx := context.Background()
	db, err := platformsqlite.Open(ctx, filepath.Join(t.TempDir(), "cache.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = platformsqlite.Migrate(ctx, db, "cache"); err != nil {
		t.Fatal(err)
	}
	protector, _ := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{2}, 32)})
	cache := NewSQLiteResponseCache(db, protector, CacheLimits{MaxEntries: 1, MaxEntryBytes: 1024})
	now := time.Now()
	for i, key := range []string{"old", "new"} {
		if err = cache.Put(ctx, domain.CacheEntry{Key: key, SchemaVersion: 1, Response: domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}, FetchedAt: now.Add(time.Duration(i) * time.Millisecond), ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, _ := cache.Get(ctx, "old"); found {
		t.Fatal("oldest not evicted")
	}
	if _, err = db.Exec("UPDATE http_cache SET schema_version=2"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Get(ctx, "new"); err == nil || found {
		t.Fatal("incompatible format reused")
	}
}
