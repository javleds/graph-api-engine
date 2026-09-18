package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/javleds/api-engine/migrations"
)

func Migrate(ctx context.Context, db *sql.DB, kind string) error {
	if kind != "engine" && kind != "cache" {
		return fmt.Errorf("unknown database kind %q", kind)
	}
	data, err := migrations.Files.ReadFile(kind + "/000001_initial.up.sql")
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		var count int
		var actual string
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("unexpected migration history: %d versions", count)
		}
		if err := tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=1").Scan(&actual); err != nil {
			return err
		}
		if actual != checksum {
			return fmt.Errorf("migration 1 checksum mismatch")
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("apply initial %s migration: %w", kind, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(1,?,?)", checksum, time.Now().UTC().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}
