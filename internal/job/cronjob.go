// Package job handles periodic maintenance tasks.
package job

import (
	"fmt"
	"slices"

	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/search"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------------- fileJob ---------------------------------------
// ----------------------------------------------------------------------------------------

type fileJob struct{}

func (j *fileJob) Name() string { return "file-sync" }

func (j *fileJob) Run() error {
	logging.MarkSessionStart(logging.KeyFileSync)
	logging.LogDebug(logging.KeyFileSync, "running file cronjob")

	if err := git.PullRebase(); err != nil {
		logging.LogWarning(logging.KeyFileSync, "git pull failed: %v", err)
	}

	var filesToProcess []string
	var filesToDelete []string

	if _, err := git.CommitAllPending(); err != nil {
		logging.LogError(logging.KeyFileSync, "failed to commit pending changes: %v", err)
	}

	lastCommit, err := git.GetLastProcessedCommit()
	if err != nil {
		logging.LogError(logging.KeyFileSync, "failed to get last processed commit: %v", err)
	} else {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError(logging.KeyFileSync, "failed to get current commit: %v", err)
		} else if currentCommit != "" && currentCommit != lastCommit {
			hadError := false

			changedFiles, err := git.GetFilesChangedSinceCommit(lastCommit)
			if err != nil {
				logging.LogError(logging.KeyFileSync, "failed to get changed files: %v", err)
				hadError = true
			} else if len(changedFiles) > 0 {
				logging.LogInfo(logging.KeyFileSync, "detected %d files changed since last commit", len(changedFiles))
				filesToProcess = append(filesToProcess, changedFiles...)
			}

			deletedFiles, err := git.GetDeletedFilesSinceCommit(lastCommit)
			if err != nil {
				logging.LogError(logging.KeyFileSync, "failed to get deleted files: %v", err)
				hadError = true
			} else if len(deletedFiles) > 0 {
				logging.LogInfo(logging.KeyFileSync, "detected %d files deleted since last commit", len(deletedFiles))
				filesToDelete = append(filesToDelete, deletedFiles...)
			}

			movedFiles, err := git.GetFileRenames(lastCommit)
			if err != nil {
				logging.LogError(logging.KeyFileSync, "failed to get file renames: %v", err)
				hadError = true
			} else if len(movedFiles) > 0 {
				logging.LogInfo(logging.KeyFileSync, "detected %d file moves since last commit", len(movedFiles))
				for _, move := range movedFiles {
					oldNormalized := pathutils.ToWithPrefix(move.OldPath)
					newNormalized := pathutils.ToWithPrefix(move.NewPath)
					logging.LogInfo(logging.KeyFileSync, "processing file move: %s -> %s", oldNormalized, newNormalized)
					// no-refresh: this whole run ends with one RebuildAllCaches() below
					if err := files.UpdateLinksForMovedFileNoRefresh(logging.KeyFileSync, oldNormalized, newNormalized); err != nil {
						logging.LogError(logging.KeyFileSync, "failed to update links for moved file %s -> %s: %v", oldNormalized, newNormalized, err)
						// fall back to generic add/delete handling so the new path still gets metadata
						filesToProcess = append(filesToProcess, move.NewPath)
						filesToDelete = append(filesToDelete, move.OldPath)
					} else {
						logging.LogInfo(logging.KeyFileSync, "successfully updated links for moved file %s -> %s", oldNormalized, newNormalized)
					}
				}
			}

			if hadError {
				logging.LogWarning(logging.KeyFileSync, "not advancing last processed commit due to errors above, will retry next run")
			} else if err := git.SetLastProcessedCommit(currentCommit); err != nil {
				logging.LogError(logging.KeyFileSync, "failed to save last processed commit: %v", err)
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

	// build/extend the persisted deleted-files search index before wiping the
	// metadata below, so title/content search over deleted files can read the
	// index instead of walking the full commit log on every keystroke
	git.IndexDeletedFiles(lastCommit, filesToDelete)

	if len(filesToDelete) > 0 {
		logging.LogInfo(logging.KeyFileSync, "deleting metadata for %d files", len(filesToDelete))
		for _, filePath := range filesToDelete {
			normalizedPath := pathutils.ToWithPrefix(filePath)
			if err := files.MetaDataDeleteNoRefresh(logging.KeyFileSync, normalizedPath); err != nil {
				logging.LogError(logging.KeyFileSync, "failed to delete metadata for %s: %v", normalizedPath, err)
				continue
			}
			logging.LogDebug(logging.KeyFileSync, "deleted metadata for %s", normalizedPath)
		}
	}

	if len(filesToProcess) == 0 {
		logging.LogDebug(logging.KeyFileSync, "no files to process")
	} else {
		logging.LogInfo(logging.KeyFileSync, "processing %d files", len(filesToProcess))
		for _, filePath := range filesToProcess {
			normalizedPath := pathutils.ToWithPrefix(filePath)
			metadata := &files.Metadata{
				Path:   normalizedPath,
				Editor: files.EditorTypeToastUI,
			}
			if err := files.MetaDataSaveNoRefresh(metadata); err != nil {
				logging.LogError(logging.KeyFileSync, "failed to save metadata for %s: %v", normalizedPath, err)
				continue
			}
			logging.LogDebug(logging.KeyFileSync, "processed metadata for %s", normalizedPath)
		}
	}

	if err := files.RebuildAllCaches(); err != nil {
		logging.LogError(logging.KeyFileSync, "failed to save system data to cache: %v", err)
	}

	// run filter index as a sub-step so it gets its own history entry
	execute(&filterMu, &filterJob{})

	logging.LogDebug(logging.KeyFileSync, "file cronjob completed")
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
	logging.MarkSessionStart(logging.KeySearchReindex)
	logging.LogDebug(logging.KeySearchReindex, "running search index cronjob")
	if err := search.IndexAllFiles(); err != nil {
		return fmt.Errorf("failed to reindex search: %w", err)
	}
	logging.LogDebug(logging.KeySearchReindex, "search index cronjob completed")
	return nil
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- rebuildJob -------------------------------------
// ----------------------------------------------------------------------------------------

// rebuildJob is the lightweight scheduled rebuild (links only).
type rebuildJob struct{}

func (j *rebuildJob) Name() string { return "metadata-links-rebuild" }

func (j *rebuildJob) Run() error {
	logging.MarkSessionStart(logging.KeyMetadataRebuild)
	logging.LogDebug(logging.KeyMetadataRebuild, "running metadata rebuild cronjob")
	if err := files.MetaDataLinksRebuild(logging.KeyMetadataRebuild); err != nil {
		return fmt.Errorf("metadata rebuild failed: %w", err)
	}
	logging.LogDebug(logging.KeyMetadataRebuild, "metadata rebuild cronjob completed")
	return nil
}
