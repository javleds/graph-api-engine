package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type ResponseCache interface {
	Get(context.Context, string) (domain.CacheEntry, bool, error)
	Put(context.Context, domain.CacheEntry) error
	Delete(context.Context, string) error
}
