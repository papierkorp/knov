// Package job - async, resumable jobs (folder delete, bulk delete-by-filter, full metadata
// rebuild, backup restore) started via StartAsync so the triggering HTTP request returns
// immediately and the caller polls for completion instead of blocking on a potentially slow
// operation.
package job

import (
	"encoding/json"
	"fmt"
	"sync"

	"knov/internal/files"
	"knov/internal/jobStorage"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
)

// Job type names, shared with the server package so status-routing code (e.g.
// api_jobs.go) doesn't have to duplicate these as untyped string literals.
const (
	JobTypeDeleteFolder    = "delete-folder"
	JobTypeBulkDeleteFiles = "bulk-delete-files"
	JobTypeFullRebuild     = "metadata-full-rebuild"
	JobTypeRestore         = "restore"
	JobTypeFileSync        = "file-sync"
	JobTypeExport          = "export"
)

// resumers maps a resumable job's Name() to a constructor that rebuilds it (and returns its
// dedup mutex) from the args persisted at StartAsync time. Used by RecoverInterrupted on
// startup. Registered directly here rather than via self-registration (contrast
// externalsuite.go's suiteRunners) since these are job's own types, not external packages.
var resumers = map[string]func(args string) (Job, *sync.Mutex, error){
	JobTypeDeleteFolder: func(args string) (Job, *sync.Mutex, error) {
		var a deleteFolderArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return nil, nil, fmt.Errorf("invalid delete-folder args: %w", err)
		}
		j := &deleteFolderJob{folderPath: a.FolderPath, fullPath: pathutils.ToDocsPath(a.FolderPath), fullPaths: a.FullPaths}
		return j, &deleteFolderMu, nil
	},
	JobTypeBulkDeleteFiles: func(args string) (Job, *sync.Mutex, error) {
		var a bulkDeleteArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return nil, nil, fmt.Errorf("invalid bulk-delete-files args: %w", err)
		}
		j := &bulkDeleteFilesJob{fullPaths: a.FullPaths, groupType: a.GroupType, groupVal: a.GroupVal}
		return j, &bulkDeleteFilesMu, nil
	},
	// no args to unmarshal - a full rebuild always starts fresh from disk, so resuming it
	// after a crash is just re-running the same no-state job again.
	JobTypeFullRebuild: func(args string) (Job, *sync.Mutex, error) {
		return &fullRebuildJob{}, &rebuildMu, nil
	},
	JobTypeRestore: func(args string) (Job, *sync.Mutex, error) {
		var a restoreArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return nil, nil, fmt.Errorf("invalid restore args: %w", err)
		}
		target, err := resolveExistingSet(a.SetName)
		if err != nil {
			return nil, nil, err
		}
		return &restoreJob{target: target, setName: a.SetName}, &backupMu, nil
	},
}

// IsCancellable reports whether jobType's Run() actually checks its context for cancellation -
// used by the jobs UI to decide whether a running job of this type gets a cancel button.
// bulk-delete-files/delete-folder check it between file deletes; metadata-full-rebuild checks
// it between files (canceling leaves metadata half-applied, so the job fires a notification
// telling the user to re-run - only reachable via StartFullRebuild, not the synchronous or
// cron paths, whose ctx is never canceled). Other StartAsync types either finish quickly
// (file-sync) or are unsafe to abort mid-run (restore).
func IsCancellable(jobType string) bool {
	switch jobType {
	case JobTypeBulkDeleteFiles, JobTypeDeleteFolder, JobTypeFullRebuild, JobTypeExport:
		return true
	default:
		return false
	}
}

type deleteFolderArgs struct {
	FolderPath string   `json:"folderPath"`
	FullPaths  []string `json:"fullPaths"`
}

type bulkDeleteArgs struct {
	FullPaths []string `json:"fullPaths"`
	GroupType string   `json:"groupType"`
	GroupVal  string   `json:"groupVal"`
}

// StartDeleteFolder resolves folderPath's files once (a stable snapshot, so a crash mid-run
// and later resume can't pick up files added afterwards) and starts the delete in the
// background. Returns the job id to poll for completion.
func StartDeleteFolder(folderPath string) (string, error) {
	fullPath := pathutils.ToDocsPath(folderPath)
	fullPaths, err := files.ListFilesInFolder(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to list folder contents: %w", err)
	}

	args, err := json.Marshal(deleteFolderArgs{FolderPath: folderPath, FullPaths: fullPaths})
	if err != nil {
		return "", fmt.Errorf("failed to encode job args: %w", err)
	}

	j := &deleteFolderJob{folderPath: folderPath, fullPath: fullPath, fullPaths: fullPaths}
	return StartAsync(&deleteFolderMu, j, string(args))
}

