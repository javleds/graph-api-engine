package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type Fetcher interface {
	Fetch(context.Context, domain.FetchRequest) (domain.FetchResult, error)
}
