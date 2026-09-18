package worker

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type memoryExecutionStore struct {
	snapshot           execution.ExecutionSnapshot
	current            execution.Execution
	completion         execution.ExecutionCompletion
	finished           bool
	finishContextError error
	failFinish         error
}

func (s *memoryExecutionStore) Create(context.Context, execution.ExecutionSnapshot) (execution.Execution, bool, error) {
	panic("unused")
}
func (s *memoryExecutionStore) FindReplay(context.Context, string, string) (execution.Execution, error) {
	panic("unused")
}
func (s *memoryExecutionStore) Load(context.Context, execution.ExecutionID) (execution.ExecutionSnapshot, execution.Execution, error) {
	return s.snapshot, s.current, nil
}
func (s *memoryExecutionStore) Claim(_ context.Context, _ execution.ExecutionID, _ string, version int64, _ time.Time) (bool, error) {
	if s.current.Status != execution.Pending || version != s.current.Version {
		return false, nil
	}
	s.current.Status = execution.Running
	s.current.Version++
	return true, nil
}
func (s *memoryExecutionStore) Finish(ctx context.Context, completion execution.ExecutionCompletion) error {
	s.finishContextError = ctx.Err()
	s.finished = true
	s.completion = completion
	return s.failFinish
}
