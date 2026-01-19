// Package metadataStorage provides metadata storage functionality
package metadataStorage

import (
	"fmt"

	"knov/internal/logging"
)

// MetadataStorage interface defines methods for metadata storage
type MetadataStorage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	GetAll() (map[string][]byte, error)
	Exists(key string) bool
	GetBackendType() string
}

var storage MetadataStorage

// Init initializes metadata storage with the specified provider
func Init(provider, storagePath string) error {
	var err error

	switch provider {
	case "json":
		storage, err = newJSONStorage(storagePath)
	case "sqlite":
		storage, err = newSQLiteStorage(storagePath)
	default:
		logging.LogWarning("unknown metadata storage provider '%s', using json", provider)
		storage, err = newJSONStorage(storagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize metadata storage: %w", err)
	}

	logging.LogInfo("metadata storage initialized: %s", provider)
	return nil
}

// Get retrieves metadata by key
func Get(key string) ([]byte, error) {
	return storage.Get(key)
}

// Set stores metadata with key
func Set(key string, data []byte) error {
	return storage.Set(key, data)
}

// Delete removes metadata by key
func Delete(key string) error {
	return storage.Delete(key)
}

// GetAll returns all metadata key-value pairs
func GetAll() (map[string][]byte, error) {
	return storage.GetAll()
}

// Exists checks if metadata key exists
func Exists(key string) bool {
	return storage.Exists(key)
}

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}
