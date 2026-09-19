package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	securitydomain "github.com/javleds/api-engine/internal/security/domain"
)

type ExecutionStore struct {
	BeforeV2Write        func(context.Context) error
	db                   *sql.DB
	protector            securityports.SecretProtector
	maxPendingDeliveries int
}

func NewExecutionStore(db *sql.DB, protector securityports.SecretProtector, maximum ...int) *ExecutionStore {
	limit := 1000
	if len(maximum) > 0 {
		limit = maximum[0]
	}
	return &ExecutionStore{db: db, protector: protector, maxPendingDeliveries: limit}
}

func (s *ExecutionStore) Create(ctx context.Context, snapshot d.ExecutionSnapshot) (d.Execution, bool, error) {
	if snapshot.APIVersion == 0 {
		snapshot.APIVersion = 1
	}
	if snapshot.APIVersion != 1 && snapshot.APIVersion != 2 {
		return d.Execution{}, false, errors.New("unsupported execution protocol")
	}
	blob, err := s.seal(ctx, "executions", snapshot.OwnerID, string(snapshot.ID), snapshot.Definition)
	if err != nil {
		return d.Execution{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return d.Execution{}, false, err
	}
	defer tx.Rollback()
	if snapshot.IdempotencyKeyHash != "" {
		existing, err := scanExecution(tx.QueryRowContext(ctx, executionSelect+" WHERE owner_id=? AND idempotency_key_hash=?", snapshot.OwnerID, snapshot.IdempotencyKeyHash))
		if err == nil {
			if existing.Fingerprint != snapshot.Fingerprint {
				return d.Execution{}, false, d.ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if !errors.Is(err, d.ErrNotFound) {
			return d.Execution{}, false, err
		}
	}
	if snapshot.WebhookEnabled {
		var pending int
		err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM executions WHERE webhook_enabled=1 AND status IN ('pending','running'))+(SELECT count(*) FROM webhook_deliveries WHERE status IN ('pending','retrying','delivering'))`).Scan(&pending)
		if err != nil {
			return d.Execution{}, false, err
		}
		if pending >= s.maxPendingDeliveries {
			return d.Execution{}, false, d.ErrEngineBusy
		}
	}
	var key any
	if snapshot.IdempotencyKeyHash != "" {
		key = snapshot.IdempotencyKeyHash
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO executions(id,owner_id,status,definition_version,webhook_enabled,request_fingerprint,idempotency_key_hash,snapshot_blob,created_at,metadata_expires_at,api_version) VALUES(?,?,'pending',1,?,?,?,?,?,?,?)`, snapshot.ID, snapshot.OwnerID, snapshot.WebhookEnabled, snapshot.Fingerprint, key, blob, snapshot.CreatedAt.UnixMilli(), snapshot.MetadataExpiresAt.UnixMilli(), snapshot.APIVersion)
	if err != nil {
		return d.Execution{}, false, err
	}
	if snapshot.APIVersion == 2 {
		if err := s.initializeV2(ctx, tx, snapshot.ID, snapshot.DeadlineAt); err != nil {
			return d.Execution{}, false, err
		}
	}
	execution, err := scanExecution(tx.QueryRowContext(ctx, executionSelect+" WHERE id=?", snapshot.ID))
	if err != nil {
		return execution, false, err
	}
	return execution, false, tx.Commit()
}

func (s *ExecutionStore) FindReplay(ctx context.Context, owner, key string) (d.Execution, error) {
	row := s.db.QueryRowContext(ctx, executionSelect+" WHERE owner_id=? AND idempotency_key_hash=?", owner, key)
	return scanExecution(row)
}

func (s *ExecutionStore) Read(ctx context.Context, owner string, id d.ExecutionID) (d.Execution, error) {
	return scanExecution(s.db.QueryRowContext(ctx, executionSelect+" WHERE owner_id=? AND id=?", owner, id))
}

func (s *ExecutionStore) Load(ctx context.Context, id d.ExecutionID) (d.ExecutionSnapshot, d.Execution, error) {
	execution, err := scanExecution(s.db.QueryRowContext(ctx, executionSelect+" WHERE id=?", id))
	if err != nil {
		return d.ExecutionSnapshot{}, execution, err
	}
	var blob []byte
	var fingerprint string
	var key sql.NullString
	var expires int64
	err = s.db.QueryRowContext(ctx, "SELECT snapshot_blob,request_fingerprint,idempotency_key_hash,metadata_expires_at FROM executions WHERE id=?", id).Scan(&blob, &fingerprint, &key, &expires)
	if err != nil {
		return d.ExecutionSnapshot{}, execution, err
	}
	plaintext, err := s.open(ctx, "executions", execution.OwnerID, string(id), blob)
	if err != nil {
		return d.ExecutionSnapshot{}, execution, err
	}
	return d.ExecutionSnapshot{APIVersion: execution.APIVersion, ID: id, OwnerID: execution.OwnerID, Fingerprint: fingerprint, IdempotencyKeyHash: key.String, Definition: plaintext, WebhookEnabled: execution.WebhookEnabled, CreatedAt: execution.CreatedAt, MetadataExpiresAt: time.UnixMilli(expires).UTC()}, execution, nil
}

func (s *ExecutionStore) Claim(ctx context.Context, id d.ExecutionID, instance string, version int64, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, "UPDATE executions SET status='running',worker_instance_id=?,started_at=?,version=version+1 WHERE id=? AND status='pending' AND version=?", instance, now.UnixMilli(), id, version)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *ExecutionStore) ReadResult(ctx context.Context, owner string, id d.ExecutionID, now time.Time) (d.Execution, []byte, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return d.Execution{}, nil, err
	}
	defer tx.Rollback()
	execution, err := scanExecution(tx.QueryRowContext(ctx, executionSelect+" WHERE owner_id=? AND id=?", owner, id))
	if err != nil {
		return execution, nil, err
	}
	if execution.APIVersion == 2 {
		return execution, nil, d.ErrNotFound
	}
	if execution.Status == d.Failed {
		return execution, nil, d.ErrExecutionFailed
	}
	if !execution.Status.Terminal() {
		return execution, nil, d.ErrNotReady
	}
	if execution.ResultExpiresAt == nil || !now.Before(*execution.ResultExpiresAt) {
		return execution, nil, d.ErrResultExpired
	}
	var blob []byte
	if err := tx.QueryRowContext(ctx, "SELECT result_blob FROM execution_results WHERE execution_id=?", id).Scan(&blob); err != nil {
		return execution, nil, err
	}
	if err := tx.Commit(); err != nil {
		return execution, nil, err
	}
	data, err := s.open(ctx, "execution_results", owner, string(id), blob)
	return execution, data, err
}

