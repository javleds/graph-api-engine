package ports

import (
	"context"
	"time"
)

type QuotaCooldownStore interface {
	Get(context.Context, string) (time.Time, error)
	Extend(context.Context, string, time.Time) error
	DeleteExpired(context.Context, time.Time, int) (int64, error)
}
