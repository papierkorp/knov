// Package storage ..
package storage

import (
	"knov/internal/logging"
)

var globalStorageManager *StorageManager
var globalConfigStorageManager *StorageManager

// Storage interface defines methods for storing and retrieving data
type Storage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	Exists(key string) bool
}

// StorageManager manages storage backend
type StorageManager struct {
	backend Storage
}

// Init initializes the global storage manager
func Init(storageMethod, configPath string) error {
	var backend Storage
	var err error

	switch storageMethod {
	case "json":
		backend, err = NewJSONStorage()
	case "sqlite":
		logging.LogError("sqlite storage not implemented yet, using json")
		backend, err = NewJSONStorage()
	case "postgres":
		logging.LogError("postgres storage not implemented yet, using json")
		backend, err = NewJSONStorage()
	default:
		logging.LogWarning("unknown storage type '%s', using json", storageMethod)
		backend, err = NewJSONStorage()
	}

	if err != nil {
		logging.LogError("failed to initialize storage: %v", err)
		return err
	}

	globalStorageManager = &StorageManager{backend: backend}
	logging.LogInfo("storage initialized: %s", storageMethod)

	// Initialize config storage
	var configBackend Storage
	switch storageMethod {
	case "json":
		configBackend, err = NewConfigJSONStorage(configPath)
	default:
		configBackend, err = NewConfigJSONStorage(configPath)
	}

	if err != nil {
		logging.LogError("failed to initialize config storage: %v", err)
		return err
	}

	globalConfigStorageManager = &StorageManager{backend: configBackend}
	logging.LogInfo("config storage initialized: %s", storageMethod)

	return nil
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

// Exists checks if key exists
func (sm *StorageManager) Exists(key string) bool {
	return sm.backend.Exists(key)
}

// GetStorage returns the global storage manager
func GetStorage() *StorageManager {
	return globalStorageManager
}

// GetConfigStorage returns the global config storage manager
func GetConfigStorage() *StorageManager {
	return globalConfigStorageManager
}
