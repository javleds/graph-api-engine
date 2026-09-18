package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type OutboundTransport interface {
	Execute(context.Context, domain.OutboundRequest) (domain.RawResponse, error)
}
