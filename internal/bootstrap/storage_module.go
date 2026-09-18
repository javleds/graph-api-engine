package bootstrap

import (
	"context"
	"fmt"
	executionsql "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	platformsql "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"os"
	"path/filepath"
)

func (a *Application) openStorage(ctx context.Context) (*security.AESGCMProtector, []byte, error) {
	keys := map[string][]byte{}
	for id, path := range a.Config.Security.Encryption.KeyFiles {
		key, err := readKey(path)
		if err != nil {
			return nil, nil, err
		}
		keys[id] = key
	}
	identity, err := readKey(a.Config.Security.IdentityKeyFile)
	if err != nil {
		return nil, nil, err
	}
	protector, err := security.NewAESGCMProtector(a.Config.Security.Encryption.ActiveKeyID, keys)
	if err != nil {
		return nil, nil, err
	}
	a.Lock, err = platformsql.AcquireInstanceLock(filepath.Dir(a.Config.Storage.Path))
	if err != nil {
		return nil, nil, err
	}
	a.EngineDB, err = platformsql.OpenConfigured(ctx, a.Config.Storage.Path, false, a.Config.Storage.BusyTimeout.Value())
	if err != nil {
		return nil, nil, err
	}
	a.CacheDB, err = platformsql.OpenConfigured(ctx, a.Config.Cache.Path, true, a.Config.Cache.BusyTimeout.Value())
	if err != nil {
		return nil, nil, err
	}
	if err = platformsql.Migrate(ctx, a.EngineDB, "engine"); err != nil {
		return nil, nil, err
	}
	if err = platformsql.Migrate(ctx, a.CacheDB, "cache"); err != nil {
		return nil, nil, err
	}
	a.Store = executionsql.NewExecutionStore(a.EngineDB, protector, a.Config.Webhooks.MaxPendingDeliveries)
	var version string
	if err = a.EngineDB.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		return nil, nil, err
	}
	a.Logger.Info("storage opened", "sqlite_version", version)
	return protector, identity, nil
}
func readKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read configured key file")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key files must contain exactly 32 bytes")
	}
	return key, nil
}
