// Package jobStorage provides persistent state for async jobs, so a crash or
// restart while a job is running can be detected and handled on next startup.
package jobStorage

import (
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
)

// JobRecord represents a single persisted async job run.
type JobRecord struct {
	ID         string
	Type       string
	Args       string // JSON blob, job-type specific
	Status     string
	StartedAt  time.Time
	FinishedAt *time.Time
	Error      string
}

// JobStorage defines the storage backend interface.
type JobStorage interface {
	Create(id, jobType, args string) error
	UpdateStatus(id, status, errMsg string) error
	Get(id string) (*JobRecord, error)
	ListRunning() ([]JobRecord, error)
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

// GetBackendType returns the storage backend type.
func GetBackendType() string {
	return storage.GetBackendType()
}
