package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string, cache bool) (*sql.DB, error) {
	return OpenConfigured(ctx, path, cache, 5*time.Second)
}

func OpenConfigured(ctx context.Context, path string, cache bool, busyTimeout time.Duration) (*sql.DB, error) {
	if busyTimeout < time.Millisecond {
		return nil, fmt.Errorf("SQLite busy timeout must be at least one millisecond")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	synchronous := "FULL"
	if cache {
		synchronous = "NORMAL"
	}
	params := url.Values{}
	for _, pragma := range []string{"journal_mode(WAL)", "foreign_keys(ON)", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()), "synchronous(" + synchronous + ")"} {
		params.Add("_pragma", pragma)
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: params.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	var version string
	if err = db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if !SafeVersion(version) {
		db.Close()
		return nil, fmt.Errorf("unsafe SQLite version %s: require 3.51.3 or later", version)
	}
	return db, nil
}

func SafeVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	minimum := []int{3, 51, 3}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return false
		}
		if n > minimum[i] {
			return true
		}
		if n < minimum[i] {
			return false
		}
	}
	return true
}
