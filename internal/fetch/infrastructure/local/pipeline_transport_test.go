package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type pipelineTransport func(context.Context, domain.OutboundRequest) (domain.RawResponse, error)

func (f pipelineTransport) Execute(ctx context.Context, request domain.OutboundRequest) (domain.RawResponse, error) {
	return f(ctx, request)
}
