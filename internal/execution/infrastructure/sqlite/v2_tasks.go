package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

func (s *ExecutionStore) initializeV2(ctx context.Context, tx *sql.Tx, id d.ExecutionID, deadline time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO v2_state(execution_id,deadline_at) VALUES(?,?)", id, deadline.UnixMilli())
	return err
}
func (s *ExecutionStore) ReadV2State(ctx context.Context, id d.ExecutionID) (d.V2State, error) {
	var v d.V2State
	var deadline, seq int64
	err := s.db.QueryRowContext(ctx, `SELECT checkpoint_version,deadline_at,generation,recoveries,last_sequence,published_bytes,stored_bytes,logical_requests,http_attempts,terminal,pending_tasks FROM v2_state WHERE execution_id=?`, id).Scan(&v.CheckpointVersion, &deadline, &v.Generation, &v.Recoveries, &seq, &v.PublishedBytes, &v.StoredBytes, &v.LogicalRequests, &v.HTTPAttempts, &v.Terminal, &v.PendingTasks)
	if errors.Is(err, sql.ErrNoRows) {
		return v, d.ErrNotFound
	}
	v.DeadlineAt = time.UnixMilli(deadline).UTC()
	v.LastSequence = strconv.FormatInt(seq, 10)
	return v, err
}
func (s *ExecutionStore) NextV2Task(ctx context.Context, id d.ExecutionID) (d.V2Task, error) {
	return s.readV2Task(ctx, id, " AND done=0 ORDER BY priority,task_key LIMIT 1", nil)
}
func (s *ExecutionStore) ReadV2Task(ctx context.Context, id d.ExecutionID, key string) (d.V2Task, error) {
	return s.readV2Task(ctx, id, " AND task_key=?", []any{key})
}
func (s *ExecutionStore) readV2Task(ctx context.Context, id d.ExecutionID, where string, args []any) (d.V2Task, error) {
	var t d.V2Task
	var blob []byte
	var owner string
	values := append([]any{id}, args...)
	err := s.db.QueryRowContext(ctx, `SELECT task_key,parent_key,priority,revision,done,payload_blob,owner_id FROM v2_tasks JOIN executions ON executions.id=execution_id WHERE execution_id=?`+where, values...).Scan(&t.Key, &t.ParentKey, &t.Priority, &t.Revision, &t.Done, &blob, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return t, d.ErrNotFound
	}
	if err != nil {
		return t, err
	}
	t.Payload, err = s.open(ctx, "v2_tasks", owner, string(id)+":"+t.Key, blob)
	return t, err
}
func (s *ExecutionStore) ReadV2Input(ctx context.Context, id d.ExecutionID, key string) (json.RawMessage, error) {
	var blob []byte
	var owner string
	err := s.db.QueryRowContext(ctx, `SELECT payload_blob,owner_id FROM v2_inputs JOIN executions ON executions.id=execution_id WHERE execution_id=? AND input_key=?`, id, key).Scan(&blob, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, d.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.open(ctx, "v2_inputs", owner, string(id)+":"+key, blob)
}
func (s *ExecutionStore) SeedV2Tasks(ctx context.Context, id d.ExecutionID, owner string, generation int64, tasks []d.V2Task) error {
	return s.CommitV2Task(ctx, d.V2Commit{ID: id, OwnerID: owner, Generation: generation, NewTasks: tasks, Limits: d.DefaultV2Limits()})
}
func (s *ExecutionStore) ReserveV2Budget(ctx context.Context, id d.ExecutionID, generation int64, kind string, maximum int64) error {
	if s.BeforeV2Write != nil {
		if err := s.BeforeV2Write(ctx); err != nil {
			return err
		}
	}
	column := "logical_requests"
	if kind == "attempt" {
		column = "http_attempts"
	} else if kind != "logical" {
		return errors.New("unknown durable budget")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE v2_state SET `+column+`=`+column+`+1 WHERE execution_id=? AND generation=? AND terminal=0 AND `+column+`<?`, id, generation, maximum)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return d.ErrV2Limit
	}
	return err
}
func (s *ExecutionStore) CommitV2Task(ctx context.Context, c d.V2Commit) error {
	if s.BeforeV2Write != nil {
		if err := s.BeforeV2Write(ctx); err != nil {
			return err
		}
	}
	if c.Limits.MaxPublishedBytes == 0 {
		c.Limits = d.DefaultV2Limits()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner string
	var generation, seq, published int64
	var terminal bool
	err = tx.QueryRowContext(ctx, `SELECT owner_id,generation,last_sequence,published_bytes,terminal FROM v2_state JOIN executions ON executions.id=execution_id WHERE execution_id=?`, c.ID).Scan(&owner, &generation, &seq, &published, &terminal)
	if err != nil {
		return err
	}
	if owner != c.OwnerID || generation != c.Generation || terminal {
		return d.ErrVersionConflict
	}
	if c.Task.Key != "" {
		blob, err := s.seal(ctx, "v2_tasks", owner, string(c.ID)+":"+c.Task.Key, c.Task.Payload)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE v2_tasks SET payload_blob=?,plaintext_size=?,done=?,revision=revision+1 WHERE execution_id=? AND task_key=? AND revision=? AND done=0`, blob, len(c.Task.Payload), c.Task.Done, c.ID, c.Task.Key, c.Task.Revision)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return d.ErrVersionConflict
		}
	}
	for _, t := range c.NewTasks {
		blob, err := s.seal(ctx, "v2_tasks", owner, string(c.ID)+":"+t.Key, t.Payload)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO v2_tasks(execution_id,task_key,parent_key,priority,payload_blob,plaintext_size) VALUES(?,?,?,?,?,?) ON CONFLICT(execution_id,task_key) DO NOTHING`, c.ID, t.Key, t.ParentKey, t.Priority, blob, len(t.Payload))
		if err != nil {
			return err
		}
	}
	for _, input := range c.Inputs {
		blob, err := s.seal(ctx, "v2_inputs", owner, string(c.ID)+":"+input.Key, input.Payload)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO v2_inputs(execution_id,input_key,payload_blob,plaintext_size) VALUES(?,?,?,?) ON CONFLICT(execution_id,input_key) DO NOTHING`, c.ID, input.Key, blob, len(input.Payload))
		if err != nil {
			return err
		}
	}
	for _, key := range c.DeleteInputs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM v2_inputs WHERE execution_id=? AND input_key=?`, c.ID, key); err != nil {
			return err
		}
	}

	for _, chunk := range c.Chunks {
		seq++
		n, err := s.insertV2Chunk(ctx, tx, owner, c.ID, seq, chunk)
		if err != nil {
			return err
		}
		published += n
	}
	var tasks, nodes, stored int64
	err = tx.QueryRowContext(ctx, `SELECT pending_tasks,node_count,stored_bytes FROM v2_state WHERE execution_id=?`, c.ID).Scan(&tasks, &nodes, &stored)
	if err != nil {
		return err
	}
	var other, reservations int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(CASE WHEN execution_id<>? THEN stored_bytes ELSE 0 END),0),COALESCE(sum(CASE WHEN terminal=0 THEN 262144 ELSE 0 END),0) FROM v2_state`, c.ID).Scan(&other, &reservations); err != nil {
		return err
	}
	if published > c.Limits.MaxPublishedBytes-(256<<10) || seq >= c.Limits.MaxChunks || nodes > c.Limits.MaxNodes || tasks > c.Limits.MaxPendingTasks || stored+other+reservations > c.Limits.MaxStoredBytes {
		return d.ErrV2Limit
	}
	_, err = tx.ExecContext(ctx, `UPDATE v2_state SET last_sequence=?,published_bytes=?,stored_bytes=? WHERE execution_id=?`, seq, published, stored, c.ID)
	if err != nil {
		return err
	}
	if c.Progress != nil {
		raw, err := json.Marshal(c.Progress)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE executions SET progress_json=? WHERE id=?`, string(raw), c.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *ExecutionStore) insertV2Chunk(ctx context.Context, tx *sql.Tx, owner string, id d.ExecutionID, seq int64, chunk d.V2Chunk) (int64, error) {
	chunk.Version = 2
	chunk.ExecutionID = id
	chunk.Sequence = strconv.FormatInt(seq, 10)
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(chunk)
	if err != nil {
		return 0, err
	}
	if len(raw) > 256<<10 {
		return 0, d.ErrV2ValueTooLarge
	}
	blob, err := s.seal(ctx, "v2_chunks", owner, string(id)+":"+chunk.Sequence, raw)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO v2_chunks(execution_id,sequence,payload_blob,plaintext_size) VALUES(?,?,?,?)`, id, seq, blob, len(raw))
	if err != nil {
		return 0, err
	}
	if chunk.Type == "data" && chunk.NodeID != "" {
		if err = s.projectV2Node(ctx, tx, owner, id, seq, chunk); err != nil {
			return 0, err
		}
	}
	return int64(len(raw)), nil
}
func (s *ExecutionStore) projectV2Node(ctx context.Context, tx *sql.Tx, owner string, id d.ExecutionID, seq int64, c d.V2Chunk) error {
	node := d.V2Node{NodeID: c.NodeID, ParentID: c.ParentID, Key: c.Key, Index: c.Index, Path: c.Path, Kind: c.Operation, Value: c.Value, Status: "pending", Cursor: strconv.FormatInt(seq, 10)}
	if c.Operation == "close" {
		var blob []byte
		err := tx.QueryRowContext(ctx, `SELECT payload_blob FROM v2_nodes WHERE execution_id=? AND node_id=?`, id, c.NodeID).Scan(&blob)
		if err != nil {
			return err
		}
		raw, err := s.open(ctx, "v2_nodes", owner, string(id)+":"+c.NodeID, blob)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &node); err != nil {
			return err
		}
		node.Status = c.Status
	} else if c.Operation == "value" {
		node.Status = "completed"
		if c.Status != "" {
			node.Status = c.Status
		}
	}
	raw, err := json.Marshal(node)
	if err != nil {
		return err
	}
	blob, err := s.seal(ctx, "v2_nodes", owner, string(id)+":"+c.NodeID, raw)
	if err != nil {
		return err
	}
	position := seq
	if node.Index != nil {
		position = *node.Index + 1
	}
	if c.Operation == "close" {
		_, err = tx.ExecContext(ctx, `UPDATE v2_nodes SET payload_blob=?,plaintext_size=? WHERE execution_id=? AND node_id=?`, blob, len(raw), id, c.NodeID)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO v2_nodes(execution_id,node_id,parent_id,position,payload_blob,plaintext_size) VALUES(?,?,?,?,?,?)`, id, c.NodeID, c.ParentID, position, blob, len(raw))
	}
	return err
}
func (s *ExecutionStore) appendV2Terminal(ctx context.Context, tx *sql.Tx, owner string, c d.ExecutionCompletion) error {
	var seq int64
	if err := tx.QueryRowContext(ctx, `SELECT last_sequence FROM v2_state WHERE execution_id=?`, c.ID).Scan(&seq); err != nil {
		return err
	}
	if c.LastSequence != "" && c.LastSequence != strconv.FormatInt(seq+1, 10) {
		return d.ErrVersionConflict
	}
	value, err := json.Marshal(map[string]any{"errorSummary": c.Report.ErrorSummary, "roots": c.Report.Roots, "error": c.Error})
	if err != nil {
		return err
	}
	size, err := s.insertV2Chunk(ctx, tx, owner, c.ID, seq+1, d.V2Chunk{Type: "terminal", Status: string(c.Status), CreatedAt: c.FinishedAt, Value: value})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE v2_state SET terminal=1,last_sequence=?,published_bytes=published_bytes+?,stored_bytes=stored_bytes WHERE execution_id=? AND terminal=0`, seq+1, size, c.ID)
	return err
}
