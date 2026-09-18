package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
)

type flightSubscription struct {
	context   context.Context
	account   ports.AttemptAccount
	failure   chan error
	rejected  bool
	rejection error
}
