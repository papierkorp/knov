// Package job - async, resumable delete jobs (folder delete, bulk delete-by-filter), started
// via StartAsync so the triggering HTTP request returns immediately and the caller polls for
// completion instead of blocking on a potentially slow delete.
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

// resumers maps a resumable job's Name() to a constructor that rebuilds it (and returns its
// dedup mutex) from the args persisted at StartAsync time. Used by RecoverInterrupted on
// startup. Registered directly here rather than via self-registration (contrast
// externalsuite.go's suiteRunners) since these are job's own types, not external packages.
var resumers = map[string]func(args string) (Job, *sync.Mutex, error){
	"delete-folder": func(args string) (Job, *sync.Mutex, error) {
		var a deleteFolderArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return nil, nil, fmt.Errorf("invalid delete-folder args: %w", err)
		}
		j := &deleteFolderJob{folderPath: a.FolderPath, fullPath: pathutils.ToDocsPath(a.FolderPath), fullPaths: a.FullPaths}
		return j, &deleteFolderMu, nil
	},
	"bulk-delete-files": func(args string) (Job, *sync.Mutex, error) {
		var a bulkDeleteArgs
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return nil, nil, fmt.Errorf("invalid bulk-delete-files args: %w", err)
		}
		j := &bulkDeleteFilesJob{fullPaths: a.FullPaths, groupType: a.GroupType, groupVal: a.GroupVal}
		return j, &bulkDeleteFilesMu, nil
	},
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
		if _, err := StartAsync(mu, j, rec.Args); err != nil {
			markInterrupted(rec, err.Error())
			continue
		}
		// the interrupted record stays as-is (still "running") - StartAsync creates a
		// fresh record for the resumed attempt; UpdateStatus on the old id below would
		// race with it, so just drop the stale row instead of double-bookkeeping it.
		if err := jobStorage.UpdateStatus(rec.ID, jobStorage.StatusInterrupted, "resumed as a new job after restart"); err != nil {
			logging.LogError(logging.KeyApp, "failed to close out stale job record %s: %v", rec.ID, err)
		}
		logging.LogInfo(logging.KeyApp, "resumed interrupted job %s (%s) after restart", rec.Type, rec.ID)
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
