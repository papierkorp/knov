package job

import (
	"errors"
	"time"
)

// ErrAlreadyRunning is returned by execute when the job's mutex is already held.
var ErrAlreadyRunning = errors.New("job already running")

// JobStatus represents the outcome of a job run.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusOK      JobStatus = "ok"
	JobStatusError   JobStatus = "error"
)

// JobRun records a single execution of a named job.
type JobRun struct {
	Name       string
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     JobStatus
	Error      string
	Output     any
}

// Job is implemented by anything that can be scheduled and tracked.
type Job interface {
	Name() string
	Run() error
}

// Outputter may be implemented by a Job to expose its typed result in JobRun.Output.
type Outputter interface {
	Output() any
}

// Messenger may be implemented by a Job to provide a summary stored in JobRun.Error on success.
type Messenger interface {
	Message() string
}

// MediaCleanupResult holds the outcome of an orphaned media cleanup run.
type MediaCleanupResult struct {
	Deleted int
	Size    int64
	Failed  int
}
