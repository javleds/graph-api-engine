package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

type ExecutionFinalizer struct{ store *ExecutionStore }

func NewExecutionFinalizer(store *ExecutionStore) *ExecutionFinalizer {
	return &ExecutionFinalizer{store: store}
}

func (f *ExecutionFinalizer) Finish(ctx context.Context, c d.ExecutionCompletion) error {
	if err := d.ValidateCompletion(c); err != nil {
		return err
	}
	snapshot, execution, err := f.store.Load(ctx, c.ID)
	if err != nil {
		return err
	}
	if execution.WebhookEnabled && c.Delivery == nil {
		return errors.New("webhook-enabled completion requires outbox event")
	}
	prepared, err := prepareCompletion(ctx, f.store, snapshot.OwnerID, c)
	if err != nil {
		return err
	}
	tx, err := f.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if c.APIVersion == 2 && c.ExpectedCheckpointRevision != nil {
		var revision int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT revision FROM v2_tasks WHERE execution_id=? AND task_key='coordinator'),-1)`, c.ID).Scan(&revision); err != nil {
			return err
		}
		if revision != *c.ExpectedCheckpointRevision {
			return d.ErrVersionConflict
		}
		if err := tx.QueryRowContext(ctx, `SELECT logical_requests,http_attempts FROM v2_state WHERE execution_id=?`, c.ID).Scan(&c.Progress.LogicalRequests, &c.Progress.HTTPAttempts); err != nil {
			return err
		}
		progress, err := json.Marshal(c.Progress)
		if err != nil {
			return err
		}
		prepared.progress = string(progress)
	}
	result, err := tx.ExecContext(ctx, `UPDATE executions SET status=?,error_json=?,error_report_json=?,progress_json=?,finished_at=?,result_expires_at=?,version=version+1 WHERE id=? AND version=? AND (status='running' OR (status='pending' AND ?='failed'))`, c.Status, prepared.failure, prepared.report, prepared.progress, c.FinishedAt.UnixMilli(), prepared.expires, c.ID, c.ExpectedVersion, c.Status)
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
	if c.APIVersion == 2 {
		if err := f.store.appendV2Terminal(ctx, tx, snapshot.OwnerID, c); err != nil {
			return err
		}
	}
	if c.Status != d.Failed && c.APIVersion != 2 {
		hash := sha256.Sum256(c.Data)
		_, err = tx.ExecContext(ctx, `INSERT INTO execution_results(execution_id,result_blob,plaintext_size,sha256,created_at,expires_at) VALUES(?,?,?,?,?,?)`, c.ID, prepared.result, len(c.Data), hex.EncodeToString(hash[:]), c.FinishedAt.UnixMilli(), c.ResultExpiresAt.UnixMilli())
		if err != nil {
			return err
		}
	}
	if c.Delivery != nil {
		if err := insertDelivery(ctx, tx, snapshot.OwnerID, c, prepared); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertDelivery(ctx context.Context, tx *sql.Tx, owner string, c d.ExecutionCompletion, p preparedCompletion) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO webhook_deliveries(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,next_attempt_at,created_at,deadline_at) VALUES(?,?,?,?,?,?,?,'pending',?,?,?)`, c.Delivery.ID, c.Delivery.EventID, c.ID, owner, "execution."+string(c.Status), p.target, p.payload, c.FinishedAt.UnixMilli(), c.FinishedAt.UnixMilli(), c.Delivery.DeadlineAt.UnixMilli())
	return err
}
