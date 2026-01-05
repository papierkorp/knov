// Package storage ..
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/logging"
)

// ----------------------------------------------------------------------------------------
// -------------------------------- Base JSON Storage -------------------------------------
// ----------------------------------------------------------------------------------------

// baseJSONStorage provides common functionality for all JSON storage types
type baseJSONStorage struct {
	basePath string
	mutex    sync.RWMutex
}

// Get retrieves data by key
func (js *baseJSONStorage) Get(key string) ([]byte, error) {
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
func (js *baseJSONStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logging.LogError("failed to write file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug("stored data for key: %s", key)
	return nil
}

// Delete removes data by key
func (js *baseJSONStorage) Delete(key string) error {
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
func (js *baseJSONStorage) List(prefix string) ([]string, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	var keys []string

	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		relPath, err := filepath.Rel(js.basePath, path)
		if err != nil {
			return err
		}

		key := strings.TrimSuffix(relPath, ".json")
		key = strings.ReplaceAll(key, string(filepath.Separator), "/")

		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			logging.LogDebug("base path does not exist: %s", js.basePath)
			return []string{}, nil
		}
		logging.LogError("failed to list keys with prefix %s: %v", prefix, err)
		return nil, err
	}

	logging.LogDebug("listed %d keys with prefix: %s", len(keys), prefix)
	return keys, nil
}

// Exists checks if key exists
func (js *baseJSONStorage) Exists(key string) bool {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)
	_, err := os.Stat(filePath)
	return err == nil
}

// Close closes the storage (no-op for JSON)
func (js *baseJSONStorage) Close() error {
	return nil
}

// GetWithDefault retrieves data or returns default if not found
func (js *baseJSONStorage) GetWithDefault(key string, defaultValue []byte) ([]byte, error) {
	data, err := js.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return defaultValue, nil
	}
	return data, nil
}

// BulkSet stores multiple key-value pairs
func (js *baseJSONStorage) BulkSet(data map[string][]byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	for key, value := range data {
		filePath := js.getFilePath(key)
		dir := filepath.Dir(filePath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			logging.LogError("failed to create directory %s: %v", dir, err)
			return err
		}

		if err := os.WriteFile(filePath, value, 0644); err != nil {
			logging.LogError("failed to write file %s: %v", filePath, err)
			return err
		}
	}

	logging.LogDebug("bulk set %d keys", len(data))
	return nil
}

// getFilePath returns the file path for a key
func (js *baseJSONStorage) getFilePath(key string) string {
	key = strings.ReplaceAll(key, "/", string(filepath.Separator))
	return filepath.Join(js.basePath, key+".json")
}
