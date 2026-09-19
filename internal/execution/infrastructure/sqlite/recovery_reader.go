package sqlite

import (
	"context"
	d "github.com/javleds/api-engine/internal/execution/domain"
)

// InterruptedIDs is used only while holding the exclusive data-directory lock.
func InterruptedIDs(ctx context.Context, store *ExecutionStore, instance string) ([]d.ExecutionID, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT id FROM executions WHERE api_version=1 AND status IN ('pending','running') AND (worker_instance_id IS NULL OR worker_instance_id<>?) ORDER BY created_at,id`, instance)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []d.ExecutionID{}
	for rows.Next() {
		var id d.ExecutionID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
