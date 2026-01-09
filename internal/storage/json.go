// Package storage ..
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/logging"
)

// JSONStorage implements Storage interface using JSON files
type JSONStorage struct {
	basePath    string
	storageType string
	mutex       sync.RWMutex
}

// NewJSONStorage creates a new JSON storage instance
func NewJSONStorage(basePath, storageType string) (*JSONStorage, error) {
	fullPath := filepath.Join(basePath, storageType)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	return &JSONStorage{
		basePath:    fullPath,
		storageType: storageType,
	}, nil
}

// Get retrieves data by key
func (js *JSONStorage) Get(key string) ([]byte, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logging.LogError("failed to read file %s: %v", filePath, err)
		return nil, err
	}

	logging.LogDebug("retrieved data for key: %s", key)
	return data, nil
}

// Set stores data with key
func (js *JSONStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		return err
	}

	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		var temp interface{}
		if err := json.Unmarshal(data, &temp); err != nil {
			logging.LogWarning("data for key %s is not valid json: %v", key, err)
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logging.LogError("failed to write file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("stored data for key: %s", key)
	return nil
}

// Delete removes data by key
func (js *JSONStorage) Delete(key string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logging.LogError("failed to delete file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("deleted data for key: %s", key)
	return nil
}

// List returns all keys with given prefix
func (js *JSONStorage) List(prefix string) ([]string, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	var keys []string

	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
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
		logging.LogError("failed to list keys with prefix %s: %v", prefix, err)
		return nil, err
	}

	logging.LogDebug("listed %d keys with prefix: %s", len(keys), prefix)
	return keys, nil
}

// GetAll returns all key-value pairs
func (js *JSONStorage) GetAll() (map[string][]byte, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	result := make(map[string][]byte)

	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			relPath, err := filepath.Rel(js.basePath, path)
			if err != nil {
				return err
			}

			key := js.pathToKey(relPath)
			data, err := os.ReadFile(path)
			if err != nil {
				logging.LogWarning("failed to read file %s: %v", path, err)
				return nil
			}
			result[key] = data
		}
		return nil
	})

	if err != nil {
		logging.LogError("failed to get all keys: %v", err)
		return nil, err
	}

	logging.LogDebug("retrieved %d key-value pairs", len(result))
	return result, nil
}

// Exists checks if key exists
func (js *JSONStorage) Exists(key string) bool {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)
	_, err := os.Stat(filePath)
	return err == nil
}

// GetBackendType returns the backend type
func (js *JSONStorage) GetBackendType() string {
	return "json"
}

// getFilePath converts key to file path
func (js *JSONStorage) getFilePath(key string) string {
	return filepath.Join(js.basePath, key+".json")
}

// pathToKey converts file path back to key
func (js *JSONStorage) pathToKey(path string) string {
	return strings.TrimSuffix(path, ".json")
}
