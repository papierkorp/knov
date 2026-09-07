// Package job - on-demand jobs triggered from the admin UI or API, not by the scheduler's tickers.
package job

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/jobStorage"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
)

// ----------------------------------------------------------------------------------------
// ------------------------------------ fullRebuildJob -------------------------------------
// ----------------------------------------------------------------------------------------

// RunFullRebuild runs the full metadata rebuild synchronously: init all + purge stale/duplicates
// + links + orphaned media cache. Uses the same rebuildMu as the scheduled job to prevent
// concurrent runs. Kept alongside the async StartFullRebuild for callers (e.g. jobstest) that
// need the result available as soon as the call returns.
func RunFullRebuild() error {
	return execute(&rebuildMu, &fullRebuildJob{})
}

// StartFullRebuild runs the full metadata rebuild in the background, triggered from the admin
// UI so the request doesn't block for the duration of a potentially slow full-vault rebuild.
// Returns the job id to poll for completion.
func StartFullRebuild() (string, error) {
	return StartAsync(&rebuildMu, &fullRebuildJob{}, "")
}

type fullRebuildJob struct{ withProgress }

func (j *fullRebuildJob) Name() string { return JobTypeFullRebuild }

// Resumable is true because a full rebuild always starts fresh from disk - re-running it after
// a crash is exactly the same as running it fresh, no snapshot/state needed.
func (j *fullRebuildJob) Resumable() bool { return true }

