// Package metadataStorage - JSON backend implementation
package metadataStorage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/backup"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

// jsonStorage implements MetadataStorage interface using JSON files
type jsonStorage struct {
	basePath string
	mutex    sync.RWMutex
}

// newJSONStorage creates a new JSON metadata storage instance under storagePath/metadata.
func newJSONStorage(storagePath string) (*jsonStorage, error) {
	return newJSONStorageAt(filepath.Join(storagePath, "metadata"))
}

// newJSONStorageAt creates a new JSON metadata storage instance rooted directly at dir, without
// joining on a "metadata" subfolder. Used to open a backup already extracted to its own leaf
// directory (see restoreMigrate), instead of relying on that directory happening to be named
// "metadata".
func newJSONStorageAt(dir string) (*jsonStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &jsonStorage{
		basePath: dir,
	}, nil
}

// Get retrieves metadata by key
func (js *jsonStorage) Get(key string) ([]byte, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	filePath := js.getFilePath(key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logging.LogError(logging.KeyApp, "failed to read metadata file %s: %v", filePath, err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved metadata for key: %s", key)
	return data, nil
}

// Set stores metadata with key
func (js *jsonStorage) Set(key string, data []byte) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "failed to create metadata directory %s: %v", dir, err)
		return err
	}

	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		var temp interface{}
		if err := json.Unmarshal(data, &temp); err != nil {
			logging.LogWarning(logging.KeyApp, "metadata for key %s is not valid json: %v", key, err)
		}
	}

	if err := utils.WriteFileAtomic(filePath, data, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write metadata file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "stored metadata for key: %s", key)
	return nil
}

// Delete removes metadata by key
func (js *jsonStorage) Delete(key string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	filePath := js.getFilePath(key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logging.LogError(logging.KeyApp, "failed to delete metadata file %s: %v", filePath, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "deleted metadata for key: %s", key)
	return nil
}

// GetAll returns all metadata key-value pairs
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
				logging.LogWarning(logging.KeyApp, "failed to read metadata file %s: %v", path, err)
				return nil
			}

			result[key] = data
		}
		return nil
	})

	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get all metadata: %v", err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved %d metadata entries", len(result))
	return result, nil
}

// Exists checks if metadata key exists
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

// Close is a no-op - the json backend holds no persistent file handle to release.
func (js *jsonStorage) Close() error {
	return nil
}

// getFilePath converts a key to a file path
func (js *jsonStorage) getFilePath(key string) string {
	return filepath.Join(js.basePath, key+".json")
}

// pathToKey converts a file path to a key
func (js *jsonStorage) pathToKey(relPath string) string {
	key := strings.TrimSuffix(relPath, ".json")
	return pathutils.ToSlash(key)
}

// Backup snapshots every metadata file into destDir.
func (js *jsonStorage) Backup(destDir string) error {
	js.mutex.RLock()
	defer js.mutex.RUnlock()
	return backup.BackupFile(js.basePath, destDir)
}

// Restore overwrites the metadata directory with a previously backed-up snapshot from srcDir.
func (js *jsonStorage) Restore(srcDir string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()
	return backup.RestoreFile(srcDir, js.basePath)
}

// Cleanup removes the entire json metadata folder
func (js *jsonStorage) Cleanup() error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	// only the json files and the folders they leave empty: the sqlite backend keeps its
	// metadata.db in this same folder, and a migration to it calls this after writing it
	var dirs []string
	err := filepath.Walk(js.basePath, func(path string, info os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case info.IsDir():
			dirs = append(dirs, path)
		case strings.HasSuffix(path, ".json"):
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		logging.LogError(logging.KeyApp, "json metadata cleanup: failed to remove files in %s: %v", js.basePath, err)
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // fails for a folder that still holds something, which stays
	}

	logging.LogInfo(logging.KeyApp, "json metadata cleanup: removed the json files in %s", js.basePath)
	return nil
}
