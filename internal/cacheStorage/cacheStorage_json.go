// Package cacheStorage - JSON backend implementation
package cacheStorage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/logging"
	"knov/internal/utils"
)

// jsonStorage implements CacheStorage interface using JSON files
type jsonStorage struct {
	basePath string
	mutex    sync.RWMutex
}

// newJSONStorage creates a new JSON cache storage instance under storagePath/cache.
func newJSONStorage(storagePath string) (*jsonStorage, error) {
	basePath := filepath.Join(storagePath, "cache")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &jsonStorage{
		basePath: basePath,
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
		logging.LogError(logging.KeyApp, "failed to read cache file %s: %v", filePath, err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved cache data for key: %s", key)
	return data, nil
}

// Set stores data with key
func (js *jsonStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "failed to create cache directory %s: %v", dir, err)
		return err
	}

	if err := utils.WriteFileAtomic(filePath, data, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write cache file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "stored cache data for key: %s", key)
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
		logging.LogError(logging.KeyApp, "failed to delete cache file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "deleted cache data for key: %s", key)
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
		logging.LogError(logging.KeyApp, "failed to list cache keys with prefix %s: %v", prefix, err)
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

// Flush removes all cache entries
func (js *jsonStorage) Flush() error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		return os.Remove(path)
	})
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to flush cache: %v", err)
		return err
	}

	logging.LogInfo(logging.KeyApp, "cache flushed")
	return nil
}
