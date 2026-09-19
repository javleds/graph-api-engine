package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

func (s *ExecutionStore) checkV2Read(ctx context.Context, owner string, id d.ExecutionID, now time.Time) (d.V2State, error) {
	e, err := s.Read(ctx, owner, id)
	if err != nil {
		return d.V2State{}, err
	}
	state, err := s.ReadV2State(ctx, id)
	if err != nil {
		return state, err
	}
	if e.Status.Terminal() && (e.ResultExpiresAt == nil || !now.Before(*e.ResultExpiresAt)) {
		return state, d.ErrResultExpired
	}
	return state, nil
}
func (s *ExecutionStore) ReadV2Chunks(ctx context.Context, owner string, id d.ExecutionID, after int64, limit int, maxBytes int64, now time.Time) (d.V2ChunkPage, error) {
	page := d.V2ChunkPage{Chunks: []json.RawMessage{}, NextCursor: strconv.FormatInt(after, 10)}
	state, err := s.checkV2Read(ctx, owner, id, now)
	if err != nil {
		return page, err
	}
	last, _ := strconv.ParseInt(state.LastSequence, 10, 64)
	if after < 0 || after > last {
		return page, d.ErrV2Cursor
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if maxBytes <= 0 || maxBytes > 1<<20 {
		maxBytes = 1 << 20
	}
	page.Terminal = state.Terminal
	page.LastSequence = state.LastSequence
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,payload_blob FROM v2_chunks WHERE execution_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`, id, after, last, limit)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var used int64 = 256
	for rows.Next() {
		var seq int64
		var blob []byte
		if err = rows.Scan(&seq, &blob); err != nil {
			return page, err
		}
		raw, err := s.open(ctx, "v2_chunks", owner, string(id)+":"+strconv.FormatInt(seq, 10), blob)
		if err != nil {
			return page, err
		}
		if used+int64(len(raw))+1 > maxBytes && len(page.Chunks) > 0 {
			break
		}
		used += int64(len(raw)) + 1
		page.Chunks = append(page.Chunks, json.RawMessage(raw))
		page.NextCursor = strconv.FormatInt(seq, 10)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	cursor, _ := strconv.ParseInt(page.NextCursor, 10, 64)
	page.HasMore = cursor < last
	return page, nil
}
func (s *ExecutionStore) ReadV2Tree(ctx context.Context, owner string, id d.ExecutionID, parent string, after int64, limit int, now time.Time) (d.V2TreePage, error) {
	page := d.V2TreePage{Nodes: []d.V2Node{}, NextCursor: strconv.FormatInt(after, 10)}
	state, err := s.checkV2Read(ctx, owner, id, now)
	if err != nil {
		return page, err
	}
	last, _ := strconv.ParseInt(state.LastSequence, 10, 64)
	if after < 0 || after > last {
		return page, d.ErrV2Cursor
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	page.Terminal = state.Terminal
	page.LastSequence = state.LastSequence
	// Node cursors are their immutable declaration sequence. Ordered publication by the
	// coordinator ensures pagination remains stable while descendants are added.
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,payload_blob FROM v2_nodes WHERE execution_id=? AND parent_id=? AND position>? ORDER BY position,node_id LIMIT ?`, id, parent, after, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var used int
	for rows.Next() {
		var key string
		var blob []byte
		if err = rows.Scan(&key, &blob); err != nil {
			return page, err
		}
		raw, err := s.open(ctx, "v2_nodes", owner, string(id)+":"+key, blob)
		if err != nil {
			return page, err
		}
		var node d.V2Node
		if err = json.Unmarshal(raw, &node); err != nil {
			return page, err
		}
		if len(page.Nodes) >= limit || used+len(raw) > 1<<20 {
			page.HasMore = true
			break
		}
		used += len(raw)
		page.Nodes = append(page.Nodes, node)
		position, _ := strconv.ParseInt(node.Cursor, 10, 64)
		if node.Index != nil {
			position = *node.Index + 1
		}
		page.NextCursor = strconv.FormatInt(position, 10)
	}
	return page, rows.Err()
}
func (s *ExecutionStore) RecoverV2(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE v2_state SET generation=generation+1,recoveries=recoveries+1 WHERE execution_id IN (SELECT id FROM executions WHERE api_version=2 AND status IN ('pending','running'))`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE executions SET status='pending',worker_instance_id=NULL,version=version+1 WHERE api_version=2 AND status IN ('pending','running')`)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *ExecutionStore) PendingV2IDs(ctx context.Context, limit int) ([]d.ExecutionID, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM executions WHERE api_version=2 AND status='pending' ORDER BY created_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []d.ExecutionID{}
	for rows.Next() {
		var id d.ExecutionID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *ExecutionStore) PurgeV2(ctx context.Context, now time.Time, limit int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.purgeV2(ctx, tx, now, limit); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *ExecutionStore) purgeV2(ctx context.Context, tx *sql.Tx, now time.Time, limit int) error {
	for _, table := range []string{"v2_chunks", "v2_nodes", "v2_inputs", "v2_tasks"} {
		_, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE rowid IN (SELECT x.rowid FROM `+table+` x JOIN executions e ON e.id=x.execution_id WHERE e.result_expires_at<=? AND e.status IN ('completed','completed_with_errors','failed') LIMIT ?)`, now.UnixMilli(), limit)
		if err != nil {
			return err
		}
	}
	return nil
}
