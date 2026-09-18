package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"time"
)

type AdmissionGate interface {
	Acquire(context.Context, domain.AdmissionRequest) (AdmissionPermit, error)
	Cooldown(context.Context, string, time.Time) error
}
