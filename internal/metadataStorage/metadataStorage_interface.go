// Package metadataStorage provides metadata storage functionality
package metadataStorage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/backup"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/logging"
)

const markerKey = "metadata-backend"

// MetadataStorage interface defines methods for metadata storage
type MetadataStorage interface {
	Get(key string) ([]byte, error)
	// Set stores data, a JSON-encoded files.Metadata - the only real caller is
	// files.metaDataSaveRaw, which always json.Marshal's a Metadata struct first. A structured
	// backend (sqlite today, e.g. Postgres later) may parse known fields out of data rather than
	// storing it verbatim, so Get is not guaranteed to return the exact bytes passed to Set.
	Set(key string, data []byte) error
	Delete(key string) error
	GetAll() (map[string][]byte, error)
	Exists(key string) bool
	GetBackendType() string
	// Cleanup removes all data managed by this backend.
	// Called once after a successful migration to a new backend.
	Cleanup() error
	Backup(destDir string) error
	Restore(srcDir string) error
}

var storage MetadataStorage

func init() {
	backup.Register("metadata", backupAdapter{})
}

// backupAdapter defers to the package-level storage var, which isn't set until Init runs -
// unlike backup.Register, which happens at package init time before that.
type backupAdapter struct{}

func (backupAdapter) Backup(destDir string) error { return storage.Backup(destDir) }
func (backupAdapter) Restore(srcDir string) error { return storage.Restore(srcDir) }

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

// errFound aborts a filepath.Walk early once a match is seen, without treating it as a real error.
var errFound = errors.New("found")

// hasJSONMetadataFiles reports whether the json backend's directory holds at least one
// per-key .json file.
func hasJSONMetadataFiles(storagePath string) bool {
	base := filepath.Join(storagePath, "metadata")
	if _, err := os.Stat(base); err != nil {
		return false
	}
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			return errFound
		}
		return nil
	})
	return errors.Is(err, errFound)
}

// hasYAMLFrontMatter reports whether any docs file already carries YAML front matter.
func hasYAMLFrontMatter() bool {
	docsPath := filepath.Join(configmanager.GetAppConfig().DataPath, "docs")
	if _, err := os.Stat(docsPath); err != nil {
		return false
	}
	err := filepath.Walk(docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.HasPrefix(content, frontMatterDelimiter) {
			return errFound
		}
		return nil
	})
	return errors.Is(err, errFound)
}

// detectLegacyProvider guesses which provider was active before the marker itself existed, by
// checking which backend's data is actually present on disk. Needed because installs that
// predate this migration feature never wrote a marker, even though they had real data - without
// this, that data would silently be left behind on the first upgrade to a different provider.
// Checked in a fixed order (sqlite, json, yaml) so only one legacy backend is ever assumed even
// if more than one happens to have leftover data. Returns "" for a genuinely fresh install.
func detectLegacyProvider(provider, storagePath string) string {
	if provider != "sqlite" {
		if _, err := os.Stat(filepath.Join(storagePath, "metadata", metadataDBFile)); err == nil {
			return "sqlite"
		}
	}
	if provider != "json" && hasJSONMetadataFiles(storagePath) {
		return "json"
	}
	if provider != "yaml" && hasYAMLFrontMatter() {
		return "yaml"
	}
	return ""
}

// checkMetadataMigration detects whether a migration is needed.
// detected reports whether previous came from detectLegacyProvider (no marker existed yet)
// rather than from a recorded marker, so Init can log why a migration was triggered.
func checkMetadataMigration(provider, storagePath string) (needsMigration bool, previous string, detected bool) {
	previous = readMarker()
	if previous == "" {
		previous = detectLegacyProvider(provider, storagePath)
		detected = previous != ""
	}
	if previous == "" || previous == provider {
		return false, previous, detected
	}
	return true, previous, detected
}

// migrate copies all entries from src to dst, then calls src.Cleanup().
// Every step is logged to logs/database-migration.log.
func migrate(src, dst MetadataStorage) error {
	all, err := src.GetAll()
	if err != nil {
		return fmt.Errorf("failed to read source storage: %w", err)
	}

	logging.LogInfo(logging.KeyDBMigration, "metadata: starting migration: %s -> %s (%d entries)", src.GetBackendType(), dst.GetBackendType(), len(all))

	var written, failed int
	for key, data := range all {
		if err := dst.Set(key, data); err != nil {
			logging.LogWarning(logging.KeyDBMigration, "metadata: error writing %s: %v", key, err)
			failed++
		} else {
			logging.LogDebug(logging.KeyDBMigration, "metadata: migrated %s", key)
			written++
		}
	}

	if failed > 0 {
		logging.LogWarning(logging.KeyDBMigration, "metadata: migration had %d write errors — skipping cleanup to preserve source data", failed)
		return fmt.Errorf("migration completed with %d write errors (see logs/database-migration.log)", failed)
	}

	logging.LogInfo(logging.KeyDBMigration, "metadata: cleaning up old backend (%s)", src.GetBackendType())
	if err := src.Cleanup(); err != nil {
		logging.LogWarning(logging.KeyDBMigration, "metadata: cleanup of old backend failed: %v", err)
	}

	logging.LogInfo(logging.KeyDBMigration, "metadata: migration complete: %d entries migrated", written)
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

	needsMigration, previous, detected := checkMetadataMigration(provider, storagePath)

	if needsMigration {
		if detected {
			logging.LogInfo(logging.KeyDBMigration, "metadata: detected existing %s storage on disk (no migration marker found), migrating to %s", previous, provider)
		}
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
