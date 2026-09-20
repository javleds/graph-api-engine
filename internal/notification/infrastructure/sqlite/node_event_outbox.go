package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
)

func (o *SQLiteWebhookOutbox) PendingNodeExecutions(ctx context.Context, after string, limit int) ([]domain.NodeEventCursor, error) {
	rows, err := o.db.QueryContext(ctx, `SELECT s.execution_id,e.owner_id,s.webhook_cursor FROM v2_state s JOIN executions e ON e.id=s.execution_id WHERE s.results_enabled=1 AND e.api_version=2 AND e.webhook_enabled=1 AND s.webhook_cursor<s.last_result_sequence AND s.execution_id>? ORDER BY s.execution_id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cursors []domain.NodeEventCursor
	for rows.Next() {
		var c domain.NodeEventCursor
		if err := rows.Scan(&c.ExecutionID, &c.OwnerID, &c.After); err != nil {
			return nil, err
		}
		cursors = append(cursors, c)
	}
	return cursors, rows.Err()
}

func (o *SQLiteWebhookOutbox) SaveNodeDelivery(ctx context.Context, cursor domain.NodeEventCursor, d domain.PreparedNodeDelivery, maxPending int, maxStored int64) (bool, error) {
	if d.Sequence != cursor.After+1 || d.ExecutionID != cursor.ExecutionID || d.OwnerID != cursor.OwnerID {
		return false, errors.New("invalid node delivery cursor")
	}
	target, err := o.seal(ctx, "webhook_target", d.OwnerID, d.ID, d.Target)
	if err != nil {
		return false, err
	}
	payload, err := o.seal(ctx, "webhook_payload", d.OwnerID, d.ID, d.Payload)
	if err != nil {
		return false, err
	}
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT webhook_cursor FROM v2_state WHERE execution_id=?`, cursor.ExecutionID).Scan(&current); err != nil {
		return false, err
	}
	if current >= d.Sequence {
		return true, nil
	}
	if current != cursor.After {
		return false, execution.ErrVersionConflict
	}
	status := "pending"
	var failure any
	// Reserve bounded diagnostic space before admission, including later attempt failures.
	size := len(d.Payload) + len(d.Target) + 512
	if d.Failure != "" {
		status = "failed"
		raw, _ := json.Marshal(execution.ExecutionFailure{Code: d.Failure, Message: "No se pudo entregar la notificación."})
		failure = string(raw)
	} else {
		var pending int
		if err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM executions WHERE webhook_enabled=1 AND status IN ('pending','running'))+(SELECT count(*) FROM webhook_deliveries WHERE status IN ('pending','retrying','delivering'))`).Scan(&pending); err != nil {
			return false, err
		}
		if maxPending > 0 && pending >= maxPending {
			return false, nil
		}
	}
	var stored int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(stored_bytes+CASE WHEN terminal=0 THEN 262144 ELSE 0 END),0) FROM v2_state`).Scan(&stored); err != nil {
		return false, err
	}
	if maxStored > 0 && stored+int64(size) > maxStored {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO webhook_deliveries(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,next_attempt_at,created_at,deadline_at,last_error_json,plaintext_size) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID, d.EventID, d.ExecutionID, d.OwnerID, "node.finished", target, payload, status, d.NextAttemptAt.UnixMilli(), d.OccurredAt.UnixMilli(), d.DeadlineAt.UnixMilli(), failure, size)
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE v2_state SET webhook_cursor=? WHERE execution_id=? AND webhook_cursor=?`, d.Sequence, d.ExecutionID, cursor.After); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (o *SQLiteWebhookOutbox) seal(ctx context.Context, table, owner, id string, data []byte) ([]byte, error) {
	purpose, _ := json.Marshal([]string{table, owner, id})
	blob, err := o.protector.Seal(ctx, string(purpose), data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(blob)
}

func (o *SQLiteWebhookOutbox) NodeSummary(ctx context.Context, owner string, id execution.ExecutionID) (domain.NodeWebhookSummary, error) {
	var summary domain.NodeWebhookSummary
	err := o.db.QueryRowContext(ctx, `SELECT
 CASE WHEN e.webhook_enabled=1 THEN s.last_result_sequence-s.webhook_cursor ELSE 0 END +
 (SELECT count(*) FROM webhook_deliveries d WHERE d.execution_id=e.id AND d.event_type='node.finished' AND d.status IN ('pending','retrying','delivering')),
 (SELECT count(*) FROM webhook_deliveries d WHERE d.execution_id=e.id AND d.event_type='node.finished' AND d.status='delivered'),
 (SELECT count(*) FROM webhook_deliveries d WHERE d.execution_id=e.id AND d.event_type='node.finished' AND d.status='failed')
 FROM executions e JOIN v2_state s ON s.execution_id=e.id WHERE e.id=? AND e.owner_id=?`, id, owner).Scan(&summary.Pending, &summary.Delivered, &summary.Failed)
	if errors.Is(err, sql.ErrNoRows) {
		return summary, nil
	}
	return summary, err
}

func (o *SQLiteWebhookOutbox) DeliveryPage(ctx context.Context, owner string, id execution.ExecutionID, after string, limit int) (domain.WebhookDeliveryPage, error) {
	page := domain.WebhookDeliveryPage{Deliveries: []domain.WebhookDeliveryStatus{}, NextCursor: after}
	cursor, err := strconv.ParseInt(after, 10, 64)
	if err != nil || cursor < 0 || strconv.FormatInt(cursor, 10) != after {
		return page, errors.New("invalid delivery cursor")
	}
	if limit < 1 || limit > 100 {
		return page, errors.New("invalid delivery limit")
	}
	rows, err := o.db.QueryContext(ctx, `SELECT d.id,d.event_id,r.node_id,r.sequence,d.status,d.attempts,d.last_error_json FROM v2_results r JOIN webhook_deliveries d ON d.event_id=r.event_id WHERE d.execution_id=? AND d.owner_id=? AND d.event_type='node.finished' AND r.sequence>? ORDER BY r.sequence LIMIT ?`, id, owner, cursor, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(page.Deliveries) == limit {
			page.HasMore = true
			break
		}
		var item domain.WebhookDeliveryStatus
		var sequence int64
		var failure sql.NullString
		if err := rows.Scan(&item.ID, &item.EventID, &item.NodeID, &sequence, &item.Status, &item.Attempts, &failure); err != nil {
			return page, err
		}
		if failure.Valid {
			if err := json.Unmarshal([]byte(failure.String), &item.Error); err != nil {
				return page, err
			}
		}
		item.Sequence = strconv.FormatInt(sequence, 10)
		page.NextCursor = item.Sequence
		page.Deliveries = append(page.Deliveries, item)
	}
	return page, rows.Err()
}
