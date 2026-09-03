// Package job handles periodic maintenance tasks.
package job

import (
	"context"
	"fmt"
	"slices"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/search"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------------- fileJob ---------------------------------------
// ----------------------------------------------------------------------------------------

type fileJob struct{}

func (j *fileJob) Name() string { return JobTypeFileSync }

// StartFileSyncManual starts the file-sync job in the background via StartAsync, for UI
// triggers that need to poll for completion (e.g. the kanban board's sync button) instead of
// blocking the request for however long a run takes. Not registered as resumable - if the
// process crashes mid-run, the scheduled tick just picks the same incremental work back up on
// its own next run, so there's nothing to recover.
func StartFileSyncManual() (string, error) {
	return StartAsync(&fileMu, &fileJob{}, "")
}

func (j *fileJob) Run(_ context.Context) error {
	logging.MarkSessionStart(logging.KeyFileSync)
	logging.LogDebug(logging.KeyFileSync, "running file cronjob")

	if err := git.PullRebase(); err != nil {
		logging.LogWarning(logging.KeyFileSync, "git pull failed: %v", err)
	}

	var filesToProcess []string
	var filesToDelete []string
	var haveNewCommits bool // whether a new commit range was detected below, see syncTime

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
			haveNewCommits = true

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
						// foldersync: a physical move into a status folder sets the kanban tag to
						// match - changedAt lets it defer to a more recent tag change instead of
						// fighting it
						if changedAt, _, err := git.GetCommitDetails(move.Commit); err != nil {
							logging.LogWarning(logging.KeyFileSync, "failed to get commit time for %s, skipping kanban foldersync: %v", move.Commit, err)
						} else if err := kanban.SyncFolderTag(newNormalized, changedAt); err != nil {
							logging.LogWarning(logging.KeyFileSync, "kanban foldersync failed for %s: %v", newNormalized, err)
						}
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
		// single lookup shared by every file below - foldersync: a file that showed up (new or
		// edited) directly inside a status folder gets that status's tag, e.g. a file created
		// straight in board/archive/ picks up the archive tag without ever being dragged there.
		// Uses lastCommit (the range's lower bound, before any of these files changed) rather
		// than currentCommit: filesToProcess can span several commits when the cronjob catches
		// up after a gap, and using a timestamp that's too late would make a manual tag edit
		// made in between look older than the physical change and get overwritten by it. A
		// timestamp that's too early only means the reverse - a real physical change occasionally
		// not applied yet - which self-corrects on the next run instead of clobbering anything.
		var syncTime time.Time
		if haveNewCommits && lastCommit != "" {
			if t, _, err := git.GetCommitDetails(lastCommit); err != nil {
				logging.LogWarning(logging.KeyFileSync, "failed to get commit time for kanban foldersync: %v", err)
			} else {
				syncTime = t
			}
		}
		for _, filePath := range filesToProcess {
			normalizedPath := pathutils.ToWithPrefix(filePath)
			// Sync only fills the default editor when the field is empty - unlike the old
			// blind save, it can't clobber a user-picked editor on a file edited externally
			if err := files.MetaDataSyncNoRefresh(normalizedPath); err != nil {
				logging.LogError(logging.KeyFileSync, "failed to save metadata for %s: %v", normalizedPath, err)
				continue
			}
			if !syncTime.IsZero() {
				if err := kanban.SyncFolderTag(normalizedPath, syncTime); err != nil {
					logging.LogWarning(logging.KeyFileSync, "kanban foldersync failed for %s: %v", normalizedPath, err)
				}
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

func (j *searchIndexJob) Run(_ context.Context) error {
	logging.MarkSessionStart(logging.KeySearchReindex)
	if configmanager.GetSearchEngine() == "grep" {
		logging.LogDebug(logging.KeySearchReindex, "grep search engine active, skipping index")
		return nil
	}
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

func (j *rebuildJob) Run(ctx context.Context) error {
	logging.MarkSessionStart(logging.KeyMetadataRebuild)
	logging.LogDebug(logging.KeyMetadataRebuild, "running metadata rebuild cronjob")
	if err := files.MetaDataLinksRebuild(ctx, logging.KeyMetadataRebuild); err != nil {
		return fmt.Errorf("metadata rebuild failed: %w", err)
	}
	logging.LogDebug(logging.KeyMetadataRebuild, "metadata rebuild cronjob completed")
	return nil
}
