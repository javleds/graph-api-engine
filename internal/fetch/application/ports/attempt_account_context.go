package ports

import "context"

type attemptAccountContextKey struct{}

func WithAttemptAccount(ctx context.Context, account AttemptAccount) context.Context {
	return context.WithValue(ctx, attemptAccountContextKey{}, account)
}

func AttemptAccountFromContext(ctx context.Context) (AttemptAccount, bool) {
	account, ok := ctx.Value(attemptAccountContextKey{}).(AttemptAccount)
	return account, ok
}
