// Package kanbanStorage - JSON backend implementation
package kanbanStorage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"knov/internal/backup"
	"knov/internal/logging"
	"knov/internal/utils"
)

const kanbanEventsFile = "events.json"

// jsonKanbanStorage implements KanbanStorage using a single JSON file.
type jsonKanbanStorage struct {
	filePath string
	mutex    sync.RWMutex
}

// newJSONStorage creates a new JSON kanban storage instance under storagePath/kanban.
func newJSONStorage(storagePath string) (*jsonKanbanStorage, error) {
	return newJSONStorageAt(filepath.Join(storagePath, "kanban"))
}

// newJSONStorageAt creates a new JSON kanban storage instance with its events file directly
// under dir, without joining on a "kanban" subfolder. Used to open a backup already extracted to
// its own leaf directory (see restoreMigrate), instead of relying on that directory happening to
// be named "kanban".
func newJSONStorageAt(dir string) (*jsonKanbanStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &jsonKanbanStorage{filePath: filepath.Join(dir, kanbanEventsFile)}, nil
}

func (s *jsonKanbanStorage) readEvents() ([]Event, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	events := []Event{}
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *jsonKanbanStorage) writeEvents(events []Event) error {
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	return utils.WriteFileAtomic(s.filePath, data, 0644)
}

// LogEvent records a kanban card move.
func (s *jsonKanbanStorage) LogEvent(filePath, boardFolder, fromStatus, toStatus string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	events, err := s.readEvents()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to read kanban events: %v", err)
		return err
	}
	events = append(events, Event{
		FilePath:    filePath,
		BoardFolder: boardFolder,
		FromStatus:  fromStatus,
		ToStatus:    toStatus,
		Timestamp:   time.Now(),
	})
	return s.writeEvents(events)
}

// GetEvents retrieves kanban move events with optional filters, newest first.
func (s *jsonKanbanStorage) GetEvents(boardFolder, filePath string, from, to *time.Time, limit int) ([]Event, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	events, err := s.readEvents()
	if err != nil {
		return nil, err
	}

	filtered := make([]Event, 0, len(events))
	for _, e := range events {
		if boardFolder != "" && e.BoardFolder != boardFolder {
			continue
		}
		if filePath != "" && e.FilePath != filePath {
			continue
		}
		if from != nil && e.Timestamp.Before(*from) {
			continue
		}
		if to != nil && e.Timestamp.After(*to) {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

// RenameStatus rewrites oldStatus to newStatus in the from/to status of every event.
func (s *jsonKanbanStorage) RenameStatus(oldStatus, newStatus string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	events, err := s.readEvents()
	if err != nil {
		return err
	}
	for i := range events {
		if events[i].FromStatus == oldStatus {
			events[i].FromStatus = newStatus
		}
		if events[i].ToStatus == oldStatus {
			events[i].ToStatus = newStatus
		}
	}
	return s.writeEvents(events)
}

// GetBackendType returns the backend type.
func (s *jsonKanbanStorage) GetBackendType() string {
	return "json"
}

// insertEvents appends events verbatim, preserving their original timestamps - used by the
// provider-migration path so kanban history survives a backend switch.
func (s *jsonKanbanStorage) insertEvents(newEvents []Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	events, err := s.readEvents()
	if err != nil {
		return err
	}
	events = append(events, newEvents...)
	return s.writeEvents(events)
}

// Cleanup removes the events file, so a subsequent migration back to json starts clean rather
// than appending onto stale rows.
func (s *jsonKanbanStorage) Cleanup() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		logging.LogError(logging.KeyApp, "json kanban cleanup: failed to remove %s: %v", s.filePath, err)
		return err
	}
	return nil
}

// Backup snapshots the kanban events file into destDir.
func (s *jsonKanbanStorage) Backup(destDir string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return backup.BackupFile(filepath.Dir(s.filePath), destDir)
}

// Restore overwrites the kanban events directory with a previously backed-up snapshot from srcDir.
func (s *jsonKanbanStorage) Restore(srcDir string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return backup.RestoreFile(srcDir, filepath.Dir(s.filePath))
}
