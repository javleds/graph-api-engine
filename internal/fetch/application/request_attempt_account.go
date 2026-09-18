package application

import "context"

type requestAttemptAccount struct {
	retried func()
	reserve func(context.Context) error
	sent    func()
}

func (a requestAttemptAccount) Reserve(ctx context.Context) error {
	if a.reserve != nil {
		return a.reserve(ctx)
	}
	return nil
}

func (a requestAttemptAccount) Sent() {
	if a.sent != nil {
		a.sent()
	}
}

func (a requestAttemptAccount) Retried() {
	if a.retried != nil {
		a.retried()
	}
}
