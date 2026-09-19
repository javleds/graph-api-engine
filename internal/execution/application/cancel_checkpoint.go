package application

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

// Only committed diagnostics are needed; decoding the traversal stack would retain
// potentially large intermediate values during cancellation.
type cancelCheckpoint struct {
	Report   domain.ExecutionErrorReport `json:"report"`
	Progress domain.ExecutionProgress    `json:"progress"`
}

func (c *CancelExecution) readCheckpoint(ctx context.Context, completion *domain.ExecutionCompletion) error {
	store, ok := c.Store.(ports.V2Store)
	if !ok {
		return errors.New("v2 store is required")
	}
	// Capture the sequence before the diagnostics so finalization rejects any
	// intervening publication instead of closing newer chunks with stale progress.
	state, err := store.ReadV2State(ctx, completion.ID)
	if err != nil {
		return err
	}
	sequence, err := strconv.ParseInt(state.LastSequence, 10, 64)
	if err != nil {
		return err
	}
	completion.LastSequence = strconv.FormatInt(sequence+1, 10)
	task, err := store.ReadV2Task(ctx, completion.ID, "coordinator")
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	revision := int64(-1)
	if err == nil {
		revision = task.Revision
		var checkpoint cancelCheckpoint
		if err := json.Unmarshal(task.Payload, &checkpoint); err != nil {
			return err
		}
		completion.Report, completion.Progress = checkpoint.Report, checkpoint.Progress
	}
	completion.ExpectedCheckpointRevision = &revision
	for i := range completion.Report.Roots {
		root := &completion.Report.Roots[i]
		if root.Status == domain.Running {
			root.Status = domain.Failed
			if root.DataAvailable {
				root.Status = domain.CompletedWithErrors
			}
		}
	}
	completion.Progress.LogicalRequests = uint64(state.LogicalRequests)
	completion.Progress.HTTPAttempts = uint64(state.HTTPAttempts)
	return nil
}
