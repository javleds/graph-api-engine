package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	executiondomain "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	securitydomain "github.com/javleds/api-engine/internal/security/domain"
	"sync"
	"time"
)

type SQLiteWebhookOutbox struct {
	db            *sql.DB
	protector     securityports.SecretProtector
	claimMu       sync.Mutex
	lastExecution string
	maxConcurrent int
}

func NewSQLiteWebhookOutbox(db *sql.DB, protector securityports.SecretProtector, maxConcurrent ...int) *SQLiteWebhookOutbox {
	o := &SQLiteWebhookOutbox{db: db, protector: protector}
	if len(maxConcurrent) > 0 {
		o.maxConcurrent = maxConcurrent[0]
	}
	return o
}

func (o *SQLiteWebhookOutbox) Claim(ctx context.Context, instance string, now time.Time, lease time.Duration) (*domain.WebhookDelivery, error) {
	o.claimMu.Lock()
	defer o.claimMu.Unlock()
	if lease <= 0 {
		return nil, errors.New("positive webhook lease required")
	}
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE webhook_deliveries SET last_error_json='{"code":"DELIVERY_DEADLINE_EXCEEDED","message":"Se agotó el plazo de entrega."}',status='failed',lease_owner=NULL,lease_expires_at=NULL,version=version+1 WHERE status IN ('pending','retrying') AND deadline_at<=?`, now.UnixMilli()); err != nil {
		return nil, err
	}
	d, err := loadDue(ctx, tx, now, o.lastExecution, o.maxConcurrent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	d.LeaseOwner = instance
	d.LeaseExpiresAt = now.Add(lease)
	result, err := tx.ExecContext(ctx, `UPDATE webhook_deliveries SET status='delivering',attempts=attempts+1,lease_owner=?,lease_expires_at=?,version=version+1 WHERE id=? AND version=? AND status IN ('pending','retrying')`, instance, d.LeaseExpiresAt.UnixMilli(), d.ID, d.Version)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, executiondomain.ErrVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	o.lastExecution = string(d.ExecutionID)
	d.Status = "delivering"
	d.Version++
	d.Attempts++
	d.Target, err = o.open(ctx, "webhook_target", d.OwnerID, d.ID, d.Target)
	if err != nil {
		return nil, err
	}
	d.Payload, err = o.open(ctx, "webhook_payload", d.OwnerID, d.ID, d.Payload)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (o *SQLiteWebhookOutbox) SaveAttempt(ctx context.Context, d domain.WebhookDelivery, success bool, retryAt *time.Time, httpStatus int, failure string, now time.Time) error {
	status := "failed"
	var delivered any
	next := now.UnixMilli()
	if success {
		status = "delivered"
		delivered = now.UnixMilli()
	} else if retryAt != nil && retryAt.Before(d.DeadlineAt) {
		status = "retrying"
		next = retryAt.UnixMilli()
	}
	var lastStatus any
	if httpStatus > 0 {
		lastStatus = httpStatus
	}
	var lastError any
	if failure != "" {
		encoded, _ := json.Marshal(executiondomain.ExecutionFailure{Code: failure, Message: "No se pudo entregar la notificación."})
		lastError = string(encoded)
	}
	result, err := o.db.ExecContext(ctx, `UPDATE webhook_deliveries SET status=?,next_attempt_at=?,last_http_status=?,last_error_json=?,delivered_at=?,lease_owner=NULL,lease_expires_at=NULL,version=version+1 WHERE id=? AND version=? AND lease_owner=? AND status='delivering' AND lease_expires_at>?`, status, next, lastStatus, lastError, delivered, d.ID, d.Version, d.LeaseOwner, now.UnixMilli())
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return executiondomain.ErrVersionConflict
	}
	return nil
}

func (o *SQLiteWebhookOutbox) Recover(ctx context.Context, instance string, now time.Time) error {
	_, err := o.db.ExecContext(ctx, `UPDATE webhook_deliveries SET last_error_json=CASE WHEN deadline_at<=? THEN '{"code":"DELIVERY_DEADLINE_EXCEEDED","message":"Se agotó el plazo de entrega."}' ELSE last_error_json END,status=CASE WHEN deadline_at<=? THEN 'failed' ELSE 'retrying' END,lease_owner=NULL,lease_expires_at=NULL,next_attempt_at=?,version=version+1 WHERE status='delivering' AND (lease_owner<>? OR lease_owner IS NULL OR lease_expires_at<=?)`, now.UnixMilli(), now.UnixMilli(), now.UnixMilli(), instance, now.UnixMilli())
	return err
}

func (o *SQLiteWebhookOutbox) Status(ctx context.Context, owner string, id executiondomain.ExecutionID) (*domain.WebhookStatus, error) {
	var status domain.WebhookStatus
	err := o.db.QueryRowContext(ctx, `SELECT status,attempts FROM webhook_deliveries WHERE owner_id=? AND execution_id=? AND event_type<>'node.finished' ORDER BY created_at DESC LIMIT 1`, owner, id).Scan(&status.Status, &status.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (o *SQLiteWebhookOutbox) open(ctx context.Context, table, owner, id string, data []byte) ([]byte, error) {
	var blob securitydomain.ProtectedBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, err
	}
	purpose, _ := json.Marshal([]string{table, owner, id})
	return o.protector.Open(ctx, string(purpose), blob)
}

func loadDue(ctx context.Context, tx *sql.Tx, now time.Time, afterExecution string, maxConcurrent int) (domain.WebhookDelivery, error) {
	var d domain.WebhookDelivery
	var next, deadline int64
	err := tx.QueryRowContext(ctx, dueDeliverySQL, now.UnixMilli(), now.UnixMilli(), maxConcurrent, maxConcurrent, afterExecution).Scan(&d.ID, &d.EventID, &d.ExecutionID, &d.OwnerID, &d.EventType, &d.Target, &d.Payload, &d.Status, &d.Attempts, &d.Version, &next, &deadline)
	d.NextAttemptAt = time.UnixMilli(next).UTC()
	d.DeadlineAt = time.UnixMilli(deadline).UTC()
	return d, err
}
