// Package configStorage provides configuration storage functionality
package configStorage

import (
	"fmt"

	"knov/internal/backup"
	"knov/internal/logging"
)

// ConfigStorage interface defines methods for configuration storage
type ConfigStorage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	GetAll() (map[string][]byte, error)
	List(prefix string) ([]string, error)
	Exists(key string) bool
	GetBackendType() string
	Backup(destDir string) error
	Restore(srcDir string) error
}

var storage ConfigStorage

func init() {
	backup.Register("config", backupAdapter{})
}

// backupAdapter defers to the package-level storage var, which isn't set until Init runs -
// unlike backup.Register, which happens at package init time before that.
type backupAdapter struct{}

func (backupAdapter) Backup(destDir string) error { return storage.Backup(destDir) }
func (backupAdapter) Restore(srcDir string) error { return storage.Restore(srcDir) }
func (backupAdapter) GetBackendType() string      { return storage.GetBackendType() }

// Init initializes config storage with the specified provider
func Init(provider, storagePath string) error {
	var err error

	switch provider {
	case "json":
		storage, err = newJSONStorage(storagePath)
	default:
		logging.LogWarning(logging.KeyApp, "unknown config storage provider '%s', using json", provider)
		storage, err = newJSONStorage(storagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize config storage: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "config storage initialized: %s", provider)
	return nil
}

// Get retrieves config value by key
func Get(key string) ([]byte, error) {
	return storage.Get(key)
}

// Set stores config value with key
func Set(key string, data []byte) error {
	return storage.Set(key, data)
}

// Delete removes config value by key
func Delete(key string) error {
	return storage.Delete(key)
}

// GetAll returns all config key-value pairs
func GetAll() (map[string][]byte, error) {
	return storage.GetAll()
}

// List returns all keys with given prefix
func List(prefix string) ([]string, error) {
	return storage.List(prefix)
}

// Exists checks if config key exists
func Exists(key string) bool {
	return storage.Exists(key)
}

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}
