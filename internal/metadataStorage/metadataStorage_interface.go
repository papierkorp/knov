// Package metadataStorage provides metadata storage functionality
package metadataStorage

import (
	"fmt"
	"strings"

	"knov/internal/configStorage"
	"knov/internal/logging"
)

const markerKey = "metadata-backend"

// MetadataStorage interface defines methods for metadata storage
type MetadataStorage interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	GetAll() (map[string][]byte, error)
	Exists(key string) bool
	GetBackendType() string
	// Cleanup removes all data managed by this backend.
	// Called once after a successful migration to a new backend.
	Cleanup() error
}

var storage MetadataStorage

// readMarker returns the previously active backend name from configStorage, or "".
func readMarker() string {
	data, err := configStorage.Get(markerKey)
	if err != nil || data == nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeMarker persists the active backend name to configStorage.
func writeMarker(provider string) {
	if err := configStorage.Set(markerKey, []byte(provider)); err != nil {
		logging.LogWarning(logging.KeyApp, "metadata migration: failed to write backend marker: %v", err)
	}
}

// newBackend creates a MetadataStorage instance for the given provider.
func newBackend(provider, storagePath string) (MetadataStorage, error) {
	switch provider {
	case "json":
		return newJSONStorage(storagePath)
	case "yaml":
		return newYAMLStorage(storagePath)
	case "sqlite":
		return newSQLiteStorage(storagePath)
	default:
		return nil, fmt.Errorf("unknown metadata storage provider: %s", provider)
	}
}

// checkMetadataMigration detects whether a migration is needed.
// Returns (true, previousProvider) when the configured provider differs from the last active one.
func checkMetadataMigration(provider string) (bool, string) {
	previous := readMarker()
	if previous == "" || previous == provider {
		return false, previous
	}
	return true, previous
}

// migrate copies all entries from src to dst, then calls src.Cleanup().
// Every step is logged to logs/metadata-migration.log.
func migrate(src, dst MetadataStorage) error {
	all, err := src.GetAll()
	if err != nil {
		return fmt.Errorf("failed to read source storage: %w", err)
	}

	logging.LogInfo(logging.KeyMetaMigration, "starting migration: %s -> %s (%d entries)", src.GetBackendType(), dst.GetBackendType(), len(all))

	var written, failed int
	for key, data := range all {
		if err := dst.Set(key, data); err != nil {
			logging.LogWarning(logging.KeyMetaMigration, "error writing %s: %v", key, err)
			failed++
		} else {
			logging.LogDebug(logging.KeyMetaMigration, "migrated %s", key)
			written++
		}
	}

	if failed > 0 {
		logging.LogWarning(logging.KeyMetaMigration, "migration had %d write errors — skipping cleanup to preserve source data", failed)
		return fmt.Errorf("migration completed with %d write errors (see logs/metadata-migration.log)", failed)
	}

	logging.LogInfo(logging.KeyMetaMigration, "cleaning up old backend (%s)", src.GetBackendType())
	if err := src.Cleanup(); err != nil {
		logging.LogWarning(logging.KeyMetaMigration, "cleanup of old backend failed: %v", err)
	}

	logging.LogInfo(logging.KeyMetaMigration, "migration complete: %d entries migrated", written)
	return nil
}

// Init initializes metadata storage with the specified provider.
// If a different provider was previously active, all metadata is migrated automatically.
func Init(provider, storagePath string) error {
	switch provider {
	case "json", "yaml", "sqlite":
	default:
		logging.LogWarning(logging.KeyApp, "unknown metadata storage provider '%s', using json", provider)
		provider = "json"
	}

	needsMigration, previous := checkMetadataMigration(provider)

	if needsMigration {
		logging.LogInfo(logging.KeyApp, "metadata storage provider changed: %s -> %s, running migration", previous, provider)

		oldBackend, err := newBackend(previous, storagePath)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "metadata migration: could not open old backend %s: %v", previous, err)
		} else {
			newB, err := newBackend(provider, storagePath)
			if err != nil {
				return fmt.Errorf("failed to initialize new metadata storage %s: %w", provider, err)
			}
			if err := migrate(oldBackend, newB); err != nil {
				return fmt.Errorf("metadata migration failed: %w", err)
			}
			storage = newB
			writeMarker(provider)
			logging.LogInfo(logging.KeyApp, "metadata storage initialized after migration: %s", provider)
			return nil
		}
	}

	var err error
	storage, err = newBackend(provider, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize metadata storage: %w", err)
	}

	writeMarker(provider)
	logging.LogInfo(logging.KeyApp, "metadata storage initialized: %s", provider)
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
