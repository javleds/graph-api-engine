package application

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type inlineFlight struct{}

func (inlineFlight) Do(ctx context.Context, _ string, work func(context.Context) (domain.RawResponse, error)) (domain.FlightResult, error) {
	response, err := work(ctx)
	return domain.FlightResult{Response: response}, err
}
