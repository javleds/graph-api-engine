package application

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type lifecycleStore struct {
	snapshot  domain.ExecutionSnapshot
	current   domain.Execution
	findCalls int
}

func (s *lifecycleStore) FindReplay(context.Context, string, string) (domain.Execution, error) {
	s.findCalls++
	if s.findCalls == 1 {
		return domain.Execution{}, domain.ErrNotFound
	}
	return s.current, nil
}
func (s *lifecycleStore) Create(context.Context, domain.ExecutionSnapshot) (domain.Execution, bool, error) {
	panic("unexpected creation")
}
func (s *lifecycleStore) Load(context.Context, domain.ExecutionID) (domain.ExecutionSnapshot, domain.Execution, error) {
	return s.snapshot, s.current, nil
}
func (s *lifecycleStore) Claim(context.Context, domain.ExecutionID, string, int64, time.Time) (bool, error) {
	panic("unexpected claim")
}
