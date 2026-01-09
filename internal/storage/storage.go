// Package storage provides storage abstraction layer
package storage

import (
	"fmt"

	"knov/internal/logging"
)

// Storage interface defines methods for storing and retrieving data
type Storage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	GetAll() (map[string][]byte, error)
	Exists(key string) bool
	GetBackendType() string
}

// StorageManager manages a storage backend
type StorageManager struct {
	backend Storage
}

var (
	configStorage   *StorageManager
	metadataStorage *StorageManager
)

// InitStorages initializes all storage managers
func InitStorages(configProvider, metadataProvider, cacheProvider, storagePath string) error {
	var err error

	// initialize config storage
	configStorage, err = newStorageManager(configProvider, "config", storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize config storage: %w", err)
	}
	logging.LogInfo("config storage initialized: %s", configProvider)

	// initialize metadata storage
	metadataStorage, err = newStorageManager(metadataProvider, "metadata", storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize metadata storage: %w", err)
	}
	logging.LogInfo("metadata storage initialized: %s", metadataProvider)

	// Note: cache storage is now handled by cacheStorage package
	return nil
}

// newStorageManager creates a storage manager for a specific provider
func newStorageManager(provider, storageType, basePath string) (*StorageManager, error) {
	var backend Storage
	var err error

	switch provider {
	case "json":
		backend, err = NewJSONStorage(basePath, storageType)
	case "sqlite":
		backend, err = NewSQLiteStorage(basePath, storageType)
	default:
		logging.LogWarning("unknown storage provider '%s', using json", provider)
		backend, err = NewJSONStorage(basePath, storageType)
	}

	if err != nil {
		return nil, err
	}

	return &StorageManager{backend: backend}, nil
}

// Get retrieves data by key
func (sm *StorageManager) Get(key string) ([]byte, error) {
	return sm.backend.Get(key)
}

// Set stores data with key
func (sm *StorageManager) Set(key string, data []byte) error {
	return sm.backend.Set(key, data)
}

// Delete removes data by key
func (sm *StorageManager) Delete(key string) error {
	return sm.backend.Delete(key)
}

// List returns all keys with given prefix
func (sm *StorageManager) List(prefix string) ([]string, error) {
	return sm.backend.List(prefix)
}

// GetAll returns all key-value pairs
func (sm *StorageManager) GetAll() (map[string][]byte, error) {
	return sm.backend.GetAll()
}

// Exists checks if key exists
func (sm *StorageManager) Exists(key string) bool {
	return sm.backend.Exists(key)
}

// GetBackendType returns the backend type
func (sm *StorageManager) GetBackendType() string {
	return sm.backend.GetBackendType()
}

// GetConfigStorage returns the config storage manager
func GetConfigStorage() *StorageManager {
	return configStorage
}

// GetMetadataStorage returns the metadata storage manager
func GetMetadataStorage() *StorageManager {
	return metadataStorage
}
