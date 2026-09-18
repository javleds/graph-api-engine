package application

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"sync"
	"time"
)

type fakeOutbox struct {
	mu         sync.Mutex
	pending    []notification.WebhookDelivery
	saved      []savedAttempt
	recoveries int
}

func (o *fakeOutbox) Claim(_ context.Context, owner string, now time.Time, lease time.Duration) (*notification.WebhookDelivery, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.pending) == 0 {
		return nil, nil
	}
	d := o.pending[0]
	o.pending = o.pending[1:]
	d.Attempts++
	d.LeaseOwner = owner
	d.LeaseExpiresAt = now.Add(lease)
	return &d, nil
}
func (o *fakeOutbox) SaveAttempt(ctx context.Context, d notification.WebhookDelivery, success bool, next *time.Time, status int, code string, _ time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.saved = append(o.saved, savedAttempt{delivery: d, success: success, next: next, status: status, code: code, contextError: ctx.Err()})
	return nil
}
func (o *fakeOutbox) Recover(context.Context, string, time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.recoveries++
	return nil
}
func (o *fakeOutbox) Status(context.Context, string, execution.ExecutionID) (*notification.WebhookStatus, error) {
	return nil, nil
}
