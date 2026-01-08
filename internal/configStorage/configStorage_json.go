// Package configStorage - JSON backend implementation
package configStorage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/logging"
)

// jsonStorage implements ConfigStorage interface using JSON files
type jsonStorage struct {
	basePath string
	mutex    sync.RWMutex
}

// newJSONStorage creates a new JSON config storage instance
func newJSONStorage(storagePath string) (*jsonStorage, error) {
	fullPath := filepath.Join(storagePath, "config")
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	return &jsonStorage{
		basePath: fullPath,
	}, nil
}

// Get retrieves config value by key
func (js *jsonStorage) Get(key string) ([]byte, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logging.LogError("failed to read config file %s: %v", filePath, err)
		return nil, err
	}

	logging.LogDebug("retrieved config for key: %s", key)
	return data, nil
}

// Set stores config value with key
func (js *jsonStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create config directory %s: %v", dir, err)
		return err
	}

	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		var temp interface{}
		if err := json.Unmarshal(data, &temp); err != nil {
			logging.LogWarning("config for key %s is not valid json: %v", key, err)
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logging.LogError("failed to write config file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("stored config for key: %s", key)
	return nil
}

// Delete removes config value by key
func (js *jsonStorage) Delete(key string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logging.LogError("failed to delete config file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("deleted config for key: %s", key)
	return nil
}

// GetAll returns all config key-value pairs
func (js *jsonStorage) GetAll() (map[string][]byte, error) {
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
				logging.LogWarning("failed to read config file %s: %v", path, err)
				return nil
			}

			result[key] = data
		}
		return nil
	})

	if err != nil {
		logging.LogError("failed to get all config: %v", err)
		return nil, err
	}

	logging.LogDebug("retrieved %d config entries", len(result))
	return result, nil
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
		logging.LogError("failed to list config keys with prefix %s: %v", prefix, err)
		return nil, err
	}

	return keys, nil
}

// Exists checks if config key exists
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
	return filepath.Join(js.basePath, key+".json")
}

// pathToKey converts a file path to a key
func (js *jsonStorage) pathToKey(relPath string) string {
	key := strings.TrimSuffix(relPath, ".json")
	return filepath.ToSlash(key)
}
