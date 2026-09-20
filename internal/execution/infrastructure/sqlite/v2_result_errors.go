package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"strconv"
)

func (s *ExecutionStore) insertV2ResultErrors(ctx context.Context, tx *sql.Tx, c d.V2Commit) error {
	for _, item := range c.ResultErrors {
		var sequence int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM v2_result_errors WHERE execution_id=? AND result_node_id=?`, c.ID, item.NodeID).Scan(&sequence); err != nil {
			return err
		}
		raw, err := json.Marshal(item.Failure)
		if err != nil {
			return err
		}
		blob, err := s.seal(ctx, "v2_result_errors", c.OwnerID, string(c.ID)+":"+item.NodeID+":"+strconv.FormatInt(sequence, 10), raw)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO v2_result_errors(execution_id,result_node_id,sequence,payload_blob,plaintext_size) VALUES(?,?,?,?,?)`, c.ID, item.NodeID, sequence, blob, len(raw)); err != nil {
			return err
		}
	}
	return nil
}
