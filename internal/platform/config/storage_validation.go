package config

import (
	"fmt"
	"path/filepath"
)

func validateStorage(storage StorageConfig, cache CacheConfig) error {
	if storage.Path == "" || cache.Path == "" {
		return fmt.Errorf("database paths are required")
	}
	enginePath, err := filepath.Abs(storage.Path)
	if err != nil {
		return err
	}
	cachePath, err := filepath.Abs(cache.Path)
	if err != nil {
		return err
	}
	if enginePath == cachePath {
		return fmt.Errorf("engine and cache databases require distinct paths")
	}
	if storage.Synchronous != "FULL" || cache.Synchronous != "NORMAL" {
		return fmt.Errorf("engine synchronous must be FULL and cache synchronous must be NORMAL")
	}
	return nil
}
func validateQuotaGroups(groups []QuotaGroup) error {
	names := map[string]bool{}
	for i := range groups {
		group := &groups[i]
		if group.Name == "" || names[group.Name] {
			return fmt.Errorf("quota group names must be nonempty and unique")
		}
		names[group.Name] = true
		if group.CredentialMode == "" {
			group.CredentialMode = "separate"
		}
		if group.CredentialMode != "separate" && group.CredentialMode != "shared" {
			return fmt.Errorf("invalid quota credential mode")
		}
		if len(group.Origins) == 0 {
			return fmt.Errorf("quota group origins are required")
		}
		if err := validateOrigins(group.Origins, true); err != nil {
			return err
		}
		if group.Limits.Requests <= 0 || group.Limits.Per <= 0 || group.Limits.Concurrency < 0 {
			return fmt.Errorf("invalid quota group limits")
		}
	}
	return nil
}
