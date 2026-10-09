// Package trackerStorage persists a tracker counter's per-day deltas - the
// tabular, ever-growing half of a tracker, split out from the small
// title/columns config blob the tracker editor keeps in configStorage (see
// knov/internal/tracker).
package trackerStorage

import (
	"fmt"

	"knov/internal/backup"
	"knov/internal/logging"
)

// TrackerStorage defines persistence for tracker counter day-deltas.
type TrackerStorage interface {
	// GetDays returns counterID's recorded day->net-delta map, or an empty map
	// if it has none.
	GetDays(trackerID, counterID string) (map[string]int, error)
	// AddDelta atomically adds delta to counterID's bucket for day.
	AddDelta(trackerID, counterID, day string, delta int) error
	// ResetCounter clears every recorded day for counterID - used both to zero
	// a counter back to 0 and to hard-delete a counter removed from the editor.
	ResetCounter(trackerID, counterID string) error
	// DeleteTracker clears every counter's days for trackerID.
	DeleteTracker(trackerID string) error
	GetBackendType() string
	Backup(destDir string) error
	Restore(srcDir string) error
}

var storage TrackerStorage

func init() {
	backup.Register("tracker", backupAdapter{})
}

// backupAdapter defers to the package-level storage var, which isn't set until Init runs -
// unlike backup.Register, which happens at package init time before that.
type backupAdapter struct{}

func (backupAdapter) Backup(destDir string) error { return storage.Backup(destDir) }
func (backupAdapter) Restore(srcDir string) error { return storage.Restore(srcDir) }
func (backupAdapter) GetBackendType() string      { return storage.GetBackendType() }

// Init initializes tracker storage with the given provider (currently always sqlite).
// If enabled is false the noop backend is used regardless of provider.
func Init(enabled bool, provider, storagePath string) error {
	backup.CloseStorage(storage)
	if !enabled {
		storage = &noopStorage{}
		logging.LogInfo(logging.KeyApp, "tracker storage disabled")
		return nil
	}

	var err error
	switch provider {
	case "sqlite":
		storage, err = newSQLiteStorage(storagePath)
	default:
		logging.LogWarning(logging.KeyApp, "unknown tracker storage provider '%s', using sqlite", provider)
		storage, err = newSQLiteStorage(storagePath)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize tracker storage: %w", err)
	}
	logging.LogInfo(logging.KeyApp, "tracker storage initialized: %s", provider)
	return nil
}

// GetDays returns counterID's recorded day->net-delta map, or an empty map if it has none.
func GetDays(trackerID, counterID string) (map[string]int, error) {
	return storage.GetDays(trackerID, counterID)
}

// AddDelta atomically adds delta to counterID's bucket for day.
func AddDelta(trackerID, counterID, day string, delta int) error {
	return storage.AddDelta(trackerID, counterID, day, delta)
}

// ResetCounter clears every recorded day for counterID.
func ResetCounter(trackerID, counterID string) error {
	return storage.ResetCounter(trackerID, counterID)
}

// DeleteTracker clears every counter's days for trackerID.
func DeleteTracker(trackerID string) error {
	return storage.DeleteTracker(trackerID)
}

// GetBackendType returns the backend type currently active.
func GetBackendType() string {
	return storage.GetBackendType()
}

// Close closes the database of the storage, see backup.CloseStorage - for a test run that removes its scratch dir.
func Close() { backup.CloseStorage(storage) }
