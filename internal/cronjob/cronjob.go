// Package cronjob handles periodic maintenance tasks
package cronjob

import (
	"slices"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
	"knov/internal/search"
)

var (
	stopChan                chan bool
	fileInterval            time.Duration
	searchInterval          time.Duration
	metadataRebuildInterval time.Duration
)

// Start begins the cronjob scheduler
func Start() {
	stopChan = make(chan bool)

	// parse intervals from config
	fileIntervalStr := configmanager.GetAppConfig().CronjobInterval
	parsedFileInterval, err := time.ParseDuration(fileIntervalStr)
	if err != nil {
		logging.LogWarning("invalid cronjob interval '%s', using default 5m", fileIntervalStr)
		parsedFileInterval = 5 * time.Minute
	}
	fileInterval = parsedFileInterval

	searchIntervalStr := configmanager.GetAppConfig().SearchIndexInterval
	parsedSearchInterval, err := time.ParseDuration(searchIntervalStr)
	if err != nil {
		logging.LogWarning("invalid search index interval '%s', using default 15m", searchIntervalStr)
		parsedSearchInterval = 15 * time.Minute
	}
	searchInterval = parsedSearchInterval

	// start file processing cronjob
	go func() {
		ticker := time.NewTicker(fileInterval)
		defer ticker.Stop()

		// run once on startup
		runFileJobs()

		for {
			select {
			case <-ticker.C:
				runFileJobs()
			case <-stopChan:
				logging.LogInfo("file cronjob stopped")
				return
			}
		}
	}()

	// start search indexing cronjob
	go func() {
		ticker := time.NewTicker(searchInterval)
		defer ticker.Stop()

		// run once on startup
		runSearchJob()

		for {
			select {
			case <-ticker.C:
				runSearchJob()
			case <-stopChan:
				logging.LogInfo("search cronjob stopped")
				return
			}
		}
	}()

	metadataRebuildIntervalStr := configmanager.GetAppConfig().MetadataRebuildInterval
	parsedMetadataRebuildInterval, err := time.ParseDuration(metadataRebuildIntervalStr)
	if err != nil {
		logging.LogWarning("invalid metadata rebuild interval '%s', using default 30m", metadataRebuildIntervalStr)
		parsedMetadataRebuildInterval = 30 * time.Minute
	}
	metadataRebuildInterval = parsedMetadataRebuildInterval

	// start metadata rebuild cronjob
	go func() {
		ticker := time.NewTicker(metadataRebuildInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runMetadataRebuildJob()
			case <-stopChan:
				logging.LogInfo("metadata rebuild cronjob stopped")
				return
			}
		}
	}()

	logging.LogInfo("cronjob scheduler started (file: %v, search: %v, metadata rebuild: %v)", fileInterval, searchInterval, metadataRebuildInterval)
}

// Stop stops the cronjob scheduler
func Stop() {
	if stopChan != nil {
		close(stopChan)
	}
}

// Run manually triggers all cronjobs
func Run() {
	logging.LogInfo("manually triggering cronjobs")
	runFileJobs()
	runSearchJob()
	runMetadataRebuildJob()
	runFilterIndexJob()
	runNotificationPurgeJob()
	logging.LogInfo("manual cronjob execution completed")
}

