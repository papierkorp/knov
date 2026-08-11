// Package chatStorage provides chat message storage functionality
package chatStorage

import (
	"fmt"
	"time"

	"knov/internal/backup"
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
	MoveFilePath(oldPath, newPath string) error
	DeleteByFilePath(filePath string) error
	Backup(destDir string) error
	Restore(srcDir string) error
}

var storage ChatStorage

func init() {
	backup.Register("chat", backupAdapter{})
}

// backupAdapter defers to the package-level storage var, which isn't set until Init runs -
// unlike backup.Register, which happens at package init time before that.
type backupAdapter struct{}

func (backupAdapter) Backup(destDir string) error { return storage.Backup(destDir) }
func (backupAdapter) Restore(srcDir string) error { return storage.Restore(srcDir) }

// Init initializes chat storage with the specified provider
func Init(storagePath string) error {
	var err error

	storage, err = newSQLiteStorage(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize chat storage: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "chat storage initialized: sqlite")
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

// MoveFilePath reattaches all messages from oldPath to newPath (used when a file is renamed/moved)
func MoveFilePath(oldPath, newPath string) error {
	return storage.MoveFilePath(oldPath, newPath)
}

// DeleteByFilePath removes all messages attached to filePath (used when a file is deleted)
func DeleteByFilePath(filePath string) error {
	return storage.DeleteByFilePath(filePath)
}
