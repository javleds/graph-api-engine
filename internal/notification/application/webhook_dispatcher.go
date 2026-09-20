package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/notification/application/ports"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"math/rand/v2"
	"sync"
	"time"
)

// WebhookDispatcher only schedules durable notifications, never execution trees.
type WebhookDispatcher struct {
	Outbox  ports.WebhookOutbox
	Sender  ports.WebhookSender
	Options DispatcherOptions
	Clock   clock.Clock
	OnError func(error)
}

func (d *WebhookDispatcher) Run(ctx context.Context) error {
	configured := *d
	configured.Options = d.Options.defaults()
	if configured.Clock == nil {
		configured.Clock = clock.RealClock{}
	}
	d = &configured
	if d.Options.ClaimLease <= d.Options.AttemptTimeout+d.Options.PersistenceTimeout {
		return errors.New("webhook claim lease must exceed attempt and persistence timeouts")
	}
	completed := make(chan error, d.Options.MaxConcurrent)
	active := 0
	report := func(err error) {
		if err != nil && d.OnError != nil {
			d.OnError(err)
		}
	}
	for ctx.Err() == nil {
		report(d.Outbox.Recover(ctx, d.Options.InstanceID, d.Clock.Now()))
		for active < d.Options.MaxConcurrent && ctx.Err() == nil {
			delivery, err := d.Outbox.Claim(ctx, d.Options.InstanceID, d.Clock.Now(), d.Options.ClaimLease)
			if err != nil {
				report(err)
				break
			}
			if delivery == nil {
				break
			}
			active++
			go func(delivery domain.WebhookDelivery) { completed <- d.deliver(ctx, delivery, d.Options, d.Clock) }(*delivery)
		}
		timer := d.Clock.NewTimer(d.Options.PollInterval)
		select {
		case <-ctx.Done():
		case err := <-completed:
			active--
			report(err)
		case <-timer.C():
		}
		timer.Stop()
	}
	for active > 0 {
		report(<-completed)
		active--
	}
	return nil
}
func (d *WebhookDispatcher) DispatchOnce(ctx context.Context) error {
	options := d.Options.defaults()
	source := d.Clock
	if source == nil {
		source = clock.RealClock{}
	}
	if err := d.Outbox.Recover(ctx, options.InstanceID, source.Now()); err != nil {
		return err
	}
	var group sync.WaitGroup
	failures := make([]error, options.MaxConcurrent)
	launched := 0
	for index := 0; index < options.MaxConcurrent; index++ {
		if ctx.Err() != nil {
			break
		}
		delivery, err := d.Outbox.Claim(ctx, options.InstanceID, source.Now(), options.ClaimLease)
		if err != nil {
			group.Wait()
			return err
		}
		if delivery == nil {
			break
		}
		index := index
		launched++
		group.Add(1)
		go func() { defer group.Done(); failures[index] = d.deliver(ctx, *delivery, options, source) }()
	}
	group.Wait()
	for _, err := range failures[:launched] {
		if err != nil {
			return err
		}
	}
	return nil
}
func (d *WebhookDispatcher) deliver(ctx context.Context, delivery domain.WebhookDelivery, options DispatcherOptions, source clock.Clock) (err error) {
	// A sender panic closes this delivery without crashing the delivery loop.
	defer func() {
		if recover() != nil {
			persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), options.PersistenceTimeout)
			defer cancel()
			err = d.Outbox.SaveAttempt(persist, delivery, false, nil, 0, "WEBHOOK_SENDER_PANIC", source.Now())
		}
	}()
	if delivery.Attempts > options.MaxAttempts || !source.Now().Before(delivery.DeadlineAt) {
		code := "WEBHOOK_ATTEMPTS_EXHAUSTED"
		if !source.Now().Before(delivery.DeadlineAt) {
			code = "DELIVERY_DEADLINE_EXCEEDED"
		}
		persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), options.PersistenceTimeout)
		defer cancel()
		return d.Outbox.SaveAttempt(persist, delivery, false, nil, 0, code, source.Now())
	}
	attemptCtx, cancel := context.WithTimeout(ctx, options.AttemptTimeout)
	attempt, sendErr := d.Sender.Send(attemptCtx, delivery)
	cancel()
	if sendErr != nil {
		attempt = domain.WebhookAttempt{ErrorCode: "WEBHOOK_INTERNAL_ERROR"}
	}
	now := source.Now()
	var next *time.Time
	if !attempt.Accepted() && attempt.Retryable && delivery.Attempts < options.MaxAttempts {
		delay := retryDelay(delivery.Attempts, options)
		candidate := now.Add(delay)
		if attempt.RetryAfter.After(candidate) {
			candidate = attempt.RetryAfter
		}
		if candidate.Before(delivery.DeadlineAt) {
			next = &candidate
		} else {
			attempt.ErrorCode = "DELIVERY_DEADLINE_EXCEEDED"
		}
	}
	if !attempt.Accepted() && !now.Before(delivery.DeadlineAt) {
		next = nil
		attempt.ErrorCode = "DELIVERY_DEADLINE_EXCEEDED"
	}
	persist, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), options.PersistenceTimeout)
	defer persistCancel()
	return d.Outbox.SaveAttempt(persist, delivery, attempt.Accepted(), next, attempt.StatusCode, attempt.ErrorCode, now)
}
func retryDelay(attempt int, options DispatcherOptions) time.Duration {
	delay := options.BaseDelay
	for count := 1; count < attempt && delay < options.MaxBackoff; count++ {
		if delay > options.MaxBackoff/2 {
			delay = options.MaxBackoff
			break
		}
		delay *= 2
	}
	if delay > options.MaxBackoff {
		delay = options.MaxBackoff
	}
	if options.Jitter != nil {
		return max(time.Duration(0), min(delay, options.Jitter(delay)))
	}
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(delay) + 1))
}
