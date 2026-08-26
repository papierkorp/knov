// Package kanbanStorage provides persistence for kanban event logs.
package kanbanStorage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/configStorage"
	"knov/internal/logging"
)

// markerKey persists the last active backend name in configStorage, so a provider change
// between app runs can be detected and migrated automatically.
const markerKey = "kanban-events-backend"

// Event represents a single kanban card move.
type Event struct {
	FilePath    string    `json:"filePath"`
	BoardFolder string    `json:"boardFolder"`
	FromStatus  string    `json:"fromStatus"`
	ToStatus    string    `json:"toStatus"`
	Timestamp   time.Time `json:"timestamp"`
}

// KanbanStorage defines the interface for kanban event persistence.
type KanbanStorage interface {
	LogEvent(filePath, boardFolder, fromStatus, toStatus string) error
	GetEvents(boardFolder, filePath string, from, to *time.Time, limit int) ([]Event, error)
	// insertEvents bulk-inserts events verbatim, preserving their original timestamps. Used
	// only by the provider-migration path below, since LogEvent always stamps time.Now().
	insertEvents(events []Event) error
	GetBackendType() string
	// Cleanup removes all data managed by this backend. Called once after a successful
	// migration to a new backend.
	Cleanup() error
	Backup(destDir string) error
	Restore(srcDir string) error
}

var storage KanbanStorage

func init() {
	backup.Register("kanban", backupAdapter{})
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
		logging.LogWarning(logging.KeyApp, "kanban migration: failed to write backend marker: %v", err)
	}
}

// newBackend creates a KanbanStorage instance for the given provider.
func newBackend(provider, storagePath string) (KanbanStorage, error) {
	switch provider {
	case "sqlite":
		return newSQLiteStorage(storagePath)
	case "json":
		return newJSONStorage(storagePath)
	default:
		return nil, fmt.Errorf("unknown kanban storage provider: %s", provider)
	}
}

// detectLegacyProvider guesses which provider was active before the marker itself existed, by
// checking which backend's data file is actually present on disk. Needed because installs that
// predate this migration feature never wrote a marker, even though they had real sqlite (or
// json) data - without this, that data would silently be left behind on the first upgrade.
// Returns "" when neither file is present (a genuinely fresh install).
func detectLegacyProvider(provider, storagePath string) string {
	if provider != "sqlite" {
		if _, err := os.Stat(filepath.Join(storagePath, "kanban", kanbanDBFile)); err == nil {
			return "sqlite"
		}
	}
	if provider != "json" {
		if _, err := os.Stat(filepath.Join(storagePath, "kanban", kanbanEventsFile)); err == nil {
			return "json"
		}
	}
	return ""
}

// checkMigration detects whether the configured provider differs from the last active one.
// detected reports whether previous came from detectLegacyProvider (no marker existed yet)
// rather than from a recorded marker, so Init can log why a migration was triggered.
func checkMigration(provider, storagePath string) (needsMigration bool, previous string, detected bool) {
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

// migrate copies every event from src to dst, then calls src.Cleanup().
// Every step is logged to logs/database-migration.log.
func migrate(src, dst KanbanStorage) error {
	events, err := src.GetEvents("", "", nil, nil, 0)
	if err != nil {
		return fmt.Errorf("failed to read source storage: %w", err)
	}

	logging.LogInfo(logging.KeyDBMigration, "kanban: starting migration: %s -> %s (%d events)", src.GetBackendType(), dst.GetBackendType(), len(events))

	if err := dst.insertEvents(events); err != nil {
		logging.LogError(logging.KeyDBMigration, "kanban: migration failed: %v", err)
		return fmt.Errorf("kanban migration failed: %w", err)
	}

	logging.LogInfo(logging.KeyDBMigration, "kanban: cleaning up old backend (%s)", src.GetBackendType())
	if err := src.Cleanup(); err != nil {
		logging.LogWarning(logging.KeyDBMigration, "kanban: cleanup of old backend failed: %v", err)
	}

	logging.LogInfo(logging.KeyDBMigration, "kanban: migration complete: %d events migrated", len(events))
	return nil
}

// Init initializes kanban event storage.
// If enabled is false the noop backend is used regardless of provider.
// If a different provider was previously active, all events are migrated automatically.
func Init(enabled bool, provider, storagePath string) error {
	if !enabled {
		storage = &noopStorage{}
		logging.LogInfo(logging.KeyApp, "kanban storage disabled")
		return nil
	}

	switch provider {
	case "sqlite", "json":
	default:
		logging.LogWarning(logging.KeyApp, "unknown kanban storage provider '%s', using sqlite", provider)
		provider = "sqlite"
	}

	needsMigration, previous, detected := checkMigration(provider, storagePath)

	if needsMigration {
		if detected {
			logging.LogInfo(logging.KeyDBMigration, "kanban: detected existing %s storage on disk (no migration marker found), migrating to %s", previous, provider)
		}
		logging.LogInfo(logging.KeyApp, "kanban storage provider changed: %s -> %s, running migration", previous, provider)

		oldBackend, err := newBackend(previous, storagePath)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "kanban migration: could not open old backend %s: %v", previous, err)
		} else {
			newB, err := newBackend(provider, storagePath)
			if err != nil {
				return fmt.Errorf("failed to initialize new kanban storage %s: %w", provider, err)
			}
			if err := migrate(oldBackend, newB); err != nil {
				return fmt.Errorf("kanban migration failed: %w", err)
			}
			storage = newB
			writeMarker(provider)
			logging.LogInfo(logging.KeyApp, "kanban storage initialized after migration: %s", provider)
			return nil
		}
	}

	s, err := newBackend(provider, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize kanban storage: %w", err)
	}
	storage = s
	writeMarker(provider)
	logging.LogInfo(logging.KeyApp, "kanban storage initialized: %s", provider)
	return nil
}

// LogEvent records a kanban card move.
func LogEvent(filePath, boardFolder, fromStatus, toStatus string) error {
	return storage.LogEvent(filePath, boardFolder, fromStatus, toStatus)
}

// GetEvents retrieves kanban move events with optional filters, newest first.
// Pass empty strings / nil times to skip those filters; limit=0 means no limit.
func GetEvents(boardFolder, filePath string, from, to *time.Time, limit int) ([]Event, error) {
	return storage.GetEvents(boardFolder, filePath, from, to, limit)
}
