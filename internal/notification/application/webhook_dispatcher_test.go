package application

import (
	"context"
	"fmt"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfterBeyondDeliveryDeadlineEndsDelivery(t *testing.T) {
	now := time.Unix(100, 0)
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", DeadlineAt: now.Add(time.Minute)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		return notification.WebhookAttempt{StatusCode: 429, Retryable: true, RetryAfter: now.Add(time.Hour), ErrorCode: "WEBHOOK_HTTP_ERROR"}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}, Options: DispatcherOptions{InstanceID: "instance"}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved := outbox.saved[0]
	if saved.next != nil || saved.code != "DELIVERY_DEADLINE_EXCEEDED" {
		t.Fatal(saved)
	}
}
func TestRetryAfterIsNeverShortenedToBackoff(t *testing.T) {
	now := time.Unix(100, 0)
	after := now.Add(10 * time.Minute)
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", DeadlineAt: now.Add(time.Hour)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		return notification.WebhookAttempt{StatusCode: 503, Retryable: true, RetryAfter: after, ErrorCode: "WEBHOOK_HTTP_ERROR"}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outbox.saved[0].next == nil || !outbox.saved[0].next.Equal(after) {
		t.Fatal(outbox.saved)
	}
}
func TestAttemptLimitDoesNotScheduleMoreRetries(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", Attempts: 7, DeadlineAt: now.Add(time.Hour)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		return notification.WebhookAttempt{StatusCode: 503, Retryable: true, ErrorCode: "WEBHOOK_HTTP_ERROR"}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}, Options: DispatcherOptions{MaxAttempts: 8}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outbox.saved[0].next != nil || outbox.saved[0].delivery.Attempts != 8 {
		t.Fatal(outbox.saved)
	}
}
func TestDispatcherBatchAndConcurrencyAreBounded(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{}
	for index := 0; index < 10; index++ {
		outbox.pending = append(outbox.pending, notification.WebhookDelivery{ID: fmt.Sprint(index), DeadlineAt: now.Add(time.Hour)})
	}
	var active, peak atomic.Int32
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
		return notification.WebhookAttempt{StatusCode: 204}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Options: DispatcherOptions{MaxConcurrent: 3}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(outbox.saved) != 3 || len(outbox.pending) != 7 || peak.Load() > 3 {
		t.Fatal(outbox, peak.Load())
	}
}
func TestShutdownPersistsAttemptWithIndependentContext(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", DeadlineAt: now.Add(time.Hour)}}}
	ctx, cancel := context.WithCancel(context.Background())
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		cancel()
		return notification.WebhookAttempt{ErrorCode: "WEBHOOK_INTERRUPTED", Retryable: true}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender}
	if err := dispatcher.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if outbox.saved[0].contextError != nil || outbox.saved[0].next == nil {
		t.Fatal(outbox.saved)
	}
}
func TestSenderPanicIsContainedByDelivery(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", DeadlineAt: now.Add(time.Hour)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		panic("secret")
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outbox.saved[0].code != "WEBHOOK_SENDER_PANIC" {
		t.Fatal(outbox.saved)
	}
}

func TestExpiredOrExhaustedClaimsAreNotSent(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "old", DeadlineAt: now.Add(-time.Second)}, {ID: "exhausted", Attempts: 8, DeadlineAt: now.Add(time.Hour)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		t.Error("out-of-budget delivery sent")
		return notification.WebhookAttempt{}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(outbox.saved) != 2 {
		t.Fatal(outbox.saved)
	}
	for _, saved := range outbox.saved {
		if saved.next != nil || saved.success {
			t.Fatal(saved)
		}
	}
}
func TestRunStopsAfterJoiningDeliveries(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", DeadlineAt: now.Add(time.Hour)}}}
	ctx, cancel := context.WithCancel(context.Background())
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		cancel()
		return notification.WebhookAttempt{StatusCode: 200}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender}
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(outbox.saved) != 1 || !outbox.saved[0].success {
		t.Fatal(outbox.saved)
	}
}
func TestRunRejectsLeaseShorterThanAttemptPersistenceWindow(t *testing.T) {
	dispatcher := WebhookDispatcher{Options: DispatcherOptions{ClaimLease: time.Second, AttemptTimeout: time.Second}}
	if err := dispatcher.Run(context.Background()); err == nil {
		t.Fatal("invalid lease accepted")
	}
}
func TestBackoffRemainsBoundedWithDeterministicJitter(t *testing.T) {
	now := time.Now()
	outbox := &fakeOutbox{pending: []notification.WebhookDelivery{{ID: "d", Attempts: 6, DeadlineAt: now.Add(time.Hour)}}}
	sender := fakeSender(func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error) {
		return notification.WebhookAttempt{StatusCode: 500, Retryable: true, ErrorCode: "WEBHOOK_HTTP_ERROR"}, nil
	})
	dispatcher := WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}, Options: DispatcherOptions{BaseDelay: time.Second, MaxBackoff: 3 * time.Second, Jitter: func(delay time.Duration) time.Duration { return delay }}}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !outbox.saved[0].next.Equal(now.Add(3 * time.Second)) {
		t.Fatal(outbox.saved)
	}
}
