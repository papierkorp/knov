// Package jobStorage provides persistent state for async jobs, so a crash or
// restart while a job is running can be detected and handled on next startup.
package jobStorage

import (
	"knov/internal/backup"
	"fmt"
	"time"

	"knov/internal/logging"
)

// Status values for a stored job run.
const (
	StatusRunning     = "running"
	StatusDone        = "done"
	StatusError       = "error"
	StatusInterrupted = "interrupted"
	StatusCanceled    = "canceled"
)

// JobRecord represents a single persisted async job run.
type JobRecord struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Args       string     `json:"args"` // JSON blob, job-type specific
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Error      string     `json:"error"`
}

// JobStorage defines the storage backend interface.
type JobStorage interface {
	Create(id, jobType, args string) error
	UpdateStatus(id, status, errMsg string) error
	Get(id string) (*JobRecord, error)
	ListRunning() ([]JobRecord, error)
	List(limit int) ([]JobRecord, error)
	Purge(maxCount int, maxAgeDays int) error
	GetBackendType() string
}

var storage JobStorage

// Init initializes job storage with the specified provider.
func Init(storagePath string) error {
	var err error

	storage, err = newSQLiteStorage(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize job storage: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "job storage initialized: sqlite")
	return nil
}

// Create persists a new job record with status=running.
func Create(id, jobType, args string) error {
	return storage.Create(id, jobType, args)
}

// UpdateStatus updates a job record's status and, for terminal statuses,
// its finished_at timestamp and error message.
func UpdateStatus(id, status, errMsg string) error {
	return storage.UpdateStatus(id, status, errMsg)
}

// Get returns a single job record by id, or nil if not found.
func Get(id string) (*JobRecord, error) {
	return storage.Get(id)
}

// ListRunning returns every job record still marked as running, e.g. to
// resume or mark interrupted on startup after a crash.
func ListRunning() ([]JobRecord, error) {
	return storage.ListRunning()
}

// List returns the most recent job records of any status, newest first, capped at limit.
func List(limit int) ([]JobRecord, error) {
	return storage.List(limit)
}

// Purge removes finished job records exceeding maxCount or older than maxAgeDays.
// A limit <= 0 is treated as "no limit". Rows still marked running are never removed.
func Purge(maxCount int, maxAgeDays int) error {
	return storage.Purge(maxCount, maxAgeDays)
}

// GetBackendType returns the storage backend type.
func GetBackendType() string {
	return storage.GetBackendType()
}

// Close closes the database of the storage, see backup.CloseStorage - for a test run that removes its scratch dir.
func Close() { backup.CloseStorage(storage) }
