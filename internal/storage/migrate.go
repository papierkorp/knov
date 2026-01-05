// Package storage - migration between storage providers
package storage

import (
	"fmt"

	"knov/internal/logging"
)

// MigrateStorage copies all data from one storage to another
// useful for switching between providers (json -> sqlite, etc.)
func MigrateStorage(from, to Storage, storageType string) error {
	logging.LogInfo("migrating %s storage", storageType)

	keys, err := from.List("")
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	migrated := 0
	failed := 0

	for _, key := range keys {
		data, err := from.Get(key)
		if err != nil {
			logging.LogWarning("failed to read key %s: %v", key, err)
			failed++
			continue
		}

		if data == nil {
			continue
		}

		if err := to.Set(key, data); err != nil {
			logging.LogWarning("failed to write key %s: %v", key, err)
			failed++
			continue
		}

		migrated++
		logging.LogDebug("migrated key: %s", key)
	}

	logging.LogInfo("migration complete: %d migrated, %d failed", migrated, failed)
	return nil
}

// MigrateMetadata migrates metadata storage between providers
func MigrateMetadata(from, to MetadataStorage) error {
	logging.LogInfo("migrating metadata storage")

	// use GetAll for metadata to get everything at once
	allData, err := from.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get all metadata: %w", err)
	}

	// use BulkSet for efficiency
	if err := to.BulkSet(allData); err != nil {
		return fmt.Errorf("failed to bulk set metadata: %w", err)
	}

	logging.LogInfo("migrated %d metadata entries", len(allData))
	return nil
}

// MigrateConfig migrates config storage between providers
func MigrateConfig(from, to Storage) error {
	return MigrateStorage(from, to, "config")
}

// MigrateCache migrates cache storage between providers
func MigrateCache(from, to CacheStorage) error {
	return MigrateStorage(from, to, "cache")
}