func (j *fullRebuildJob) Run(ctx context.Context) (err error) {
	logging.LogInfo(logging.KeyFullRebuild, "running full metadata rebuild")

	files.StartMetaGetCounter()
	defer files.StopMetaGetCounter()

	// a canceled rebuild leaves metadata half-applied - the bare "canceled" job status
	// doesn't convey that, so tell the user it needs re-running.
	defer func() {
		if errors.Is(err, context.Canceled) {
			if _, nErr := notificationStorage.Add("warning",
				"full metadata rebuild was canceled - metadata may be inconsistent, re-run it", true); nErr != nil {
				logging.LogError(logging.KeyFullRebuild, "failed to store canceled-rebuild notification: %v", nErr)
			}
		}
	}()

	if err := files.MetaDataInitializeAll(); err != nil {
		return fmt.Errorf("failed to initialize metadata: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	stalePurged, err := files.MetaDataPurgeStale()
	if err != nil {
		logging.LogError(logging.KeyFullRebuild, "full rebuild: failed to purge stale metadata: %v", err)
	}

	dupPurged, err := files.MetaDataPurgeDuplicates()
	if err != nil {
		logging.LogError(logging.KeyFullRebuild, "full rebuild: failed to purge duplicate metadata: %v", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := files.MetaDataLinksRebuild(ctx, logging.KeyFullRebuild, j.prog.Report); err != nil {
		return fmt.Errorf("failed to rebuild metadata links: %w", err)
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning(logging.KeyFullRebuild, "full rebuild: failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo(logging.KeyFullRebuild, "full rebuild: purged %d stale, %d duplicate metadata entries", stalePurged, dupPurged)
	logging.LogInfo(logging.KeyFullRebuild, "full metadata rebuild completed")
	return nil
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- filterJob --------------------------------------
// ----------------------------------------------------------------------------------------

type filterJob struct{}

func (j *filterJob) Name() string { return "filter-reindex" }

func (j *filterJob) Run(_ context.Context) error {
	logging.LogDebug(logging.KeyFileSync, "running filter index cronjob")

	ids, err := filter.GetAllFilters()
	if err != nil {
		return fmt.Errorf("failed to list filters: %w", err)
	}

	var lastErr error
	for _, id := range ids {
		config, err := filter.GetFilterConfig(id)
		if err != nil {
			logging.LogWarning(logging.KeyFileSync, "cronjob: failed to load filter config %s: %v", id, err)
			continue
		}
		if config == nil {
			continue
		}
		if err := filter.GenerateFilterIndex(id, config); err != nil {
			logging.LogWarning(logging.KeyFileSync, "cronjob: failed to regenerate filter index %s: %v", id, err)
			lastErr = err
		}
	}

	logging.LogDebug(logging.KeyFileSync, "filter index cronjob completed (%d filters)", len(ids))
	return lastErr
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- notifJob ---------------------------------------
// ----------------------------------------------------------------------------------------

type notifJob struct{}

func (j *notifJob) Name() string { return "notification-purge" }

func (j *notifJob) Run(_ context.Context) error {
	if err := notificationStorage.Purge(100, 3); err != nil {
		return fmt.Errorf("failed to purge notifications: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ jobRecordPurgeJob --------------------------------
// ----------------------------------------------------------------------------------------

// jobRecordPurgeJob trims the persistent jobStorage table (async-job records). It does not
// touch the separate in-memory job-history ring buffer (see history.go).
type jobRecordPurgeJob struct{}

func (j *jobRecordPurgeJob) Name() string { return "job-record-purge" }

func (j *jobRecordPurgeJob) Run(_ context.Context) error {
	if err := jobStorage.Purge(200, 30); err != nil {
		return fmt.Errorf("failed to purge job records: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- cacheInvalidateJob ----------------------------------
// ----------------------------------------------------------------------------------------

type cacheInvalidateJob struct{}

func (j *cacheInvalidateJob) Name() string { return "cache-invalidate" }

func (j *cacheInvalidateJob) Run(_ context.Context) error {
	if err := files.CacheInvalidate(); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// ----------------------------------- mediaCleanupJob ------------------------------------
// ----------------------------------------------------------------------------------------

type mediaCleanupJob struct {
	result MediaCleanupResult
}

func (j *mediaCleanupJob) Name() string { return "media-cleanup" }

func (j *mediaCleanupJob) Run(_ context.Context) error {
	result, err := doMediaCleanup()
	j.result = result
	return err
}

func (j *mediaCleanupJob) Output() any { return j.result }

func (j *mediaCleanupJob) Message() string {
	msg := fmt.Sprintf("deleted %d files (%.2f MB)", j.result.Deleted, float64(j.result.Size)/(1024*1024))
	if j.result.Failed > 0 {
		msg += fmt.Sprintf(", %d failed", j.result.Failed)
	}
	return msg
}

// doMediaCleanup is the shared implementation used by mediaCleanupJob.Run.
func doMediaCleanup() (MediaCleanupResult, error) {
	orphanedMedia, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		return MediaCleanupResult{}, fmt.Errorf("failed to get orphaned media: %w", err)
	}

	var result MediaCleanupResult
	for _, mediaPath := range orphanedMedia {
		// double-check the file is still orphaned (cache may be stale)
		meta, err := files.MetaDataGet(mediaPath)
		if err == nil && meta != nil && len(meta.LinksToHere) > 0 {
			logging.LogWarning(logging.KeyMediaCleanup, "media-cleanup: skipping %s: no longer orphaned", mediaPath)
			continue
		}

		fullPath := pathutils.ToMediaPath(strings.TrimPrefix(mediaPath, "media/"))
		if info, err := contentStorage.GetFileInfo(fullPath); err == nil && info != nil {
			result.Size += info.Size()
		}

		if err := contentStorage.DeleteFile(fullPath); err != nil {
			logging.LogError(logging.KeyMediaCleanup, "media-cleanup: failed to delete %s: %v", mediaPath, err)
			result.Failed++
			continue
		}
		// no-refresh: avoid a full background cache rebuild per deleted file
		// when cleaning up dozens of orphaned media at once; refreshed once below.
		if err := files.MetaDataDeleteNoRefresh(logging.KeyMediaCleanup, mediaPath); err != nil {
			logging.LogWarning(logging.KeyMediaCleanup, "media-cleanup: failed to delete metadata for %s: %v", mediaPath, err)
		}
		result.Deleted++
		logging.LogInfo(logging.KeyMediaCleanup, "media-cleanup: deleted %s", mediaPath)
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning(logging.KeyMediaCleanup, "media-cleanup: failed to refresh orphaned media cache: %v", err)
	}
	if result.Deleted > 0 {
		files.RefreshCaches()
	}

	return result, nil
}

// ----------------------------------------------------------------------------------------
// -------------------------------- repairBrokenLinksJob -----------------------------------
// ----------------------------------------------------------------------------------------

type repairBrokenLinksJob struct {
	entries []string
	result  RepairBrokenLinksResult
}

func (j *repairBrokenLinksJob) Name() string { return "repair-broken-links" }

func (j *repairBrokenLinksJob) Run(_ context.Context) error {
	var result RepairBrokenLinksResult
	for _, entry := range j.entries {
		parts := strings.SplitN(entry, "|", 3)
		if len(parts) != 3 {
			continue
		}
		sourceFile, target, suggested := parts[0], parts[1], parts[2]
		ok, err := files.RepairBrokenLink(sourceFile, target, suggested)
		if err != nil {
			logging.LogError(logging.KeyRepairLinks, "skipped: %s: %s -> %s (error: %v)", sourceFile, target, suggested, err)
			result.Skipped++
			continue
		}
		if !ok {
			logging.LogWarning(logging.KeyRepairLinks, "skipped: %s: %s -> %s (no matching link occurrence found)", sourceFile, target, suggested)
			result.Skipped++
			continue
		}
		logging.LogInfo(logging.KeyRepairLinks, "repaired: %s: %s -> %s", sourceFile, target, suggested)
		go git.CommitFile(pathutils.ToFullPath(sourceFile))
		result.Repaired++
	}

	if result.Repaired > 0 {
		files.RefreshCaches()
	}

	j.result = result
	return nil
}

func (j *repairBrokenLinksJob) Output() any { return j.result }

func (j *repairBrokenLinksJob) Message() string {
	return fmt.Sprintf("repaired %d links, %d skipped", j.result.Repaired, j.result.Skipped)
}

// ----------------------------------------------------------------------------------------
// -------------------------------------- gitPullJob --------------------------------------
// ----------------------------------------------------------------------------------------

type gitPullJob struct{}

func (j *gitPullJob) Name() string { return "git-pull" }

func (j *gitPullJob) Run(_ context.Context) error {
	if configmanager.GetGitRemote() == "" {
		return fmt.Errorf("no remote configured")
	}
	if err := git.PullRebase(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// -------------------------------------- gitPushJob --------------------------------------
// ----------------------------------------------------------------------------------------

type gitPushJob struct{}

func (j *gitPushJob) Name() string { return "git-push" }

func (j *gitPushJob) Run(_ context.Context) error {
	if configmanager.GetGitRemote() == "" {
		return fmt.Errorf("no remote configured")
	}
	git.Push()
	return nil
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ gitRepackJob ---------------------------------------
// ----------------------------------------------------------------------------------------

type gitRepackJob struct{}

func (j *gitRepackJob) Name() string { return "git-repack" }

func (j *gitRepackJob) Run(_ context.Context) error {
	if err := git.RepackIfNeeded(); err != nil {
		return fmt.Errorf("git repack failed: %w", err)
	}
	return nil
}

// deleteResolvedFiles deletes a pre-resolved snapshot of files via files.BulkDeleteFiles,
// invalidates each deleted file's history cache, and commits the deletion, all before
// returning. Shared by bulkDeleteFilesJob and deleteFolderJob, which differ only in what they
// do with the deleted paths afterwards. If ctx is canceled mid-delete, BulkDeleteFiles stops
// after the file it's currently on - whatever was deleted up to that point is still committed
// below, the caller's Run() is left to report the cancellation.
//
// The commit runs synchronously (not in a detached goroutine) because this only ever runs
// inside a StartAsync job: the caller is already a background goroutine, so there's no request
// to keep fast, and a crash mid-commit leaves the job's jobStorage row "running" - so
// RecoverInterrupted replays this same snapshot on next startup and retries the commit,
// instead of the commit being silently lost forever as it would be if detached.
//
// The returned count also covers paths already gone before this call (e.g. a resumed run
// re-deleting its snapshot after a crash) - see files.BulkDeleteFiles - so a resumed run still
// commits those. Paths that failed to remove for another reason are excluded, since they may
// still exist on disk and committing them as deleted would desync git/cache from actual state.
func deleteResolvedFiles(ctx context.Context, logPrefix string, fullPaths []string, report func(done, total int)) []string {
	deleted := files.BulkDeleteFiles(ctx, logging.KeyApp, fullPaths, report)

	for _, fullPath := range deleted {
		if err := git.InvalidateFileHistoryCache(pathutils.ToRelative(fullPath)); err != nil {
			logging.LogWarning(logging.KeyApp, "%s: failed to invalidate file history cache for %s: %v", logPrefix, fullPath, err)
		}
	}
	if len(deleted) > 0 {
		// commit failure is logged only, not returned as a job error - the files are already
		// gone from disk either way, so the delete itself still succeeded.
		if err := git.CommitDeletedFiles(deleted); err != nil {
			logging.LogError(logging.KeyApp, "%s: failed to commit deleted files: %v", logPrefix, err)
		}
	}
	return deleted
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- bulkDeleteFilesJob -----------------------------------
// ----------------------------------------------------------------------------------------

// bulkDeleteFilesJob deletes a pre-resolved set of files (e.g. everything matching a
// collection/folder/tag) and their metadata, then commits the deletion in the background.
// The actual file/metadata work lives in files.BulkDeleteFiles - this is just the
// history-tracking + git wiring around it.
type bulkDeleteFilesJob struct {
	fullPaths           []string
	groupType, groupVal string
	result              BulkDeleteResult
	withProgress
}

func (j *bulkDeleteFilesJob) Name() string { return JobTypeBulkDeleteFiles }

// Resumable is true because fullPaths is already the resolved snapshot the caller wants
// deleted - re-running it after a crash is safe (files.BulkDeleteFiles skips paths already gone).
func (j *bulkDeleteFilesJob) Resumable() bool { return true }

func (j *bulkDeleteFilesJob) Run(ctx context.Context) error {
	deleted := deleteResolvedFiles(ctx, "bulk-delete-files", j.fullPaths, j.prog.Report)
	j.result = BulkDeleteResult{Deleted: len(deleted)}
	return ctx.Err()
}

func (j *bulkDeleteFilesJob) Output() any { return j.result }

func (j *bulkDeleteFilesJob) Message() string {
	return fmt.Sprintf("deleted %d files (%s=%s)", j.result.Deleted, j.groupType, j.groupVal)
}

// ----------------------------------------------------------------------------------------
// ----------------------------------- deleteFolderJob --------------------------------------
// ----------------------------------------------------------------------------------------

// deleteFolderJob deletes a pre-resolved snapshot of the files inside a folder (resolved once
// by StartDeleteFolder before the job starts, not re-walked here - see ListFilesInFolder),
// then removes the now-empty folder tree and commits the deletion in the background.
type deleteFolderJob struct {
	folderPath string   // relative, for history/messages only
	fullPath   string   // resolved absolute folder path, removed once its files are gone
	fullPaths  []string // resolved absolute file paths to delete
	result     BulkDeleteResult
	withProgress
}

func (j *deleteFolderJob) Name() string { return JobTypeDeleteFolder }

// Resumable is true because fullPaths is the resolved snapshot taken at job start, not the
// folder path - resuming re-deletes exactly that snapshot instead of re-walking a folder that
// may have gained new files since the crash.
func (j *deleteFolderJob) Resumable() bool { return true }

func (j *deleteFolderJob) Run(ctx context.Context) error {
	deleted := deleteResolvedFiles(ctx, "delete-folder", j.fullPaths, j.prog.Report)
	j.result = BulkDeleteResult{Deleted: len(deleted)}
	if err := ctx.Err(); err != nil {
		// canceled before every file was deleted - the folder is deliberately left as-is
		// rather than attempting RemoveEmptyDirTree below, which would just fail on it.
		return err
	}

	// RemoveEmptyDirTree only removes directories left empty by the deletes above - unlike
	// os.RemoveAll, it won't touch a file written into the folder after the snapshot was
	// taken (e.g. during a slow delete, or a crash-to-resume gap). A failure here (leftover
	// content, or a locked file on Windows) must still surface as a job error, not a silent
	// "folder deleted" success while the folder is still there.
	if err := files.RemoveEmptyDirTree(j.fullPath); err != nil {
		return fmt.Errorf("deleted %d files but failed to remove folder %s: %w", len(deleted), j.folderPath, err)
	}
	return nil
}

func (j *deleteFolderJob) Output() any { return j.result }

func (j *deleteFolderJob) Message() string {
	return fmt.Sprintf("deleted folder %s (%d files)", j.folderPath, j.result.Deleted)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ moveFolderJob ----------------------------------------
// ----------------------------------------------------------------------------------------

// moveFolderJob moves a folder to a new parent and updates the links of every file inside it.
// The actual work lives in files.MoveFolder - this is just the history-tracking wrapper.
type moveFolderJob struct {
	currentPath, newPath string
	result               BulkUpdateResult
}

func (j *moveFolderJob) Name() string { return "move-folder" }

func (j *moveFolderJob) Run(_ context.Context) error {
	updated, failed, err := files.MoveFolder(logging.KeyApp, pathutils.ToDocsPath(j.currentPath), pathutils.ToDocsPath(j.newPath))
	if err != nil {
		return err
	}

	j.result = BulkUpdateResult{Updated: updated, Failed: failed}
	return nil
}

func (j *moveFolderJob) Output() any { return j.result }

func (j *moveFolderJob) Message() string {
	return fmt.Sprintf("moved %s -> %s (%d files updated)", j.currentPath, j.newPath, j.result.Updated)
}

// ----------------------------------------------------------------------------------------
// -------------------------------- bulkUpdateMetadataJob -----------------------------------
// ----------------------------------------------------------------------------------------

// bulkUpdateMetadataJob applies a metadata patch to a pre-resolved (filter-matched) set of
// files. The actual work lives in files.BulkUpdateMetadata - this is just the
// history-tracking wrapper.
type bulkUpdateMetadataJob struct {
	matched []files.File
	patch   files.BulkUpdatePatch
	result  BulkUpdateResult
	withProgress
}

func (j *bulkUpdateMetadataJob) Name() string { return "bulk-update-metadata" }

func (j *bulkUpdateMetadataJob) Run(_ context.Context) error {
	updated, failed := files.BulkUpdateMetadata(logging.KeyApp, j.matched, j.patch, j.prog.Report)
	j.result = BulkUpdateResult{Updated: updated, Failed: failed}
	return nil
}

func (j *bulkUpdateMetadataJob) Output() any { return j.result }

func (j *bulkUpdateMetadataJob) Message() string {
	msg := fmt.Sprintf("updated %d files", j.result.Updated)
	if j.result.Failed > 0 {
		msg += fmt.Sprintf(", %d failed", j.result.Failed)
	}
	return msg
}
