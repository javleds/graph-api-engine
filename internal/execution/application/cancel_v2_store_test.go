package application

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type cancelV2Store struct {
	*lifecycleStore
	ports.V2Store
	task      domain.V2Task
	taskErr   error
	readState bool
}

func (s *cancelV2Store) ReadV2Task(context.Context, domain.ExecutionID, string) (domain.V2Task, error) {
	if !s.readState {
		panic("checkpoint read before sequence")
	}
	return s.task, s.taskErr
}
func (s *cancelV2Store) ReadV2State(context.Context, domain.ExecutionID) (domain.V2State, error) {
	s.readState = true
	return domain.V2State{LastSequence: "12", LogicalRequests: 3, HTTPAttempts: 4}, nil
}
