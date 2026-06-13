// Package kanbanStorage provides persistence for kanban event logs.
package kanbanStorage

import (
	"time"

	"knov/internal/logging"
)

// Event represents a single kanban card move.
type Event struct {
	FilePath   string    `json:"filePath"`
	Collection string    `json:"collection"`
	FromStatus string    `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	Timestamp  time.Time `json:"timestamp"`
}

// KanbanStorage defines the interface for kanban event persistence.
type KanbanStorage interface {
	LogEvent(filePath, collection, fromStatus, toStatus string) error
	GetEvents(collection, filePath string, from, to *time.Time, limit int) ([]Event, error)
}

var storage KanbanStorage

// Init initializes kanban event storage.
// If enabled is false the noop backend is used regardless of provider.
func Init(enabled bool, provider, storagePath string) error {
	if !enabled {
		storage = &noopStorage{}
		logging.LogInfo("kanban storage disabled")
		return nil
	}
	switch provider {
	case "sqlite":
		s, err := newSQLiteStorage(storagePath)
		if err != nil {
			return err
		}
		storage = s
		logging.LogInfo("kanban storage initialized: sqlite")
	default:
		s, err := newSQLiteStorage(storagePath)
		if err != nil {
			return err
		}
		storage = s
		logging.LogInfo("kanban storage initialized: sqlite (unknown provider %q, using sqlite)", provider)
	}
	return nil
}

// LogEvent records a kanban card move.
func LogEvent(filePath, collection, fromStatus, toStatus string) error {
	return storage.LogEvent(filePath, collection, fromStatus, toStatus)
}

// GetEvents retrieves kanban move events with optional filters, newest first.
// Pass empty strings / nil times to skip those filters; limit=0 means no limit.
func GetEvents(collection, filePath string, from, to *time.Time, limit int) ([]Event, error) {
	return storage.GetEvents(collection, filePath, from, to, limit)
}
