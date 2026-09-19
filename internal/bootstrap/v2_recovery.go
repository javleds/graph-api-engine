package bootstrap

import (
	"context"
	"time"
)

// Recovered jobs are admitted in bounded batches through the same global slots
// as new requests. Claim CAS makes overlapping observations harmless.
func (a *Application) resumeV2(ctx context.Context) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		if a.ready.Load() {
			ids, err := a.Store.PendingV2IDs(ctx, a.Config.Executions.MaxConcurrent)
			if err != nil {
				if ctx.Err() == nil {
					a.ready.Store(false)
					a.Logger.Error("v2 recovery read failed")
				}
				return
			}
			for _, id := range ids {
				reservation, err := a.Dispatcher.Reserve(ctx)
				if err != nil {
					break
				}
				if err = reservation.Commit(ctx, id); err != nil {
					reservation.Cancel()
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
