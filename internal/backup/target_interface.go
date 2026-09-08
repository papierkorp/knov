package backup

import (
	"io"
	"time"
)

// EventKind identifies what a logged Event recorded.
type EventKind string

const (
	EventBackup  EventKind = "backup"
	EventRestore EventKind = "restore"
)

// EventSource identifies what triggered a logged Event.
type EventSource string

const (
	SourceManual    EventSource = "manual"    // triggered from /system/backup
	SourceScheduled EventSource = "scheduled" // KNOV_BACKUP_AUTO_PROFILES, via job.checkAutoBackup
	SourceRestore   EventSource = "restore"   // the pre-restore safety snapshot Restore always takes first
)

// Event is one durable, append-only record of a backup being created or a restore being
// applied - kept independent of whether the referenced set still exists on the target, so
// history survives both rotation (the set being deleted later) and a restore's own restart
// (which would otherwise wipe any purely in-memory record of the very event it just performed).
type Event struct {
	Kind   EventKind   `json:"kind"`
	Set    string      `json:"set"`
	Time   time.Time   `json:"time"`
	Source EventSource `json:"source"`
}

// BackupTarget is where finished backup set archives are stored. Byte-stream shaped (not
// filesystem-path shaped) so a remote implementation (S3, NFS) can be added later as a new file
// without touching Run/Restore.
type BackupTarget interface {
	Write(name string, r io.Reader) error
	List() ([]string, error)
	Read(name string) (io.ReadCloser, error)
	Delete(name string) error
	// Lock marks name to never be deleted by Rotate, until Unlock is called.
	Lock(name string) error
	Unlock(name string) error
	Locked(name string) (bool, error)
	// LockedNames returns every currently-locked set name in one call, so Log/Rotate don't do a
	// per-set round-trip (one HEAD request each on a remote target) just to read lock state.
	LockedNames() (map[string]bool, error)
	// LogEvent durably appends kind/set/source/now to the target's event history - see Events.
	LogEvent(kind EventKind, set string, source EventSource) error
	// Events returns every logged event, oldest first.
	Events() ([]Event, error)
}
