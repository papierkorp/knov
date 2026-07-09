// Package job - on-demand jobs triggered from the admin UI or API, not by the scheduler's tickers.
package job

import (
	"fmt"
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
	logging.LogInfo("running full metadata rebuild")

	files.StartMetaGetCounter()
	defer files.StopMetaGetCounter()

	if err := files.MetaDataInitializeAll(); err != nil {
		return fmt.Errorf("failed to initialize metadata: %w", err)
	}

	stalePurged, err := files.MetaDataPurgeStale()
	if err != nil {
		logging.LogError("full rebuild: failed to purge stale metadata: %v", err)
	}

	dupPurged, err := files.MetaDataPurgeDuplicates()
	if err != nil {
		logging.LogError("full rebuild: failed to purge duplicate metadata: %v", err)
	}

	if err := files.MetaDataLinksRebuild(); err != nil {
		return fmt.Errorf("failed to rebuild metadata links: %w", err)
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning("full rebuild: failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo("full rebuild: purged %d stale, %d duplicate metadata entries", stalePurged, dupPurged)
	logging.LogInfo("full metadata rebuild completed")
	return nil
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- filterJob --------------------------------------
// ----------------------------------------------------------------------------------------

type filterJob struct{}

func (j *filterJob) Name() string { return "filter-reindex" }

func (j *filterJob) Run() error {
	logging.LogDebug("running filter index cronjob")

	ids, err := filter.GetAllFilters()
	if err != nil {
		return fmt.Errorf("failed to list filters: %w", err)
	}

	var lastErr error
	for _, id := range ids {
		config, err := filter.GetFilterConfig(id)
		if err != nil || config == nil {
			logging.LogWarning("cronjob: failed to load filter config %s: %v", id, err)
			continue
		}
		if err := filter.GenerateFilterIndex(id, config); err != nil {
			logging.LogWarning("cronjob: failed to regenerate filter index %s: %v", id, err)
			lastErr = err
		}
	}

	logging.LogDebug("filter index cronjob completed (%d filters)", len(ids))
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
			logging.LogWarning("media-cleanup: skipping %s: no longer orphaned", mediaPath)
			continue
		}

		fullPath := pathutils.ToMediaPath(strings.TrimPrefix(mediaPath, "media/"))
		if info, err := contentStorage.GetFileInfo(fullPath); err == nil && info != nil {
			result.Size += info.Size()
		}

		if err := contentStorage.DeleteFile(fullPath); err != nil {
			logging.LogError("media-cleanup: failed to delete %s: %v", mediaPath, err)
			result.Failed++
			continue
		}
		// no-refresh: avoid a full background cache rebuild per deleted file
		// when cleaning up dozens of orphaned media at once; refreshed once below.
		if err := files.MetaDataDeleteNoRefresh(mediaPath); err != nil {
			logging.LogWarning("media-cleanup: failed to delete metadata for %s: %v", mediaPath, err)
		}
		result.Deleted++
		logging.LogInfo("media-cleanup: deleted %s", mediaPath)
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning("media-cleanup: failed to refresh orphaned media cache: %v", err)
	}
	if result.Deleted > 0 {
		files.RefreshCaches()
	}

	return result, nil
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
