package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/observability"
	"time"
)

type measuredGate struct {
	gate    ports.AdmissionGate
	metrics *observability.Metrics
}

func (g measuredGate) Acquire(ctx context.Context, request domain.AdmissionRequest) (ports.AdmissionPermit, error) {
	start := time.Now()
	permit, err := g.gate.Acquire(ctx, request)
	g.metrics.Observe("quota_wait_seconds", time.Since(start).Seconds())
	return permit, err
}
func (g measuredGate) Cooldown(ctx context.Context, scope string, until time.Time) error {
	return g.gate.Cooldown(ctx, scope, until)
}
