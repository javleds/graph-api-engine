package worker

import (
	"context"
	"time"

	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type expiredParallelStore struct {
	ports.V2Store
}

func (s expiredParallelStore) ReadV2State(ctx context.Context, id domain.ExecutionID) (domain.V2State, error) {
	state, err := s.V2Store.ReadV2State(ctx, id)
	state.DeadlineAt = time.Now().Add(-time.Minute)
	return state, err
}
