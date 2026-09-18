package application

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type transportStub func(context.Context, domain.OutboundRequest) (domain.RawResponse, error)

func (f transportStub) Execute(ctx context.Context, r domain.OutboundRequest) (domain.RawResponse, error) {
	return f(ctx, r)
}