// StartBulkDeleteFiles starts deleting the given (already-resolved) files in the background.
// Returns the job id to poll for completion.
func StartBulkDeleteFiles(fullPaths []string, groupType, groupVal string) (string, error) {
	args, err := json.Marshal(bulkDeleteArgs{FullPaths: fullPaths, GroupType: groupType, GroupVal: groupVal})
	if err != nil {
		return "", fmt.Errorf("failed to encode job args: %w", err)
	}

	j := &bulkDeleteFilesJob{fullPaths: fullPaths, groupType: groupType, groupVal: groupVal}
	return StartAsync(&bulkDeleteFilesMu, j, string(args))
}

// ParseBulkDeleteArgs decodes the args persisted by StartBulkDeleteFiles, e.g. so a caller
// polling a job's status can find out which browse page to return to once it's done.
func ParseBulkDeleteArgs(args string) (groupType, groupVal string, err error) {
	var a bulkDeleteArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", "", fmt.Errorf("invalid bulk-delete-files args: %w", err)
	}
	return a.GroupType, a.GroupVal, nil
}

// RecoverInterrupted scans jobStorage for jobs still marked "running" after a restart (a clean
// shutdown leaves none - only a crash or kill mid-run does). Resumable job types are
// re-invoked with their persisted args; everything else is marked interrupted and surfaced to
// the user via a pending notification on their next page load.
func RecoverInterrupted() {
	running, err := jobStorage.ListRunning()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list running jobs for recovery: %v", err)
		return
	}

	for _, rec := range running {
		resume, ok := resumers[rec.Type]
		if !ok {
			markInterrupted(rec, "job type not resumable")
			continue
		}

		j, mu, err := resume(rec.Args)
		if err != nil {
			markInterrupted(rec, err.Error())
			continue
		}
		// resumers only holds constructors for types meant to be resumable, but Resumable()
		// is the authoritative gate - honor it rather than trusting registration alone.
		if r, ok := j.(Resumable); !ok || !r.Resumable() {
			markInterrupted(rec, "job type not resumable")
			continue
		}
		// resumeAsync continues under rec.ID rather than minting a new id, so a client
		// still polling GET /api/jobs/{rec.ID} from before the restart sees the resumed
		// run's actual outcome instead of a dead "interrupted" status.
		if err := resumeAsync(mu, j, rec.ID); err != nil {
			markInterrupted(rec, err.Error())
			continue
		}
		logging.LogInfo(logging.KeyApp, "resumed interrupted job %s (%s) after restart", rec.Type, rec.ID)
	}
}

// notifyAsyncFinished stores a pending notification for a finished StartAsync job so tabs
// other than the one that triggered it surface the outcome on their next page load (the
// triggering tab gets its own toast/redirect from the polling handler). Mirrors
// markInterrupted's pending-notification pattern - no live push. Canceled runs are skipped:
// they're user-initiated from a tab, and types that need special cancel messaging (e.g.
// full-rebuild) already add their own notification.
func notifyAsyncFinished(name, status, errMsg string) {
	var level, msg string
	switch status {
	case jobStorage.StatusDone:
		level, msg = "success", fmt.Sprintf("%s finished", name)
	case jobStorage.StatusError:
		level, msg = "error", fmt.Sprintf("%s failed: %s", name, errMsg)
	default:
		return
	}
	if _, err := notificationStorage.Add(level, msg, true); err != nil {
		logging.LogError(logging.KeyApp, "failed to store finished-job notification for %s: %v", name, err)
	}
}

func markInterrupted(rec jobStorage.JobRecord, reason string) {
	if err := jobStorage.UpdateStatus(rec.ID, jobStorage.StatusInterrupted, reason); err != nil {
		logging.LogError(logging.KeyApp, "failed to mark job %s interrupted: %v", rec.ID, err)
	}
	logging.LogWarning(logging.KeyApp, "job %s (%s) interrupted by restart: %s", rec.Type, rec.ID, reason)
	if _, err := notificationStorage.Add("warning",
		fmt.Sprintf("%s was interrupted by a restart and needs to be re-run manually", rec.Type), true); err != nil {
		logging.LogError(logging.KeyApp, "failed to store interrupted-job notification: %v", err)
	}
}
