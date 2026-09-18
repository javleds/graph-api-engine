package ports

import "context"

type ExecutionDispatcher interface {
	Reserve(context.Context) (DispatchReservation, error)
}
