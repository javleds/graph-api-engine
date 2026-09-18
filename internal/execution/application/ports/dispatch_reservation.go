package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type DispatchReservation interface {
	Commit(context.Context, domain.ExecutionID) error
	Cancel()
}
