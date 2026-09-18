package worker

import (
	"context"
	"errors"
	fetchdomain "github.com/javleds/api-engine/internal/fetch/domain"
	"sync/atomic"
)

var ErrBudgetExceeded = fetchdomain.AttemptBudgetExceeded{}

// ExecutionBudget counts discovered work before launching it.
type ExecutionBudget struct {
	logical       atomic.Uint64
	attempts      atomic.Uint64
	nodes         atomic.Uint64
	retainedBytes atomic.Uint64
	maxLogical    uint64
	maxAttempts   uint64
	maxNodes      uint64
	maxBytes      uint64
	cancel        context.CancelCauseFunc
}

func (b *ExecutionBudget) request() bool {
	if b.logical.Add(1) > b.maxLogical {
		b.cancel(ErrBudgetExceeded)
		return false
	}
	return true
}
func (b *ExecutionBudget) reserveNodes(count uint64) bool {
	if b.nodes.Add(count) > b.maxNodes {
		b.cancel(ErrBudgetExceeded)
		return false
	}
	return true
}

func (b *ExecutionBudget) attempt(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for {
		used := b.attempts.Load()
		if used >= b.maxAttempts {
			b.cancel(ErrBudgetExceeded)
			return ErrBudgetExceeded
		}
		if b.attempts.CompareAndSwap(used, used+1) {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func (b *ExecutionBudget) retainBytes(count uint64) error {
	if b.retainedBytes.Add(count) > b.maxBytes {
		err := errors.New("RESULT_TOO_LARGE")
		b.cancel(err)
		return err
	}
	return nil
}
