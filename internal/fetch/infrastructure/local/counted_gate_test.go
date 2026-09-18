package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"sync/atomic"
	"time"
)

type countedGate struct {
	gate       ports.AdmissionGate
	admissions atomic.Int32
}

func (g *countedGate) Acquire(ctx context.Context, r domain.AdmissionRequest) (ports.AdmissionPermit, error) {
	permit, err := g.gate.Acquire(ctx, r)
	if err == nil {
		g.admissions.Add(1)
	}
	return permit, err
}
func (g *countedGate) Cooldown(ctx context.Context, scope string, until time.Time) error {
	return g.gate.Cooldown(ctx, scope, until)
}