func runFileJobs() {
	logging.LogDebug("running file cronjobs")

	// pull from remote before processing to stay in sync
	if err := git.PullRebase(); err != nil {
		logging.LogWarning("cronjob: git pull failed: %v", err)
	}

	var filesToProcess []string
	var filesToDelete []string

	// stage and commit all pending changes (modified, deleted, new) in one shot.
	// avoids go-git worktree.Status() detection issues by using AddWithOptions{All:true}.
	if _, err := git.CommitAllPending(); err != nil {
		logging.LogError("cronjob: failed to commit pending changes: %v", err)
	}

	// check for files changed since last processed commit
	lastCommit, err := git.GetLastProcessedCommit()
	if err != nil {
		logging.LogError("cronjob: failed to get last processed commit: %v", err)
	} else {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("cronjob: failed to get current commit: %v", err)
		} else if currentCommit != "" && currentCommit != lastCommit {
			// new commits detected
			changedFiles, err := git.GetFilesChangedSinceCommit(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get changed files: %v", err)
			} else if len(changedFiles) > 0 {
				logging.LogInfo("detected %d files changed since last commit", len(changedFiles))
				filesToProcess = append(filesToProcess, changedFiles...)
			}

			// get deleted files from commits
			deletedFiles, err := git.GetDeletedFilesSinceCommit(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get deleted files: %v", err)
			} else if len(deletedFiles) > 0 {
				logging.LogInfo("detected %d files deleted since last commit", len(deletedFiles))
				filesToDelete = append(filesToDelete, deletedFiles...)
			}

			// check for file moves/renames
			movedFiles, err := git.GetFileRenames(lastCommit)
			if err != nil {
				logging.LogError("cronjob: failed to get file renames: %v", err)
			} else if len(movedFiles) > 0 {
				logging.LogInfo("detected %d file moves since last commit", len(movedFiles))

				// process each file move
				for _, move := range movedFiles {
					// normalize paths for metadata operations
					oldNormalized := pathutils.ToWithPrefix(move.OldPath)
					newNormalized := pathutils.ToWithPrefix(move.NewPath)

					logging.LogInfo("processing file move: %s -> %s", oldNormalized, newNormalized)

					// update links in other files that reference the moved file
					if err := files.UpdateLinksForMovedFile(oldNormalized, newNormalized); err != nil {
						logging.LogError("cronjob: failed to update links for moved file %s -> %s: %v", oldNormalized, newNormalized, err)
					} else {
						logging.LogInfo("successfully updated links for moved file %s -> %s", oldNormalized, newNormalized)
					}

					// add new path to processing queue to ensure metadata is updated
					filesToProcess = append(filesToProcess, move.NewPath)

					// ensure old path is cleaned up
					filesToDelete = append(filesToDelete, move.OldPath)
				}
			}

			// update last processed commit
			if err := git.SetLastProcessedCommit(currentCommit); err != nil {
				logging.LogError("cronjob: failed to save last processed commit: %v", err)
			}
		}
	}

	// remove duplicates
	filesToProcess = removeDuplicates(filesToProcess)
	filesToDelete = removeDuplicates(filesToDelete)

	// remove deleted files from process list
	var filteredProcess []string
	for _, file := range filesToProcess {
		if !slices.Contains(filesToDelete, file) {
			filteredProcess = append(filteredProcess, file)
		}
	}
	filesToProcess = filteredProcess

	// process deleted files first
	if len(filesToDelete) > 0 {
		logging.LogInfo("deleting metadata for %d files", len(filesToDelete))
		for _, filePath := range filesToDelete {
			// normalize path to ensure correct prefix for metadata lookup
			normalizedPath := pathutils.ToWithPrefix(filePath)
			if err := files.MetaDataDelete(normalizedPath); err != nil {
				logging.LogError("cronjob: failed to delete metadata for %s: %v", normalizedPath, err)
				continue
			}
			logging.LogDebug("deleted metadata for %s", normalizedPath)
		}
	}

	// process changed/new files
	if len(filesToProcess) == 0 {
		logging.LogDebug("no files to process")
	} else {
		logging.LogInfo("processing %d files", len(filesToProcess))

		// process each file
		for _, filePath := range filesToProcess {
			// normalize path to ensure correct prefix for metadata lookup
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

	// save all system data to cache
	if err := files.SaveAllSystemDataToCache(); err != nil {
		logging.LogError("cronjob: failed to save system data to cache: %v", err)
	}

	// regenerate filter index files for any filters whose results may have changed
	runFilterIndexJob()

	logging.LogDebug("file cronjobs completed")
}

func runMetadataRebuildJob() {
	logging.LogDebug("running metadata rebuild cronjob")

	if err := files.MetaDataLinksRebuild(); err != nil {
		logging.LogError("cronjob: metadata rebuild failed: %v", err)
		return
	}

	logging.LogDebug("metadata rebuild cronjob completed")
}

func runFilterIndexJob() {
	logging.LogDebug("running filter index cronjob")

	ids, err := filter.GetAllFilters()
	if err != nil {
		logging.LogError("cronjob: failed to list filters: %v", err)
		return
	}

	for _, id := range ids {
		config, err := filter.GetFilterConfig(id)
		if err != nil || config == nil {
			logging.LogWarning("cronjob: failed to load filter config %s: %v", id, err)
			continue
		}
		if err := filter.GenerateFilterIndex(id, config); err != nil {
			logging.LogWarning("cronjob: failed to regenerate filter index %s: %v", id, err)
		}
	}

	logging.LogDebug("filter index cronjob completed (%d filters)", len(ids))
}

func runSearchJob() {
	logging.LogDebug("running search index cronjob")

	if err := search.IndexAllFiles(); err != nil {
		logging.LogError("cronjob: failed to reindex search: %v", err)
		return
	}

	logging.LogDebug("search index cronjob completed")
}

func removeDuplicates(files []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, file := range files {
		if !seen[file] {
			seen[file] = true
			result = append(result, file)
		}
	}

	return result
}

func runNotificationPurgeJob() {
	if err := notificationStorage.Purge(100, 3); err != nil {
		logging.LogError("cronjob: failed to purge notifications: %v", err)
	}
}
