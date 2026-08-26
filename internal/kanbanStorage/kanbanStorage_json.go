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

func newJSONStorage(storagePath string) (*jsonKanbanStorage, error) {
	fullPath := filepath.Join(storagePath, "kanban")
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}
	return &jsonKanbanStorage{filePath: filepath.Join(fullPath, kanbanEventsFile)}, nil
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
