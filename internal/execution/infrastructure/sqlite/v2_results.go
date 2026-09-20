package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"strconv"
	"time"
)

func (s *ExecutionStore) insertV2Results(ctx context.Context, tx *sql.Tx, c d.V2Commit, lastChunk int64) error {
	if len(c.Results) == 0 {
		return nil
	}
	var enabled bool
	var sequence int64
	if err := tx.QueryRowContext(ctx, `SELECT results_enabled,last_result_sequence FROM v2_state WHERE execution_id=?`, c.ID).Scan(&enabled, &sequence); err != nil {
		return err
	}
	if !enabled {
		return d.ErrV2ResultsUnavailable
	}
	for _, result := range c.Results {
		if result.FirstChunk < 1 || result.LastChunk < result.FirstChunk || result.LastChunk > lastChunk {
			return errors.New("invalid completed result range")
		}
		var nodeBlob []byte
		if err := tx.QueryRowContext(ctx, `SELECT payload_blob FROM v2_nodes WHERE execution_id=? AND node_id=?`, c.ID, result.NodeID).Scan(&nodeBlob); err != nil {
			return err
		}
		nodeRaw, err := s.open(ctx, "v2_nodes", c.OwnerID, string(c.ID)+":"+result.NodeID, nodeBlob)
		if err != nil {
			return err
		}
		var node d.V2Node
		if err = json.Unmarshal(nodeRaw, &node); err != nil {
			return err
		}
		if node.Status != result.Status || node.Status == "pending" {
			return errors.New("result node is not closed with expected status")
		}
		sequence++
		result.Version = 1
		result.ExecutionID = c.ID
		result.Sequence = strconv.FormatInt(sequence, 10)
		result.Type = "node.finished"
		// Node IDs are deterministic within the execution; this identity survives replay.
		result.EventID = string(c.ID) + ":" + result.NodeID
		if result.OccurredAt.IsZero() {
			result.OccurredAt = time.Now().UTC()
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		blob, err := s.seal(ctx, "v2_results", c.OwnerID, string(c.ID)+":"+result.NodeID, raw)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO v2_results(execution_id,node_id,sequence,event_id,payload_blob,plaintext_size,first_chunk,last_chunk,occurred_at) VALUES(?,?,?,?,?,?,?,?,?)`, c.ID, result.NodeID, sequence, result.EventID, blob, len(raw), result.FirstChunk, result.LastChunk, result.OccurredAt.UnixMilli())
		if err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE v2_state SET last_result_sequence=? WHERE execution_id=?`, sequence, c.ID)
	return err
}

func (s *ExecutionStore) resultState(ctx context.Context, owner string, id d.ExecutionID, now time.Time) (d.V2State, error) {
	state, err := s.checkV2Read(ctx, owner, id, now)
	if err != nil {
		return state, err
	}
	if !state.ResultsEnabled {
		return state, d.ErrV2ResultsUnavailable
	}
	return state, nil
}

func (s *ExecutionStore) ReadV2Results(ctx context.Context, owner string, id d.ExecutionID, after, through int64, limit int, now time.Time) (d.V2ResultPage, error) {
	page := d.V2ResultPage{Results: []d.V2Result{}, NextCursor: strconv.FormatInt(after, 10)}
	state, err := s.resultState(ctx, owner, id, now)
	if err != nil {
		return page, err
	}
	last := state.ResultCount
	if after < 0 || after > last || through < 0 || through > last || through > 0 && after > through {
		return page, d.ErrV2Cursor
	}
	if through > 0 {
		last = through
	}
	page.LastSequence = strconv.FormatInt(last, 10)
	page.Terminal = state.Terminal
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,payload_blob FROM v2_results WHERE execution_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`, id, after, last, limit)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	used := 256
	for rows.Next() {
		var node string
		var blob []byte
		if err = rows.Scan(&node, &blob); err != nil {
			return page, err
		}
		raw, err := s.open(ctx, "v2_results", owner, string(id)+":"+node, blob)
		if err != nil {
			return page, err
		}
		if used+len(raw) > 1<<20 && len(page.Results) > 0 {
			break
		}
		used += len(raw) + 1
		var result d.V2Result
		if err = json.Unmarshal(raw, &result); err != nil {
			return page, err
		}
		page.Results = append(page.Results, result)
		page.NextCursor = result.Sequence
	}
	next, _ := strconv.ParseInt(page.NextCursor, 10, 64)
	page.HasMore = next < last
	return page, rows.Err()
}

func (s *ExecutionStore) ReadV2Result(ctx context.Context, owner string, id d.ExecutionID, node string, now time.Time) (d.V2Result, error) {
	var result d.V2Result
	if _, err := s.resultState(ctx, owner, id, now); err != nil {
		return result, err
	}
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload_blob,first_chunk,last_chunk FROM v2_results WHERE execution_id=? AND node_id=?`, id, node).Scan(&blob, &result.FirstChunk, &result.LastChunk)
	if errors.Is(err, sql.ErrNoRows) {
		return result, d.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	raw, err := s.open(ctx, "v2_results", owner, string(id)+":"+node, blob)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
}
