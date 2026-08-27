// Package cacheStorage provides cache storage functionality
package cacheStorage

import (
	"fmt"

	"knov/internal/logging"
)

// CacheStorage interface defines methods for cache storage. Unlike the other *Storage packages,
// cache holds nothing but derived data rebuilt from files/git on demand (see files.CacheInvalidate
// and the periodic rebuild job) - so it's deliberately not registered with the backup package:
// restoring an old cache snapshot would only reintroduce stale derived data. Restore's recovery
// step refreshes it instead: job.restoreJob's afterRestore flushes it before the restart (a
// rebuild can't run pre-restart - restored storages' handles are already closed), backuptest's
// restoreAndReinit rebuilds it in-process after reinit.
type CacheStorage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	Exists(key string) bool
	Flush() error
}

var storage CacheStorage

// Init initializes cache storage with the specified provider
func Init(provider, storagePath string) error {
	var err error

	switch provider {
	case "sqlite":
		storage, err = newSQLiteStorage(storagePath)
	case "json":
		storage, err = newJSONStorage(storagePath)
	default:
		logging.LogWarning(logging.KeyApp, "unknown cache storage provider '%s', using sqlite", provider)
		storage, err = newSQLiteStorage(storagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize cache storage: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "cache storage initialized: %s", provider)
	return nil
}

// Get retrieves data by key
func Get(key string) ([]byte, error) {
	return storage.Get(key)
}

// Set stores data with key
func Set(key string, data []byte) error {
	return storage.Set(key, data)
}

// Delete removes data by key
func Delete(key string) error {
	return storage.Delete(key)
}

// List returns all keys with given prefix
func List(prefix string) ([]string, error) {
	return storage.List(prefix)
}

// Exists checks if key exists
func Exists(key string) bool {
	return storage.Exists(key)
}

// Flush removes all cache entries
func Flush() error {
	return storage.Flush()
}
