package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
)

// flightAttemptAccount transfers sponsorship when a subscriber can no longer pay.
// Only the physical callback accesses selected; subscription membership uses mu.
type flightAttemptAccount struct {
	coordinator *LocalFlightCoordinator
	state       *flightState
	selected    ports.AttemptAccount
}

func (a *flightAttemptAccount) Reserve(ctx context.Context) error {
	a.selected = nil
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		a.coordinator.mu.Lock()
		candidate := a.nextSubscriber()
		if candidate == nil {
			a.state.cancel()
			a.coordinator.mu.Unlock()
			return context.Canceled
		}
		a.coordinator.mu.Unlock()
		var err error
		if candidate.account != nil {
			err = candidate.account.Reserve(ctx)
		}
		if err == nil {
			a.selected = candidate.account
			return nil
		}
		a.coordinator.mu.Lock()
		candidate.rejected = true
		candidate.rejection = err
		select {
		case candidate.failure <- err:
		default:
		}
		a.coordinator.mu.Unlock()
	}
}

func (a *flightAttemptAccount) nextSubscriber() *flightSubscription {
	for _, subscriber := range a.state.subscriptions {
		if !subscriber.rejected && subscriber.context.Err() == nil {
			return subscriber
		}
	}
	return nil
}

func (a *flightAttemptAccount) Sent() {
	if a.selected != nil {
		a.selected.Sent()
	}
}

func (a *flightAttemptAccount) Retried() {
	if a.selected != nil {
		a.selected.Retried()
	}
}
