// Package kanbanStorage provides persistence for kanban event logs.
package kanbanStorage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// storageMu guards storage - restoreMigrate reassigns it at runtime (not just during Init at
// startup), so a concurrent reader must be blocked from seeing a backend that Cleanup has already
// closed/deleted but that hasn't been swapped for its replacement yet.
var storageMu sync.RWMutex

// currentStoragePath is the storagePath Init was last called with, so a mismatched-backend
// restore (see restoreMigrate) can reopen a fresh backend at the live location without guessing.
var currentStoragePath string

func init() {
	backup.Register("kanban", backupAdapter{})
}

// backupAdapter defers to the package-level storage var, which isn't set until Init runs -
// unlike backup.Register, which happens at package init time before that.
type backupAdapter struct{}

func (backupAdapter) Backup(destDir string) error {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.Backup(destDir)
}

func (backupAdapter) Restore(srcDir string) error {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.Restore(srcDir)
}

func (backupAdapter) GetBackendType() string {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.GetBackendType()
}

// RestoreMigrate restores a backup set made with a different kanban backend than the one
// currently configured, converting events into the live backend instead of applying the backup's
// raw files as-is (which would either silently no-op or wipe live data - see backup.Migratable).
func (backupAdapter) RestoreMigrate(srcDir, fromBackendType string) (bool, error) {
	return restoreMigrate(srcDir, fromBackendType)
}

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

// newBackend creates a KanbanStorage instance for the given provider, rooted at storagePath - the
// live storage root under which each backend joins its own subfolder (e.g. storagePath/kanban).
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

// newBackendAt creates a KanbanStorage instance for the given provider rooted directly at dir,
// without joining a per-backend subfolder onto it - unlike newBackend, which expects a live
// storage root. Used by restoreMigrate to open a backup already extracted to its own leaf
// directory, the same directory Storage.Restore is given (see backup.Migratable).
func newBackendAt(provider, dir string) (KanbanStorage, error) {
	switch provider {
	case "sqlite":
		return newSQLiteStorageAt(dir)
	case "json":
		return newJSONStorageAt(dir)
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

// restoreMigrate reads events out of a backup made with fromBackendType (extracted to srcDir,
// this storage's own subdirectory within the backup set - the same directory Storage.Restore is
// given), then replaces the live backend's contents with them, preserving original timestamps.
// Used only when a restored backup set's recorded backend differs from the one currently
// configured (see backup.Migratable).
//
// A "noop" fromBackendType means the backup was taken while kanban was disabled, so it has no
// events to restore - proceeding would wipe the live backend and replace it with nothing.
// Likewise if the live backend is currently "noop" (disabled), there is nothing to restore into.
// Both cases are treated as "nothing to do" rather than an error.
//
// Reuses migrate(), same as a live provider switch - oldBackend.Cleanup() is still deferred
// explicitly here too, since migrate() only cleans up its src on the success path and this
// backup copy's handle should still be released promptly (Windows) even if writing into fresh
// fails.
//
// The live backend is fully wiped (via Cleanup) and reopened fresh rather than merged into,
// matching what a same-backend Restore already does. On success, storage is reassigned to that
// fresh backend immediately - mirroring Init's own migration path (storage = newB) - so the
// package stays usable right away rather than left pointing at the backend Cleanup just closed
// and deleted; a restart is still needed afterwards, but only because other, non-Migratable
// storages in the same restore batch had their own handles closed by their plain Restore.
//
// touched is a named return, flipped exactly once - right before storage.Cleanup runs, not after
// checking whether it succeeded - since Cleanup closes the live db handle regardless of whether
// the error it returns comes from that close or from the file removal after it (see its own
// doc); by the time Cleanup returns at all, the live handle is already gone. Every return past
// that point reuses the same touched instead of hardcoding true/false, so a future failure branch
// added below can't silently under-report it the way a hand-written literal could.
func restoreMigrate(srcDir, fromBackendType string) (touched bool, err error) {
	storageMu.Lock()
	defer storageMu.Unlock()

	currentType := storage.GetBackendType()
	if fromBackendType == "noop" || currentType == "noop" {
		logging.LogInfo(logging.KeyDBMigration, "kanban: storage disabled (backup=%s, current=%s), nothing to restore", fromBackendType, currentType)
		return false, nil
	}

	oldBackend, err := newBackendAt(fromBackendType, srcDir)
	if err != nil {
		return false, fmt.Errorf("failed to open backup as %s: %w", fromBackendType, err)
	}
	// Cleanup closes oldBackend and deletes its underlying file(s) - fine here since stagingRoot
	// is disposable and gets removed by the caller regardless; this just frees the handle
	// promptly (Windows) instead of leaving it open until that removal happens.
	defer oldBackend.Cleanup()

	logging.LogInfo(logging.KeyDBMigration, "kanban: restoring backup created with %s into current %s backend", fromBackendType, currentType)

	touched = true // point of no return - see doc comment above
	if err := storage.Cleanup(); err != nil {
		return touched, fmt.Errorf("failed to clear current backend: %w", err)
	}
	fresh, err := newBackend(currentType, currentStoragePath)
	if err != nil {
		return touched, fmt.Errorf("failed to reopen %s storage after clearing for restore: %w", currentType, err)
	}
	if err := migrate(oldBackend, fresh); err != nil {
		return touched, fmt.Errorf("restore conversion failed: %w", err)
	}
	storage = fresh
	return touched, nil
}

// Init initializes kanban event storage.
// If enabled is false the noop backend is used regardless of provider.
// If a different provider was previously active, all events are migrated automatically.
func Init(enabled bool, provider, storagePath string) error {
	storageMu.Lock()
	defer storageMu.Unlock()

	currentStoragePath = storagePath

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
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.LogEvent(filePath, boardFolder, fromStatus, toStatus)
}

// GetEvents retrieves kanban move events with optional filters, newest first.
// Pass empty strings / nil times to skip those filters; limit=0 means no limit.
func GetEvents(boardFolder, filePath string, from, to *time.Time, limit int) ([]Event, error) {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.GetEvents(boardFolder, filePath, from, to, limit)
}

// GetBackendType returns the backend type currently active ("sqlite", "json", or "noop" if
// kanban event logging is disabled).
func GetBackendType() string {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return storage.GetBackendType()
}
