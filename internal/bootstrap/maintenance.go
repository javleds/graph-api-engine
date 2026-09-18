package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"path/filepath"
	"syscall"
	"time"
)

func (a *Application) checkCapacity(ctx context.Context) error {
	if !a.ready.Load() {
		return domain.StorageError("ENGINE_BUSY")
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(a.Config.Storage.Path), &stat); err != nil {
		return domain.StorageError("STORAGE_UNAVAILABLE")
	}
	if uint64(stat.Bavail)*uint64(stat.Bsize) < uint64(a.Config.Storage.MinimumFreeDiskBytes) {
		return domain.StorageError("ENGINE_BUSY")
	}
	return ctx.Err()
}
func (a *Application) isReady(ctx context.Context) bool {
	return a.ready.Load() && a.EngineDB.PingContext(ctx) == nil
}
func (a *Application) maintenance(ctx context.Context) {
	timer := time.NewTicker(a.Config.Maintenance.Interval.Value())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-timer.C:
			if err := a.Store.Purge(ctx, now, a.Config.Maintenance.BatchSize); err != nil {
				a.Logger.Error("retention maintenance failed")
			}
			if a.activeFlights != nil {
				a.Metrics.Set("flights_active", float64(a.activeFlights()))
			}
			var cacheBytes int64
			if err := a.CacheDB.QueryRowContext(ctx, "SELECT COALESCE(SUM(plaintext_size),0) FROM http_cache").Scan(&cacheBytes); err == nil {
				a.Metrics.Set("cache_bytes", float64(cacheBytes))
			}
			if _, err := a.Cache.DeleteExpired(ctx, a.Config.Maintenance.BatchSize); err != nil {
				a.Metrics.Add("cache_errors_total", 1)
			}
		}
	}
}
