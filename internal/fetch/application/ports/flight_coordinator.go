package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type FlightCoordinator interface {
	Do(context.Context, string, func(context.Context) (domain.RawResponse, error)) (domain.FlightResult, error)
}
