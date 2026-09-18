package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/platform/config"
	platformsql "github.com/javleds/api-engine/internal/platform/sqlite"
	"path/filepath"
)

func Migrate(ctx context.Context, cfg *config.Config) error {
	lock, err := platformsql.AcquireInstanceLock(filepath.Dir(cfg.Storage.Path))
	if err != nil {
		return err
	}
	defer lock.Close()
	engine, err := platformsql.OpenConfigured(ctx, cfg.Storage.Path, false, cfg.Storage.BusyTimeout.Value())
	if err != nil {
		return err
	}
	defer engine.Close()
	if err = platformsql.Migrate(ctx, engine, "engine"); err != nil {
		return err
	}
	cache, err := platformsql.OpenConfigured(ctx, cfg.Cache.Path, true, cfg.Cache.BusyTimeout.Value())
	if err != nil {
		return err
	}
	defer cache.Close()
	return platformsql.Migrate(ctx, cache, "cache")
}
