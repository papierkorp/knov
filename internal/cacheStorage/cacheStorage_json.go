// Package cacheStorage - JSON backend implementation
package cacheStorage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/logging"
)

// jsonStorage implements CacheStorage interface using JSON files
type jsonStorage struct {
	basePath string
	mutex    sync.RWMutex
}

// newJSONStorage creates a new JSON cache storage instance
func newJSONStorage(storagePath string) (*jsonStorage, error) {
	fullPath := filepath.Join(storagePath, "cache")
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	return &jsonStorage{
		basePath: fullPath,
	}, nil
}

// Get retrieves data by key
func (js *jsonStorage) Get(key string) ([]byte, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logging.LogError("failed to read cache file %s: %v", filePath, err)
		return nil, err
	}

	logging.LogDebug("retrieved cache data for key: %s", key)
	return data, nil
}

// Set stores data with key
func (js *jsonStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create cache directory %s: %v", dir, err)
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logging.LogError("failed to write cache file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("stored cache data for key: %s", key)
	return nil
}

// Delete removes data by key
func (js *jsonStorage) Delete(key string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logging.LogError("failed to delete cache file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("deleted cache data for key: %s", key)
	return nil
}

// List returns all keys with given prefix
func (js *jsonStorage) List(prefix string) ([]string, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	var keys []string

	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(js.basePath, path)
			if err != nil {
				return err
			}

			key := js.pathToKey(relPath)
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		}
		return nil
	})

	if err != nil {
		logging.LogError("failed to list cache keys with prefix %s: %v", prefix, err)
		return nil, err
	}

	return keys, nil
}

// Exists checks if key exists
func (js *jsonStorage) Exists(key string) bool {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// GetBackendType returns the backend type
func (js *jsonStorage) GetBackendType() string {
	return "json"
}

// getFilePath converts a key to a file path
func (js *jsonStorage) getFilePath(key string) string {
	// replace path separators with underscores for file name
	fileName := strings.ReplaceAll(key, "/", "_")
	return filepath.Join(js.basePath, fileName)
}

// pathToKey converts a file path to a key
func (js *jsonStorage) pathToKey(relPath string) string {
	// convert underscores back to path separators
	key := strings.ReplaceAll(relPath, "_", "/")
	return key
}
