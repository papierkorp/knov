// Package job - on-demand jobs triggered from the admin UI or API, not by the scheduler's tickers.
package job

import (
	"fmt"
	"os"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
)

// ----------------------------------------------------------------------------------------
// ------------------------------------ fullRebuildJob -------------------------------------
// ----------------------------------------------------------------------------------------

// RunFullRebuild runs the full metadata rebuild triggered from the admin UI:
// init all + purge stale/duplicates + links + orphaned media cache.
// Uses the same rebuildMu as the scheduled job to prevent concurrent runs.
func RunFullRebuild() error {
	return execute(&rebuildMu, &fullRebuildJob{})
}

type fullRebuildJob struct{}

func (j *fullRebuildJob) Name() string { return "metadata-full-rebuild" }

func (j *fullRebuildJob) Run() error {
	logging.LogInfo(logging.KeyFullRebuild, "running full metadata rebuild")

	files.StartMetaGetCounter()
	defer files.StopMetaGetCounter()

	if err := files.MetaDataInitializeAll(); err != nil {
		return fmt.Errorf("failed to initialize metadata: %w", err)
	}

	stalePurged, err := files.MetaDataPurgeStale()
	if err != nil {
		logging.LogError(logging.KeyFullRebuild, "full rebuild: failed to purge stale metadata: %v", err)
	}

	dupPurged, err := files.MetaDataPurgeDuplicates()
	if err != nil {
		logging.LogError(logging.KeyFullRebuild, "full rebuild: failed to purge duplicate metadata: %v", err)
	}

	if err := files.MetaDataLinksRebuild(logging.KeyFullRebuild); err != nil {
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

func (j *filterJob) Run() error {
	logging.LogDebug(logging.KeyFileSync, "running filter index cronjob")

	ids, err := filter.GetAllFilters()
	if err != nil {
		return fmt.Errorf("failed to list filters: %w", err)
	}

	var lastErr error
	for _, id := range ids {
		config, err := filter.GetFilterConfig(id)
		if err != nil || config == nil {
			logging.LogWarning(logging.KeyFileSync, "cronjob: failed to load filter config %s: %v", id, err)
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

func (j *notifJob) Run() error {
	if err := notificationStorage.Purge(100, 3); err != nil {
		return fmt.Errorf("failed to purge notifications: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- cacheInvalidateJob ----------------------------------
// ----------------------------------------------------------------------------------------

type cacheInvalidateJob struct{}

func (j *cacheInvalidateJob) Name() string { return "cache-invalidate" }

func (j *cacheInvalidateJob) Run() error {
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

func (j *mediaCleanupJob) Run() error {
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

func (j *repairBrokenLinksJob) Run() error {
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

func (j *gitPullJob) Run() error {
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

func (j *gitPushJob) Run() error {
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

func (j *gitRepackJob) Run() error {
	if err := git.RepackIfNeeded(); err != nil {
		return fmt.Errorf("git repack failed: %w", err)
	}
	return nil
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
}

func (j *bulkDeleteFilesJob) Name() string { return "bulk-delete-files" }

// Resumable is true because fullPaths is already the resolved snapshot the caller wants
// deleted - re-running it after a crash is safe (files.BulkDeleteFiles skips paths already gone).
func (j *bulkDeleteFilesJob) Resumable() bool { return true }

func (j *bulkDeleteFilesJob) Run() error {
	deleted := files.BulkDeleteFiles(logging.KeyApp, j.fullPaths)

	for _, fullPath := range deleted {
		if err := git.InvalidateFileHistoryCache(pathutils.ToRelative(fullPath)); err != nil {
			logging.LogWarning(logging.KeyApp, "bulk-delete-files: failed to invalidate file history cache for %s: %v", fullPath, err)
		}
	}
	if len(deleted) > 0 {
		go func() {
			if err := git.CommitDeletedFiles(deleted); err != nil {
				logging.LogError(logging.KeyApp, "bulk-delete-files: failed to commit deleted files (%s=%s): %v", j.groupType, j.groupVal, err)
			}
		}()
	}

	j.result = BulkDeleteResult{Deleted: len(deleted)}
	return nil
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
}

func (j *deleteFolderJob) Name() string { return "delete-folder" }

// Resumable is true because fullPaths is the resolved snapshot taken at job start, not the
// folder path - resuming re-deletes exactly that snapshot instead of re-walking a folder that
// may have gained new files since the crash.
func (j *deleteFolderJob) Resumable() bool { return true }

func (j *deleteFolderJob) Run() error {
	deleted := files.BulkDeleteFiles(logging.KeyApp, j.fullPaths)

	for _, fullPath := range deleted {
		if err := git.InvalidateFileHistoryCache(pathutils.ToRelative(fullPath)); err != nil {
			logging.LogWarning(logging.KeyApp, "delete-folder: failed to invalidate file history cache for %s: %v", fullPath, err)
		}
	}
	if err := os.RemoveAll(j.fullPath); err != nil {
		logging.LogWarning(logging.KeyApp, "delete-folder: failed to remove folder %s: %v", j.folderPath, err)
	}
	if len(deleted) > 0 {
		go func() {
			if err := git.CommitDeletedFiles(deleted); err != nil {
				logging.LogError(logging.KeyApp, "delete-folder: failed to commit deleted folder %s: %v", j.folderPath, err)
			}
		}()
	}

	j.result = BulkDeleteResult{Deleted: len(deleted)}
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

func (j *moveFolderJob) Run() error {
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
}

func (j *bulkUpdateMetadataJob) Name() string { return "bulk-update-metadata" }

func (j *bulkUpdateMetadataJob) Run() error {
	updated, failed := files.BulkUpdateMetadata(logging.KeyApp, j.matched, j.patch)
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
