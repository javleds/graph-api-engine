package application

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type cancelReader struct {
	current   domain.Execution
	afterRead func()
}

func (r *cancelReader) Read(_ context.Context, owner string, id domain.ExecutionID) (domain.Execution, error) {
	if owner != r.current.OwnerID || id != r.current.ID {
		return domain.Execution{}, domain.ErrNotFound
	}
	if r.afterRead != nil {
		r.afterRead()
	}
	return r.current, nil
}
