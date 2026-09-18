package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
)

type measuredCache struct {
	cache ports.ResponseCache
	app   *Application
}

func (c measuredCache) Get(ctx context.Context, key string) (domain.CacheEntry, bool, error) {
	entry, found, err := c.cache.Get(ctx, key)
	if !found {
		c.app.Metrics.Add("cache_misses_total", 1)
	}
	return entry, found, err
}
func (c measuredCache) Put(ctx context.Context, entry domain.CacheEntry) error {
	if err := c.app.checkCapacity(ctx); err != nil {
		return nil
	}
	return c.cache.Put(ctx, entry)
}
func (c measuredCache) Delete(ctx context.Context, key string) error { return c.cache.Delete(ctx, key) }
