// Package job handles periodic maintenance tasks.
package job

import (
	"fmt"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
	"knov/internal/search"
	"knov/internal/testdata"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------------- fileJob ---------------------------------------
// ----------------------------------------------------------------------------------------

type fileJob struct{}

func (j *fileJob) Name() string { return "file-sync" }

func (j *fileJob) Run() error {
	logging.LogDebug("running file cronjob")

	if err := git.PullRebase(); err != nil {
		logging.LogWarning("cronjob: git pull failed: %v", err)
	}

	var filesToProcess []string
	var filesToDelete []string

	if _, err := git.CommitAllPending(); err != nil {
		logging.LogError("cronjob: failed to commit pending changes: %v", err)
	}

	lastCommit, err := git.GetLastProcessedCommit()
	if err != nil {
		logging.LogError("cronjob: failed to get last processed commit: %v", err)
	} else {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("cronjob: failed to get current commit: %v", err)
		} else if currentCommit != "" && currentCommit != lastCommit {
			changedFiles, err := git.GetFilesChangedSinceCommit(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get changed files: %v", err)
			} else if len(changedFiles) > 0 {
				logging.LogInfo("detected %d files changed since last commit", len(changedFiles))
				filesToProcess = append(filesToProcess, changedFiles...)
			}

			deletedFiles, err := git.GetDeletedFilesSinceCommit(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get deleted files: %v", err)
			} else if len(deletedFiles) > 0 {
				logging.LogInfo("detected %d files deleted since last commit", len(deletedFiles))
				filesToDelete = append(filesToDelete, deletedFiles...)
			}

			movedFiles, err := git.GetFileRenames(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get file renames: %v", err)
			} else if len(movedFiles) > 0 {
				logging.LogInfo("detected %d file moves since last commit", len(movedFiles))
				for _, move := range movedFiles {
					oldNormalized := pathutils.ToWithPrefix(move.OldPath)
					newNormalized := pathutils.ToWithPrefix(move.NewPath)
					logging.LogInfo("processing file move: %s -> %s", oldNormalized, newNormalized)
					if err := files.UpdateLinksForMovedFile(oldNormalized, newNormalized); err != nil {
						logging.LogError("cronjob: failed to update links for moved file %s -> %s: %v", oldNormalized, newNormalized, err)
					} else {
						logging.LogInfo("successfully updated links for moved file %s -> %s", oldNormalized, newNormalized)
					}
					filesToProcess = append(filesToProcess, move.NewPath)
					filesToDelete = append(filesToDelete, move.OldPath)
				}
			}

			if err := git.SetLastProcessedCommit(currentCommit); err != nil {
				logging.LogError("cronjob: failed to save last processed commit: %v", err)
			}
		}
	}

	filesToProcess = removeDuplicates(filesToProcess)
	filesToDelete = removeDuplicates(filesToDelete)

	var filteredProcess []string
	for _, file := range filesToProcess {
		if !slices.Contains(filesToDelete, file) {
			filteredProcess = append(filteredProcess, file)
		}
	}
	filesToProcess = filteredProcess

	if len(filesToDelete) > 0 {
		logging.LogInfo("deleting metadata for %d files", len(filesToDelete))
		for _, filePath := range filesToDelete {
			normalizedPath := pathutils.ToWithPrefix(filePath)
			if err := files.MetaDataDelete(normalizedPath); err != nil {
				logging.LogError("cronjob: failed to delete metadata for %s: %v", normalizedPath, err)
				continue
			}
			logging.LogDebug("deleted metadata for %s", normalizedPath)
		}
	}

	if len(filesToProcess) == 0 {
		logging.LogDebug("no files to process")
	} else {
		logging.LogInfo("processing %d files", len(filesToProcess))
		for _, filePath := range filesToProcess {
			normalizedPath := pathutils.ToWithPrefix(filePath)
			metadata := &files.Metadata{
				Path:   normalizedPath,
				Editor: files.EditorTypeToastUI,
			}
			if err := files.MetaDataSave(metadata); err != nil {
				logging.LogError("cronjob: failed to save metadata for %s: %v", normalizedPath, err)
				continue
			}
			logging.LogDebug("processed metadata for %s", normalizedPath)
		}
	}

	if err := files.SaveAllSystemDataToCache(); err != nil {
		logging.LogError("cronjob: failed to save system data to cache: %v", err)
	}

	// run filter index as a sub-step so it gets its own history entry
	execute(&filterMu, &filterJob{})

	logging.LogDebug("file cronjob completed")
	return nil
}

func removeDuplicates(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- searchJob --------------------------------------
// ----------------------------------------------------------------------------------------

type searchIndexJob struct{}

func (j *searchIndexJob) Name() string { return "search-reindex" }

func (j *searchIndexJob) Run() error {
	logging.LogDebug("running search index cronjob")
	if err := search.IndexAllFiles(); err != nil {
		return fmt.Errorf("failed to reindex search: %w", err)
	}
	logging.LogDebug("search index cronjob completed")
	return nil
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- rebuildJob -------------------------------------
// ----------------------------------------------------------------------------------------

// rebuildJob is the lightweight scheduled rebuild (links only).
type rebuildJob struct{}

func (j *rebuildJob) Name() string { return "metadata-links-rebuild" }

func (j *rebuildJob) Run() error {
	logging.LogDebug("running metadata rebuild cronjob")
	if err := files.MetaDataLinksRebuild(); err != nil {
		return fmt.Errorf("metadata rebuild failed: %w", err)
	}
	logging.LogDebug("metadata rebuild cronjob completed")
	return nil
}

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
		if err := files.MetaDataDelete(mediaPath); err != nil {
			logging.LogWarning("media-cleanup: failed to delete metadata for %s: %v", mediaPath, err)
		}
		result.Deleted++
		logging.LogInfo("media-cleanup: deleted %s", mediaPath)
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning("media-cleanup: failed to refresh orphaned media cache: %v", err)
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

// ----------------------------------------------------------------------------------------
// ----------------------------------- testdata jobs --------------------------------------
// ----------------------------------------------------------------------------------------

type testdataSetupJob struct{}

func (j *testdataSetupJob) Name() string { return "testdata-setup" }

func (j *testdataSetupJob) Run() error {
	if err := testdata.SetupTestData(); err != nil {
		return fmt.Errorf("failed to setup test data: %w", err)
	}
	return nil
}

type testdataCleanJob struct{}

func (j *testdataCleanJob) Name() string { return "testdata-clean" }

func (j *testdataCleanJob) Run() error {
	if err := testdata.CleanTestData(); err != nil {
		return fmt.Errorf("failed to clean test data: %w", err)
	}
	return nil
}

type filterTestJob struct {
	results *testdata.FilterTestResults
}

func (j *filterTestJob) Name() string { return "filter-test" }

func (j *filterTestJob) Run() error {
	results, err := testdata.RunFilterTests()
	j.results = results
	if err != nil {
		return fmt.Errorf("filter tests failed: %w", err)
	}
	return nil
}

func (j *filterTestJob) Output() any { return j.results }

func (j *filterTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.PassedTests, j.results.FailedTests)
}

// RunFilterTest runs the filter test suite and returns its results alongside any error.
func RunFilterTest() (*testdata.FilterTestResults, error) {
	j := &filterTestJob{}
	if err := execute(&filterTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}
