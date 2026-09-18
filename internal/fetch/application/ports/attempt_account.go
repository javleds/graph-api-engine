package ports

import "context"

// AttemptAccount reserves and records one physical attempt for its subscriber.
type AttemptAccount interface {
	Reserve(context.Context) error
	Sent()
	Retried()
}
