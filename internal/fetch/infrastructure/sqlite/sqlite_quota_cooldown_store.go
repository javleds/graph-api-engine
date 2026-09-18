package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SQLiteQuotaCooldownStore struct{ DB *sql.DB }

func (s *SQLiteQuotaCooldownStore) Get(ctx context.Context, scope string) (time.Time, error) {
	var value int64
	err := s.DB.QueryRowContext(ctx, "SELECT not_before FROM quota_cooldowns WHERE scope_key = ?", scope).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(value), nil
}

func (s *SQLiteQuotaCooldownStore) Extend(ctx context.Context, scope string, until time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO quota_cooldowns(scope_key, not_before, updated_at) VALUES(?,?,?) ON CONFLICT(scope_key) DO UPDATE SET not_before = MAX(not_before, excluded.not_before), updated_at = excluded.updated_at`, scope, until.UnixMilli(), time.Now().UnixMilli())
	return err
}

func (s *SQLiteQuotaCooldownStore) DeleteExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM quota_cooldowns WHERE scope_key IN (SELECT scope_key FROM quota_cooldowns WHERE not_before <= ? LIMIT ?)`, now.UnixMilli(), limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
