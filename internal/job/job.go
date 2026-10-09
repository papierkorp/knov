package job

import (
	"context"
	"errors"
	"time"
)

// ErrAlreadyRunning is returned by execute when the job's mutex is already held.
var ErrAlreadyRunning = errors.New("job already running")

// ErrNotRunning is returned by CancelAsync when id has no job currently in flight.
var ErrNotRunning = errors.New("job not running")

// JobStatus represents the outcome of a job run.
type JobStatus string

const (
	JobStatusRunning     JobStatus = "running"
	JobStatusOK          JobStatus = "ok"
	JobStatusError       JobStatus = "error"
	JobStatusCanceled    JobStatus = "canceled"
	JobStatusInterrupted JobStatus = "interrupted"
)

// JobRun records a single execution of a named job. ID is only set for StartAsync jobs
// (matching their jobStorage id) - it lets GetHistory tell a ring-buffer entry and its
// durable jobStorage counterpart apart when merging the two.
type JobRun struct {
	ID         string
	Name       string
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     JobStatus
	Error      string
	Output     any
}

// Job is implemented by anything that can be scheduled and tracked. ctx is canceled on a
// CancelAsync request for StartAsync jobs (context.Background() for synchronous execute jobs) -
// most Run() implementations ignore it; jobs with a per-item loop long enough to be worth
// interrupting (e.g. bulk delete) check ctx.Err() between iterations.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// Outputter may be implemented by a Job to expose its typed result in JobRun.Output.
type Outputter interface {
	Output() any
}

// Messenger may be implemented by a Job to provide a summary stored in JobRun.Error on success.
type Messenger interface {
	Message() string
}

// Resumable may be implemented by a Job started via StartAsync to indicate it can be safely
// re-invoked with the same persisted args after a crash mid-run (e.g. a delete job holding a
// pre-resolved file list, rather than a folder path it would have to re-walk non-idempotently).
// A Job that doesn't implement this is treated as not resumable.
type Resumable interface {
	Resumable() bool
}

// MediaCleanupResult holds the outcome of an orphaned media cleanup run.
type MediaCleanupResult struct {
	Deleted int   `json:"deleted"`
	Size    int64 `json:"size"`
	Failed  int   `json:"failed"`
}

// RepairBrokenLinksResult holds the outcome of a broken-links repair run.
type RepairBrokenLinksResult struct {
	Repaired int
	Skipped  int
}

// MigrateRelativeLinksResult holds the outcome of a relative links migration run, counted per
// selected doc and old target.
type MigrateRelativeLinksResult struct {
	Migrated int
	Skipped  int
}

// MigrateReservedFoldersResult holds the outcome of a reserved folders migration run.
type MigrateReservedFoldersResult struct {
	Metadata int // metadata records moved to their docs/ key
}

// BulkDeleteResult holds the outcome of a bulk/folder file delete run.
type BulkDeleteResult struct {
	Deleted int
}

// BulkUpdateResult holds the outcome of a bulk metadata update or folder move run.
type BulkUpdateResult struct {
	Updated int
	Failed  int
}
