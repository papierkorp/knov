// Package notificationStorage provides persistent notification log storage.
package notificationStorage

import (
	"fmt"
	"time"

	"knov/internal/logging"
)

// Notification represents a single stored notification entry.
type Notification struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Pending   bool      `json:"pending"`
}

// NotificationStorage defines the storage backend interface.
type NotificationStorage interface {
	Add(level, message string, pending bool) (*Notification, error)
	ConsumePending() (*Notification, error)
	GetRecent(limit int) ([]Notification, error)
	Purge(maxCount int, maxAgeDays int) error
	DeleteByID(id string) error
	Clear() error
	GetBackendType() string
}

var storage NotificationStorage

// Init initializes notification storage with the specified provider.
func Init(storagePath string) error {
	var err error

	storage, err = newSQLiteStorage(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize notification storage: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "notification storage initialized: sqlite")
	return nil
}

// Add stores a new notification. pending=true means it is waiting to be
// displayed on the next page load via the flash endpoint.
func Add(level, message string, pending bool) (*Notification, error) {
	return storage.Add(level, message, pending)
}

// ConsumePending atomically returns and clears the oldest pending notification,
// or nil if none. Atomic so concurrent callers never receive the same one twice.
func ConsumePending() (*Notification, error) {
	return storage.ConsumePending()
}

// GetRecent returns the most recent notifications, newest first.
func GetRecent(limit int) ([]Notification, error) {
	return storage.GetRecent(limit)
}

// Purge removes notifications exceeding maxCount or older than maxAgeDays.
// Called by the cronjob.
func Purge(maxCount int, maxAgeDays int) error {
	return storage.Purge(maxCount, maxAgeDays)
}

// DeleteByID removes a single notification by ID.
func DeleteByID(id string) error {
	return storage.DeleteByID(id)
}

// Clear removes all stored notifications.
func Clear() error {
	return storage.Clear()
}

// GetBackendType returns the storage backend type.
func GetBackendType() string {
	return storage.GetBackendType()
}
