package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDatabaseDurabilityMigrationsAndReconnection(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"engine", "cache"} {
		t.Run(kind, func(t *testing.T) {
			db, err := Open(ctx, filepath.Join(t.TempDir(), kind+".db"), kind == "cache")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := Migrate(ctx, db, kind); err != nil {
				t.Fatal(err)
			}
			if err := Migrate(ctx, db, kind); err != nil {
				t.Fatal(err)
			}
			db.SetMaxIdleConns(0)
			for name, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000} {
				var got int
				if err := db.QueryRow("PRAGMA " + name).Scan(&got); err != nil || got != want {
					t.Fatalf("%s=%d error=%v", name, got, err)
				}
			}
			var sync int
			if err := db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
				t.Fatal(err)
			}
			want := 2
			if kind == "cache" {
				want = 1
			}
			if sync != want {
				t.Fatalf("sync=%d", sync)
			}
			if _, err := db.Exec("UPDATE schema_migrations SET checksum='tampered'"); err != nil {
				t.Fatal(err)
			}
			if err := Migrate(ctx, db, kind); err == nil {
				t.Fatal("modified migration accepted")
			}
		})
	}
}

func TestExclusiveInstanceAndVersion(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := AcquireInstanceLock(dir); err == nil {
		second.Close()
		t.Fatal("second instance accepted")
	}
	first.Close()
	next, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	for version, want := range map[string]bool{"3.51.2": false, "3.51.3": true, "3.52.0": true, "3.50.99": false, "invalid": false} {
		if SafeVersion(version) != want {
			t.Fatalf("version %s", version)
		}
	}
}
