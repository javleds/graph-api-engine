package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	securitydomain "github.com/javleds/api-engine/internal/security/domain"
	"time"
)

type SQLiteResponseCache struct {
	DB        *sql.DB
	Protector securityports.SecretProtector
	Clock     clock.Clock
	Limits    CacheLimits
}

func NewSQLiteResponseCache(db *sql.DB, protector securityports.SecretProtector, limits CacheLimits) *SQLiteResponseCache {
	if limits.MaxEntryBytes <= 0 {
		limits.MaxEntryBytes = 1 << 20
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = 100000
	}
	if limits.MaxLogicalBytes <= 0 {
		limits.MaxLogicalBytes = 512 << 20
	}
	if limits.MaxTTL <= 0 {
		limits.MaxTTL = 30 * 24 * time.Hour
	}
	return &SQLiteResponseCache{DB: db, Protector: protector, Clock: clock.RealClock{}, Limits: limits}
}

func (s *SQLiteResponseCache) Get(ctx context.Context, key string) (domain.CacheEntry, bool, error) {
	var encoded []byte
	var fetched, expires, age int64
	var version int
	err := s.DB.QueryRowContext(ctx, `SELECT schema_version,response_blob,fetched_at,expires_at,initial_age_ms FROM http_cache WHERE cache_key=? AND expires_at>?`, key, s.Clock.Now().UnixMilli()).Scan(&version, &encoded, &fetched, &expires, &age)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CacheEntry{}, false, nil
	}
	if err != nil {
		return domain.CacheEntry{}, false, err
	}
	if version != 1 {
		return domain.CacheEntry{}, false, errors.New("unsupported cache format")
	}
	var blob securitydomain.ProtectedBlob
	if err = json.Unmarshal(encoded, &blob); err != nil {
		return domain.CacheEntry{}, false, errors.New("invalid cache envelope")
	}
	plaintext, err := s.Protector.Open(ctx, "http_cache:"+key, blob)
	if err != nil {
		return domain.CacheEntry{}, false, err
	}
	if int64(len(plaintext)) > s.Limits.MaxEntryBytes {
		return domain.CacheEntry{}, false, errors.New("cache entry exceeds limit")
	}
	var response domain.RawResponse
	if err = json.Unmarshal(plaintext, &response); err != nil {
		return domain.CacheEntry{}, false, errors.New("invalid cached response")
	}
	if response.StatusCode != 200 || !json.Valid(response.Body) {
		return domain.CacheEntry{}, false, errors.New("invalid cached response")
	}
	entry := domain.CacheEntry{Key: key, Response: response, FetchedAt: time.UnixMilli(fetched), ExpiresAt: time.UnixMilli(expires), InitialAge: time.Duration(age) * time.Millisecond, SchemaVersion: version}
	return entry, true, nil
}

func (s *SQLiteResponseCache) Put(ctx context.Context, entry domain.CacheEntry) error {
	if entry.SchemaVersion != 1 || entry.Key == "" || entry.Response.StatusCode != 200 || !json.Valid(entry.Response.Body) {
		return errors.New("invalid cache entry")
	}
	now := s.Clock.Now()
	if !entry.ExpiresAt.After(now) {
		return nil
	}
	if maximum := now.Add(s.Limits.MaxTTL); entry.ExpiresAt.After(maximum) {
		entry.ExpiresAt = maximum
	}
	encoded, err := json.Marshal(entry.Response)
	if err != nil {
		return err
	}
	if int64(len(encoded)) > s.Limits.MaxEntryBytes {
		return errors.New("cache entry exceeds limit")
	}
	blob, err := s.Protector.Seal(ctx, "http_cache:"+entry.Key, encoded)
	if err != nil {
		return err
	}
	protected, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO http_cache(cache_key,schema_version,response_blob,plaintext_size,fetched_at,expires_at,initial_age_ms) VALUES(?,?,?,?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET schema_version=excluded.schema_version,response_blob=excluded.response_blob,plaintext_size=excluded.plaintext_size,fetched_at=excluded.fetched_at,expires_at=excluded.expires_at,initial_age_ms=excluded.initial_age_ms WHERE excluded.fetched_at>http_cache.fetched_at`, entry.Key, entry.SchemaVersion, protected, len(encoded), entry.FetchedAt.UnixMilli(), entry.ExpiresAt.UnixMilli(), entry.InitialAge.Milliseconds())
	if err != nil {
		return err
	}
	if err = s.enforceLimits(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteResponseCache) Delete(ctx context.Context, key string) error {
	_, err := s.DB.ExecContext(ctx, "DELETE FROM http_cache WHERE cache_key=?", key)
	return err
}

// Clear removes every stored upstream response. It does not affect in-flight
// requests, execution records, quota state, or HTTP transport connections.
func (s *SQLiteResponseCache) Clear(ctx context.Context) (int64, error) {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM http_cache")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SQLiteResponseCache) enforceLimits(ctx context.Context, tx *sql.Tx) error {
	var entries, bytes int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(plaintext_size),0) FROM http_cache").Scan(&entries, &bytes); err != nil {
		return err
	}
	for entries > s.Limits.MaxEntries || bytes > s.Limits.MaxLogicalBytes {
		var key string
		var size int64
		if err := tx.QueryRowContext(ctx, "SELECT cache_key,plaintext_size FROM http_cache ORDER BY fetched_at,cache_key LIMIT 1").Scan(&key, &size); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM http_cache WHERE cache_key=?", key); err != nil {
			return err
		}
		entries--
		bytes -= size
	}
	return nil
}

func (s *SQLiteResponseCache) DeleteExpired(ctx context.Context, limit int) (int64, error) {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM http_cache WHERE cache_key IN (SELECT cache_key FROM http_cache WHERE expires_at<=? LIMIT ?)`, s.Clock.Now().UnixMilli(), limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
