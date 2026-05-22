// Package chatStorage provides chat message storage functionality
package chatStorage

import (
	"fmt"
	"time"

	"knov/internal/logging"
)

// Message represents a stored chat message
type Message struct {
	ID        string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	FilePath  string // empty = global chat, set = attached to file
}

// ChatStorage interface defines methods for chat message storage
type ChatStorage interface {
	Add(content, filePath string) (*Message, error)
	Delete(id string) error
	GetByID(id string) (*Message, error)
	GetPage(filePath string, limit, offset int) ([]Message, int, error)
	GetBackendType() string
}

var storage ChatStorage

// Init initializes chat storage with the specified provider
func Init(storagePath string) error {
	var err error

	storage, err = newSQLiteStorage(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize chat storage: %w", err)
	}

	logging.LogInfo("chat storage initialized: sqlite")
	return nil
}

// Add creates a new message
func Add(content, filePath string) (*Message, error) {
	return storage.Add(content, filePath)
}

// Delete removes a message by ID
func Delete(id string) error {
	return storage.Delete(id)
}

// GetByID returns a single message by ID
func GetByID(id string) (*Message, error) {
	return storage.GetByID(id)
}

// GetPage returns paginated messages for the given file path (empty = global) and total count
func GetPage(filePath string, limit, offset int) ([]Message, int, error) {
	return storage.GetPage(filePath, limit, offset)
}

// GetBackendType returns the storage backend type
func GetBackendType() string {
	return storage.GetBackendType()
}