func (s *ExecutionStore) seal(ctx context.Context, table, owner, id string, data []byte) ([]byte, error) {
	purpose, _ := json.Marshal([]string{table, owner, id})
	blob, err := s.protector.Seal(ctx, string(purpose), data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(blob)
}

func (s *ExecutionStore) open(ctx context.Context, table, owner, id string, data []byte) ([]byte, error) {
	var blob securitydomain.ProtectedBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("decode protected storage: %w", err)
	}
	purpose, _ := json.Marshal([]string{table, owner, id})
	return s.protector.Open(ctx, string(purpose), blob)
}

const executionSelect = `SELECT id,owner_id,request_fingerprint,status,version,created_at,started_at,finished_at,result_expires_at,progress_json,error_json,error_report_json,webhook_enabled,api_version FROM executions`

func scanExecution(row *sql.Row) (d.Execution, error) {
	var execution d.Execution
	var created int64
	var started, finished, expires sql.NullInt64
	var progress, report string
	var failure sql.NullString
	err := row.Scan(&execution.ID, &execution.OwnerID, &execution.Fingerprint, &execution.Status, &execution.Version, &created, &started, &finished, &expires, &progress, &failure, &report, &execution.WebhookEnabled, &execution.APIVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return execution, d.ErrNotFound
	}
	if err != nil {
		return execution, err
	}
	execution.CreatedAt = time.UnixMilli(created).UTC()
	execution.StartedAt = optionalTime(started)
	execution.FinishedAt = optionalTime(finished)
	execution.ResultExpiresAt = optionalTime(expires)
	if err := json.Unmarshal([]byte(progress), &execution.Progress); err != nil {
		return execution, err
	}
	if err := json.Unmarshal([]byte(report), &execution.Report); err != nil {
		return execution, err
	}
	if failure.Valid {
		if err := json.Unmarshal([]byte(failure.String), &execution.Error); err != nil {
			return execution, err
		}
	}
	return execution, nil
}

func optionalTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	t := time.UnixMilli(value.Int64).UTC()
	return &t
}

// UpdateProgress does not change the claim version used for terminal CAS.
func (s *ExecutionStore) UpdateProgress(ctx context.Context, id d.ExecutionID, version int64, progress d.ExecutionProgress) error {
	encoded, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "UPDATE executions SET progress_json=? WHERE id=? AND status='running' AND version=?", string(encoded), id, version)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return d.ErrVersionConflict
	}
	return nil
}

// Purge bounds each deletion and keeps metadata with retryable deliveries.
func (s *ExecutionStore) Purge(ctx context.Context, now time.Time, limit int) error {
	if limit <= 0 {
		return errors.New("positive retention batch size required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.purgeV2(ctx, tx, now, limit); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM execution_results WHERE execution_id IN (SELECT execution_id FROM execution_results WHERE expires_at<=? ORDER BY expires_at LIMIT ?)`, now.UnixMilli(), limit); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM executions WHERE id IN (SELECT id FROM executions WHERE metadata_expires_at<=? AND status IN ('completed','completed_with_errors','failed') AND NOT EXISTS (SELECT 1 FROM webhook_deliveries w WHERE w.execution_id=executions.id AND w.status IN ('pending','retrying','delivering')) ORDER BY metadata_expires_at LIMIT ?)`, now.UnixMilli(), limit); err != nil {
		return err
	}
	return tx.Commit()
}

// RecordProgress is used by the joined v1 worker; executions are never reassigned.
func (s *ExecutionStore) RecordProgress(ctx context.Context, id d.ExecutionID, progress d.ExecutionProgress) error {
	encoded, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "UPDATE executions SET progress_json=? WHERE id=? AND status='running'", string(encoded), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return d.ErrVersionConflict
	}
	return nil
}
