// Package cacheStorage provides cache storage functionality
package cacheStorage

import (
	"fmt"

	"knov/internal/logging"
)

// CacheStorage interface defines methods for cache storage
type CacheStorage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	Exists(key string) bool
	GetBackendType() string
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
		logging.LogWarning("unknown cache storage provider '%s', using sqlite", provider)
		storage, err = newSQLiteStorage(storagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize cache storage: %w", err)
	}

	logging.LogInfo("cache storage initialized: %s", provider)
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

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}
