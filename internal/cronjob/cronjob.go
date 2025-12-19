// Package cronjob handles periodic maintenance tasks
package cronjob

import (
	"slices"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/search"
)

var (
	stopChan       chan bool
	fileInterval   time.Duration
	searchInterval time.Duration
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

	logging.LogInfo("cronjob scheduler started (file interval: %v, search interval: %v)", fileInterval, searchInterval)
}

// Stop stops the cronjob scheduler
func Stop() {
	if stopChan != nil {
		close(stopChan)
	}
}

// Run manually triggers both file and search cronjobs
func Run() {
	logging.LogInfo("manually triggering cronjobs")
	runFileJobs()
	runSearchJob()
	logging.LogInfo("manual cronjob execution completed")
}

func runFileJobs() {
	logging.LogDebug("running file cronjobs")

	var filesToProcess []string
	var filesToDelete []string

	// check for modified files and commit them
	modifiedFiles, err := git.GetModifiedFiles()
	if err != nil {
		logging.LogError("cronjob: failed to get modified files: %v", err)
	} else if len(modifiedFiles) > 0 {
		logging.LogInfo("detected %d modified files", len(modifiedFiles))

		// commit the modifications
		if err := git.CommitModifiedFiles(modifiedFiles); err != nil {
			logging.LogError("cronjob: failed to commit modified files: %v", err)
		} else {
			filesToProcess = append(filesToProcess, modifiedFiles...)
		}
	}

	// check for uncommitted deleted files
	uncommittedDeleted, err := git.GetUncommittedDeletedFiles()
	if err != nil {
		logging.LogError("cronjob: failed to get uncommitted deleted files: %v", err)
	} else if len(uncommittedDeleted) > 0 {
		logging.LogInfo("detected %d uncommitted deleted files", len(uncommittedDeleted))
		filesToDelete = append(filesToDelete, uncommittedDeleted...)

		// commit the deletions
		if err := git.CommitDeletedFiles(uncommittedDeleted); err != nil {
			logging.LogError("cronjob: failed to commit deleted files: %v", err)
		}
	}

	// check for new untracked files
	newFiles, err := git.AddNewFiles()
	if err != nil {
		logging.LogError("cronjob: failed to add new files: %v", err)
	} else if len(newFiles) > 0 {
		filesToProcess = append(filesToProcess, newFiles...)
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
			if err := files.MetaDataDelete(filePath); err != nil {
				logging.LogError("cronjob: failed to delete metadata for %s: %v", filePath, err)
				continue
			}
			logging.LogDebug("deleted metadata for %s", filePath)
		}
	}

	// process changed/new files
	if len(filesToProcess) == 0 {
		logging.LogDebug("no files to process")
	} else {
		logging.LogInfo("processing %d files", len(filesToProcess))

		// process each file
		for _, filePath := range filesToProcess {
			metadata := &files.Metadata{Path: filePath}
			if err := files.MetaDataSave(metadata); err != nil {
				logging.LogError("cronjob: failed to save metadata for %s: %v", filePath, err)
				continue
			}
			logging.LogDebug("processed metadata for %s", filePath)
		}
	}

	// save all system data to cache
	if err := files.SaveAllSystemDataToCache(); err != nil {
		logging.LogError("cronjob: failed to save system data to cache: %v", err)
	}

	logging.LogDebug("file cronjobs completed")
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
