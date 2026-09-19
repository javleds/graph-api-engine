package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/javleds/api-engine/migrations"
)

// Migrate checks the entire applied history before applying additive migrations.
// The exclusive instance lock must be acquired by the caller first.
func Migrate(ctx context.Context, db *sql.DB, kind string) error {
	if kind != "engine" && kind != "cache" {
		return fmt.Errorf("unknown database kind %q", kind)
	}
	names, err := fs.Glob(migrations.Files, kind+"/*.up.sql")
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&exists); err != nil {
		return err
	}
	applied := map[int]string{}
	if exists != 0 {
		rows, err := tx.QueryContext(ctx, "SELECT version,checksum FROM schema_migrations ORDER BY version")
		if err != nil {
			return err
		}
		for rows.Next() {
			var version int
			var checksum string
			if err = rows.Scan(&version, &checksum); err != nil {
				rows.Close()
				return err
			}
			applied[version] = checksum
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if len(applied) > len(names) {
		return fmt.Errorf("database migration history is newer than this binary")
	}
	for i, name := range names {
		version, err := strconv.Atoi(strings.Split(strings.TrimPrefix(name, kind+"/"), "_")[0])
		if err != nil || version != i+1 {
			return fmt.Errorf("invalid migration sequence: %s", name)
		}
		data, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])
		if actual, ok := applied[version]; ok {
			if actual != checksum {
				return fmt.Errorf("migration %d checksum mismatch", version)
			}
			continue
		}
		if version <= len(applied) {
			return fmt.Errorf("migration history has a gap at version %d", version)
		}
		if _, err = tx.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,?)", version, checksum, time.Now().UTC().UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
