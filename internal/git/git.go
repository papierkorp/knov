// Package git - Git operations for file management
package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"

	"sync"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitHistoryFile represents a file in git history
type GitHistoryFile struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Commit  string    `json:"commit"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
}

// FileVersion represents a single version of a file
type FileVersion struct {
	Commit    string    `json:"commit"`
	Date      time.Time `json:"date"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	IsCurrent bool      `json:"is_current"`
}

// FileVersionList is a list of file versions
type FileVersionList []FileVersion

// FileMove represents a file that was moved/renamed
type FileMove struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
	Commit  string `json:"commit"`
}

// openRepo opens the git repository
func openRepo() (*git.Repository, error) {
	dataDir := configmanager.GetAppConfig().DataPath
	return git.PlainOpen(dataDir)
}

// EnsureRepoConfig sets local git config options that should always be present.
// Called once at startup. Safe to call on repos knov didn't create.
func EnsureRepoConfig() {
	repo, err := openRepo()
	if err != nil {
		return // no repo yet, nothing to configure
	}

	cfg, err := repo.Config()
	if err != nil {
		logging.LogWarning("git: failed to read repo config: %v", err)
		return
	}

	// Windows filesystems don't track the executable bit; without this flag
	// go-git treats every file as "mode changed" and produces spurious commits.
	if cfg.Raw.Section("core").Option("filemode") != "false" {
		cfg.Raw.Section("core").SetOption("filemode", "false")
		if err := repo.SetConfig(cfg); err != nil {
			logging.LogWarning("git: failed to set core.filemode=false: %v", err)
		} else {
			logging.LogInfo("git: set core.filemode=false in repo config")
		}
	}
}

// GetRecentlyChangedFiles returns recently changed unique files with pagination.
// count is the number of unique files to return; offset skips that many unique files first.
func GetRecentlyChangedFiles(count, offset int) ([]GitHistoryFile, error) {
	repo, err := openRepo()
	if err != nil {
		logging.LogError("failed to open git repository: %v", err)
		return nil, err
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var files []GitHistoryFile
	seen := make(map[string]bool)
	skipped := 0
	dataDir := configmanager.GetAppConfig().DataPath
	dataDirName := filepath.Base(dataDir)

	for {
		if len(files) >= count {
			break
		}

		c, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		currentTree, err := c.Tree()
		if err != nil {
			continue
		}

		var changedPaths []string
		if c.NumParents() == 0 {
			// initial commit — list all files in tree
			_ = currentTree.Files().ForEach(func(f *object.File) error {
				changedPaths = append(changedPaths, f.Name)
				return nil
			})
		} else {
			parent, err := c.Parent(0)
			if err != nil {
				continue
			}
			parentTree, err := parent.Tree()
			if err != nil {
				continue
			}
			// DiffTree is lazy — no patch computation, just file names
			changes, err := object.DiffTree(parentTree, currentTree)
			if err != nil {
				continue
			}
			for _, change := range changes {
				name := change.To.Name
				if name == "" {
					name = change.From.Name
				}
				if name != "" {
					changedPaths = append(changedPaths, name)
				}
			}
		}

		for _, relPath := range changedPaths {
			if strings.HasPrefix(relPath, dataDirName+string(filepath.Separator)) {
				relPath = strings.TrimPrefix(relPath, dataDirName+string(filepath.Separator))
			}
			if seen[relPath] {
				continue
			}
			seen[relPath] = true
			if skipped < offset {
				skipped++
				continue
			}
			files = append(files, GitHistoryFile{
				Name:    filepath.Base(relPath),
				Path:    relPath,
				Commit:  c.Hash.String()[:7],
				Date:    c.Author.When,
				Message: c.Message,
			})
			if len(files) >= count {
				break
			}
		}
	}

	return files, nil
}

// GetUntrackedFiles returns list of untracked files in git
func GetUntrackedFiles() ([]string, error) {
	repo, err := openRepo()
	if err != nil {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}

	var untrackedFiles []string
	for file, fileStatus := range status {
		if fileStatus.Staging == git.Untracked && fileStatus.Worktree == git.Untracked {
			untrackedFiles = append(untrackedFiles, file)
		}
	}

	return untrackedFiles, nil
}

// AddNewFiles adds all untracked files in the data directory to git
func AddNewFiles() ([]string, error) {
	untrackedFiles, err := GetUntrackedFiles()
	if err != nil {
		return nil, err
	}

	if len(untrackedFiles) == 0 {
		logging.LogDebug("no new files to add")
		return nil, nil
	}

	logging.LogInfo("found %d untracked files", len(untrackedFiles))

	SyncBeforeCommit(untrackedFiles)

	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	// add all files
	_, err = worktree.Add(".")
	if err != nil {
		logging.LogError("failed to add files to git: %v", err)
		return nil, err
	}

	// commit changes
	_, err = worktree.Commit("auto-commit: new files added", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		logging.LogError("failed to commit files: %v", err)
		return nil, err
	}

	logging.LogInfo("auto-committed %d new files to git", len(untrackedFiles))
	Push()
	return untrackedFiles, nil
}

// CommitDeletedFiles commits all deleted files
func CommitDeletedFiles(deletedFiles []string) error {
	if len(deletedFiles) == 0 {
		return nil
	}

	SyncBeforeCommit(deletedFiles)

	repo, err := openRepo()
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// add deletions to staging
	for _, file := range deletedFiles {
		_, err = worktree.Add(file)
		if err != nil {
			logging.LogError("failed to stage deleted file %s: %v", file, err)
			continue
		}
	}

	// commit deletions
	_, err = worktree.Commit("auto-commit: files deleted", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		logging.LogError("failed to commit deleted files: %v", err)
		return err
	}

	logging.LogInfo("auto-committed %d deleted files to git", len(deletedFiles))
	Push()
	return nil
}

// CommitFile syncs with remote and commits a single file immediately on save.
// This is called after every editor save to minimise the conflict window.
// Non-blocking push happens in the background after the commit.
func CommitFile(fullPath string) {
	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, fullPath)
	if err != nil {
		logging.LogError("git: failed to get relative path for %s: %v", fullPath, err)
		return
	}

	if remoteEnabled() {
		SyncBeforeCommit([]string{fullPath})
	}

	repo, err := openRepo()
	if err != nil {
		logging.LogError("git: failed to open repo for commit of %s: %v", relPath, err)
		return
	}

	worktree, err := repo.Worktree()
	if err != nil {
		logging.LogError("git: failed to get worktree for commit of %s: %v", relPath, err)
		return
	}

	if _, err := worktree.Add(relPath); err != nil {
		logging.LogError("git: failed to stage %s: %v", relPath, err)
		return
	}

	_, err = worktree.Commit(fmt.Sprintf("save: %s", relPath), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		if err.Error() == "nothing to commit, working tree clean" || err.Error() == "cannot create empty commit: clean working tree" {
			logging.LogDebug("git: nothing to commit for %s", relPath)
			return
		}
		logging.LogError("git: failed to commit %s: %v", relPath, err)
		return
	}

	logging.LogInfo("git: committed %s", relPath)
	Push()
}

// CommitDeletedFile stages and commits a single deleted file immediately.
// Called after in-app file deletion so the deletion is recorded without waiting for the cronjob.
func CommitDeletedFile(fullPath string) {
	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, fullPath)
	if err != nil {
		logging.LogError("git: failed to get relative path for %s: %v", fullPath, err)
		return
	}

	if remoteEnabled() {
		SyncBeforeCommit([]string{fullPath})
	}

	repo, err := openRepo()
	if err != nil {
		logging.LogError("git: failed to open repo for delete commit of %s: %v", relPath, err)
		return
	}

	worktree, err := repo.Worktree()
	if err != nil {
		logging.LogError("git: failed to get worktree for delete commit of %s: %v", relPath, err)
		return
	}

	if _, err := worktree.Add(relPath); err != nil {
		logging.LogError("git: failed to stage deleted file %s: %v", relPath, err)
		return
	}

	_, err = worktree.Commit(fmt.Sprintf("delete: %s", relPath), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		if err.Error() == "nothing to commit, working tree clean" || err.Error() == "cannot create empty commit: clean working tree" {
			logging.LogDebug("git: nothing to commit for deleted file %s", relPath)
			return
		}
		logging.LogError("git: failed to commit deleted file %s: %v", relPath, err)
		return
	}

	logging.LogInfo("git: committed deletion of %s", relPath)
	Push()
}

// CommitModifiedFiles commits all modified files
func CommitModifiedFiles(modifiedFiles []string) error {
	if len(modifiedFiles) == 0 {
		return nil
	}

	SyncBeforeCommit(modifiedFiles)

	repo, err := openRepo()
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// add modified files to staging
	for _, file := range modifiedFiles {
		_, err = worktree.Add(file)
		if err != nil {
			logging.LogError("failed to stage modified file %s: %v", file, err)
			continue
		}
	}

	// commit modifications
	_, err = worktree.Commit("auto-commit: files modified", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		logging.LogError("failed to commit modified files: %v", err)
		return err
	}

	logging.LogInfo("auto-committed %d modified files to git", len(modifiedFiles))
	Push()
	return nil
}

// CommitAllPending stages every pending change (modified, deleted, untracked)
// using the equivalent of "git add -A" and commits them.
// Returns true when a commit was actually made, false when the tree was already clean.
func CommitAllPending() (bool, error) {
	repo, err := openRepo()
	if err != nil {
		logging.LogDebug("git: no repository found, skipping commit")
		return false, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	// collect dirty files for the commit message and to snapshot before SyncBeforeCommit
	// can hard-reset them away when a remote is configured
	var localFiles []string
	var relPaths []string
	dataDir := configmanager.GetAppConfig().DataPath
	if status, err := worktree.Status(); err == nil {
		for relPath, s := range status {
			if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
				localFiles = append(localFiles, filepath.Join(dataDir, relPath))
				relPaths = append(relPaths, relPath)
			}
		}
	}
	SyncBeforeCommit(localFiles)

	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return false, fmt.Errorf("git add -A failed: %w", err)
	}

	const fileListLimit = 5
	var commitMsg string
	switch {
	case len(relPaths) == 0:
		commitMsg = "auto-commit: external changes"
	case len(relPaths) <= fileListLimit:
		commitMsg = "auto-commit: " + strings.Join(relPaths, ", ")
	default:
		commitMsg = fmt.Sprintf("auto-commit: %d files modified externally", len(relPaths))
	}

	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") || strings.Contains(err.Error(), "clean working tree") {
			logging.LogDebug("git: nothing to commit")
			return false, nil
		}
		return false, fmt.Errorf("git commit failed: %w", err)
	}

	logging.LogInfo("git: auto-committed pending changes")
	Push()
	return true, nil
}

// GetFileHistory returns the git history for a specific file
func GetFileHistory(filePath string) ([]FileVersion, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	headCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	var versions []FileVersion
	currentPath := relPath
	fromCommit := headCommit
	visited := make(map[string]bool)

	// walk the file's history segment by segment, following renames across
	// commit boundaries the same way `git log --follow` does - go-git's
	// LogOptions has no built-in --follow support
	for {
		iter, err := repo.Log(&git.LogOptions{
			From:     fromCommit.Hash,
			FileName: &currentPath,
		})
		if err != nil {
			return nil, err
		}

		var lastCommit *object.Commit
		err = iter.ForEach(func(c *object.Commit) error {
			versions = append(versions, FileVersion{
				Commit:    c.Hash.String()[:7],
				Date:      c.Author.When,
				Message:   c.Message,
				Author:    c.Author.Name,
				IsCurrent: false,
			})
			lastCommit = c
			return nil
		})
		iter.Close()
		if err != nil {
			return nil, err
		}

		if lastCommit == nil || len(lastCommit.ParentHashes) == 0 {
			break
		}

		parent, err := repo.CommitObject(lastCommit.ParentHashes[0])
		if err != nil {
			break
		}

		moves, err := detectMovesInCommit(repo, parent, lastCommit)
		if err != nil {
			logging.LogWarning("failed to detect renames for %s at commit %s: %v", currentPath, lastCommit.Hash.String()[:7], err)
			break
		}

		oldPath := ""
		for _, m := range moves {
			if m.NewPath == currentPath {
				oldPath = m.OldPath
				break
			}
		}
		if oldPath == "" || visited[oldPath] {
			break
		}

		visited[oldPath] = true
		currentPath = oldPath
		fromCommit = parent
	}

	// mark the first (most recent) as current
	if len(versions) > 0 {
		versions[0].IsCurrent = true
	}

	return versions, nil
}

// GetCurrentCommit returns the current HEAD commit hash
func GetCurrentCommit() (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", nil
	}

	ref, err := repo.Head()
	if err != nil {
		logging.LogError("failed to get current commit: %v", err)
		return "", err
	}

	return ref.Hash().String(), nil
}

// GetFilesChangedSinceCommit returns files that changed since a specific commit
func GetFilesChangedSinceCommit(lastCommit string) ([]string, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	if lastCommit == "" {
		// if no last commit, return all files
		ref, err := repo.Head()
		if err != nil {
			return nil, err
		}

		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return nil, err
		}

		tree, err := commit.Tree()
		if err != nil {
			return nil, err
		}

		var allFiles []string
		err = tree.Files().ForEach(func(f *object.File) error {
			allFiles = append(allFiles, f.Name)
			return nil
		})

		return allFiles, err
	}

	// validate that the commit exists before using it
	if !CommitExists(lastCommit) {
		logging.LogWarning("commit %s no longer exists, resetting to process all files", lastCommit)
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		return GetFilesChangedSinceCommit("")
	}

	// get current HEAD
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	lastCommitHash, err := expandCommitHash(repo, lastCommit)
	if err != nil {
		logging.LogWarning("commit %s no longer exists, resetting to process all files: %v", lastCommit, err)
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		return GetFilesChangedSinceCommit("")
	}

	headCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	lastCommitObj, err := repo.CommitObject(lastCommitHash)
	if err != nil {
		return nil, err
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	lastTree, err := lastCommitObj.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := object.DiffTree(lastTree, headTree)
	if err != nil {
		return nil, err
	}

	var changedFiles []string
	for _, change := range changes {
		name := change.To.Name
		if name == "" {
			name = change.From.Name
		}
		if name != "" {
			changedFiles = append(changedFiles, name)
		}
	}

	return changedFiles, nil
}

// GetDeletedFilesSinceCommit returns files that were deleted since a specific commit
func GetDeletedFilesSinceCommit(lastCommit string) ([]string, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	if lastCommit == "" {
		return nil, nil
	}

	if !CommitExists(lastCommit) {
		logging.LogWarning("commit %s no longer exists, cannot check for deleted files", lastCommit)
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		return nil, nil
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	lastCommitHash, err := expandCommitHash(repo, lastCommit)
	if err != nil {
		logging.LogWarning("commit %s no longer exists, cannot check for deleted files: %v", lastCommit, err)
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		return nil, nil
	}

	headCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	lastCommitObj, err := repo.CommitObject(lastCommitHash)
	if err != nil {
		return nil, err
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	lastTree, err := lastCommitObj.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := object.DiffTree(lastTree, headTree)
	if err != nil {
		return nil, err
	}

	var deletedFiles []string
	for _, change := range changes {
		if change.To.Name == "" && change.From.Name != "" {
			deletedFiles = append(deletedFiles, change.From.Name)
		}
	}

	return deletedFiles, nil
}

// GetUncommittedDeletedFiles returns files that are deleted but not yet committed
func GetUncommittedDeletedFiles() ([]string, error) {
	repo, err := openRepo()
	if err != nil {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}

	var deletedFiles []string
	for file, fileStatus := range status {
		if fileStatus.Staging == git.Deleted || fileStatus.Worktree == git.Deleted {
			deletedFiles = append(deletedFiles, file)
		}
	}

	return deletedFiles, nil
}

// GetModifiedFiles returns files that are modified but not yet committed
func GetModifiedFiles() ([]string, error) {
	repo, err := openRepo()
	if err != nil {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}

	var modifiedFiles []string
	for file, fileStatus := range status {
		if fileStatus.Staging == git.Modified || fileStatus.Worktree == git.Modified {
			modifiedFiles = append(modifiedFiles, file)
		}
	}

	return modifiedFiles, nil
}

// RestoreFileToCommit restores a file to a specific commit
func RestoreFileToCommit(filePath, commit string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	commitHash, err := expandCommitHash(repo, commit)
	if err != nil {
		logging.LogError("failed to find commit %s: %v", commit, err)
		return err
	}

	commitObj, err := repo.CommitObject(commitHash)
	if err != nil {
		logging.LogError("failed to get commit %s: %v", commit, err)
		return err
	}

	tree, err := commitObj.Tree()
	if err != nil {
		return err
	}

	file, err := tree.File(relPath)
	if err != nil {
		// file may have been deleted in this commit — try parent commit tree
		if len(commitObj.ParentHashes) > 0 {
			parentObj, parentErr := repo.CommitObject(commitObj.ParentHashes[0])
			if parentErr == nil {
				parentTree, parentErr := parentObj.Tree()
				if parentErr == nil {
					file, err = parentTree.File(relPath)
				}
			}
		}
		if err != nil {
			logging.LogError("failed to get file %s from commit %s: %v", relPath, commit, err)
			return err
		}
	}

	content, err := file.Contents()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return err
	}

	// add and commit the restored file
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = worktree.Add(relPath)
	if err != nil {
		logging.LogError("failed to add restored file %s: %v", relPath, err)
		return err
	}

	commitMessage := fmt.Sprintf("restore %s to commit %s", relPath, commit)
	_, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
	})
	if err != nil {
		logging.LogError("failed to commit restored file %s: %v", relPath, err)
		return err
	}

	logging.LogInfo("restored file %s to commit %s and logged the change", relPath, commit)
	return nil
}

// GetCommitDetails returns details for a specific commit
func GetCommitDetails(commit string) (time.Time, string, error) {
	repo, err := openRepo()
	if err != nil {
		return time.Time{}, "", err
	}

	commitHash, err := expandCommitHash(repo, commit)
	if err != nil {
		logging.LogError("failed to find commit %s: %v", commit, err)
		return time.Time{}, "", err
	}

	commitObj, err := repo.CommitObject(commitHash)
	if err != nil {
		logging.LogError("failed to get commit details for %s: %v", commit, err)
		return time.Time{}, "", err
	}

	return commitObj.Author.When, strings.TrimSpace(commitObj.Message), nil
}

// CommitExists checks if a commit hash exists in the repository
func CommitExists(commit string) bool {
	if commit == "" {
		return false
	}

	repo, err := openRepo()
	if err != nil {
		return false
	}

	// Handle short commit hashes by expanding to full hash
	fullHash, err := expandCommitHash(repo, commit)
	if err != nil {
		return false
	}

	_, err = repo.CommitObject(fullHash)
	return err == nil
}

// expandCommitHash expands a short commit hash to a full hash
func expandCommitHash(repo *git.Repository, shortHash string) (plumbing.Hash, error) {
	// If it's already a full hash, return it
	if len(shortHash) == 40 {
		return plumbing.NewHash(shortHash), nil
	}

	// Get all commit objects and find matching prefix
	iter, err := repo.CommitObjects()
	if err != nil {
		return plumbing.Hash{}, err
	}
	defer iter.Close()

	var matchedHash plumbing.Hash
	matchCount := 0

	err = iter.ForEach(func(c *object.Commit) error {
		if strings.HasPrefix(c.Hash.String(), shortHash) {
			matchedHash = c.Hash
			matchCount++
			if matchCount > 1 {
				return fmt.Errorf("ambiguous commit hash: %s", shortHash)
			}
		}
		return nil
	})

	if err != nil {
		return plumbing.Hash{}, err
	}

	if matchCount == 0 {
		return plumbing.Hash{}, fmt.Errorf("commit not found: %s", shortHash)
	}

	return matchedHash, nil
}

const lastCommitKey = "last-processed-commit"

// GetLastProcessedCommit returns the last commit that was processed for metadata
func GetLastProcessedCommit() (string, error) {
	data, err := configStorage.Get(lastCommitKey)
	if err != nil || data == nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetLastProcessedCommit saves the last processed commit hash
func SetLastProcessedCommit(commitHash string) error {
	return configStorage.Set(lastCommitKey, []byte(commitHash))
}

// GetFileAtCommit returns the content of a file at a specific commit
func GetFileAtCommit(filePath, commit string) (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	// Expand short commit hash to full hash
	commitHash, err := expandCommitHash(repo, commit)
	if err != nil {
		logging.LogWarning("commit %s does not exist for file %s: %v", commit, filePath, err)
		return "", fmt.Errorf("commit %s not found", commit)
	}

	commitObj, err := repo.CommitObject(commitHash)
	if err != nil {
		return "", err
	}

	tree, err := commitObj.Tree()
	if err != nil {
		return "", err
	}

	file, err := tree.File(relPath)
	if err != nil {
		// file may have been deleted in this commit — try parent commit tree
		if len(commitObj.ParentHashes) > 0 {
			parentObj, parentErr := repo.CommitObject(commitObj.ParentHashes[0])
			if parentErr == nil {
				parentTree, parentErr := parentObj.Tree()
				if parentErr == nil {
					file, err = parentTree.File(relPath)
				}
			}
		}
		if err != nil {
			logging.LogError("failed to get file %s at commit %s: %v", relPath, commit, err)
			return "", err
		}
	}

	content, err := file.Contents()
	if err != nil {
		return "", err
	}

	return content, nil
}

// GetFileDiff returns the diff between two commits for a file
func GetFileDiff(filePath, fromCommit, toCommit string) (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	// Handle "previous" parameter
	if toCommit == "previous" {
		// Get the parent of fromCommit
		fromHash, err := expandCommitHash(repo, fromCommit)
		if err != nil {
			return "", fmt.Errorf("failed to find commit %s: %v", fromCommit, err)
		}

		fromCommitObj, err := repo.CommitObject(fromHash)
		if err != nil {
			return "", err
		}

		parents := fromCommitObj.Parents()
		defer parents.Close()

		parentCommit, err := parents.Next()
		if err != nil {
			// No parent commit (probably the initial commit)
			return "", fmt.Errorf("no parent commit found for %s", fromCommit)
		}

		toCommit = parentCommit.Hash.String()
	}

	// Expand commit hashes
	fromCommitHash, err := expandCommitHash(repo, fromCommit)
	if err != nil {
		return "", fmt.Errorf("failed to find commit %s: %v", fromCommit, err)
	}

	toCommitHash, err := expandCommitHash(repo, toCommit)
	if err != nil {
		return "", fmt.Errorf("failed to find commit %s: %v", toCommit, err)
	}

	fromCommitObj, err := repo.CommitObject(fromCommitHash)
	if err != nil {
		return "", err
	}

	toCommitObj, err := repo.CommitObject(toCommitHash)
	if err != nil {
		return "", err
	}

	fromTree, err := fromCommitObj.Tree()
	if err != nil {
		return "", err
	}

	toTree, err := toCommitObj.Tree()
	if err != nil {
		return "", err
	}

	// relPath reflects the file's current (HEAD) name. If it was renamed at
	// some point between HEAD and either side of this comparison, resolve
	// what it was actually called there - otherwise the tree diff below sees
	// two unrelated paths and reports the file as deleted rather than diffing
	// its content.
	fromRelPath := relPath
	toRelPath := relPath
	if headRef, err := repo.Head(); err == nil {
		if headCommit, err := repo.CommitObject(headRef.Hash()); err == nil {
			fromRelPath = resolvePathAtCommit(repo, relPath, headCommit, fromCommitObj)
			toRelPath = resolvePathAtCommit(repo, relPath, headCommit, toCommitObj)
		}
	}

	if fromRelPath != toRelPath {
		fromEntry, fromErr := fromTree.FindEntry(fromRelPath)
		toEntry, toErr := toTree.FindEntry(toRelPath)
		if fromErr == nil && toErr == nil {
			change := &object.Change{
				From: object.ChangeEntry{Name: fromRelPath, Tree: fromTree, TreeEntry: *fromEntry},
				To:   object.ChangeEntry{Name: toRelPath, Tree: toTree, TreeEntry: *toEntry},
			}
			patch, err := change.Patch()
			if err != nil {
				return "", err
			}
			return patch.String(), nil
		}
	}

	changes, err := object.DiffTree(fromTree, toTree)
	if err != nil {
		return "", err
	}

	for _, change := range changes {
		if change.To.Name == relPath || change.From.Name == relPath {
			patch, err := change.Patch()
			if err != nil {
				return "", err
			}
			return patch.String(), nil
		}
	}

	return "", fmt.Errorf("no changes found for file %s between commits", relPath)
}

// resolvePathAtCommit walks backward from `from` toward `target` along first
// parents, applying the same rename detection used by GetFileRenames, and
// returns the path the file had at `target`. Falls back to `path` unchanged
// if `target` isn't reached (e.g. it's not an ancestor of `from`).
func resolvePathAtCommit(repo *git.Repository, path string, from, target *object.Commit) string {
	current := path
	cur := from
	for cur.Hash != target.Hash {
		if len(cur.ParentHashes) == 0 {
			return current
		}

		parent, err := repo.CommitObject(cur.ParentHashes[0])
		if err != nil {
			return current
		}

		moves, err := detectMovesInCommit(repo, parent, cur)
		if err == nil {
			for _, m := range moves {
				if m.NewPath == current {
					current = m.OldPath
					break
				}
			}
		}

		cur = parent
	}

	return current
}

// GetFileRenames returns files that were moved/renamed since a specific commit
func GetFileRenames(lastCommit string) ([]FileMove, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	if lastCommit == "" {
		logging.LogDebug("no last commit provided, checking recent renames")
		// if no last commit, check recent commits for renames
		ref, err := repo.Head()
		if err != nil {
			return nil, err
		}

		commits, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			return nil, err
		}

		var renames []FileMove
		commitCount := 0
		err = commits.ForEach(func(commit *object.Commit) error {
			if commitCount >= 10 { // limit to recent 10 commits
				return nil
			}
			commitCount++

			if len(commit.ParentHashes) == 0 {
				return nil // initial commit
			}

			parent, err := repo.CommitObject(commit.ParentHashes[0])
			if err != nil {
				return err
			}

			moves, err := detectMovesInCommit(repo, parent, commit)
			if err != nil {
				logging.LogWarning("failed to detect moves in commit %s: %v", commit.Hash.String()[:7], err)
				return nil
			}

			renames = append(renames, moves...)
			return nil
		})

		return renames, err
	}

	// check for renames since specific commit
	if !CommitExists(lastCommit) {
		logging.LogWarning("commit %s no longer exists, cannot check for renames", lastCommit)
		return nil, fmt.Errorf("commit %s not found", lastCommit)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	lastCommitHash, err := expandCommitHash(repo, lastCommit)
	if err != nil {
		return nil, err
	}

	lastCommitObj, err := repo.CommitObject(lastCommitHash)
	if err != nil {
		return nil, err
	}

	headCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return detectMovesInCommit(repo, lastCommitObj, headCommit)
}

// detectMovesInCommit detects file moves between two commits
func detectMovesInCommit(repo *git.Repository, fromCommit, toCommit *object.Commit) ([]FileMove, error) {
	fromTree, err := fromCommit.Tree()
	if err != nil {
		return nil, err
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := object.DiffTree(fromTree, toTree)
	if err != nil {
		return nil, err
	}

	var renames []FileMove
	deletedFiles := make(map[string]bool)
	addedFiles := make(map[string]string) // path -> hash

	// first pass: collect deletions and additions
	for _, change := range changes {
		switch change.To.Name {
		case "":
			// deletion
			deletedFiles[change.From.Name] = true
		default:
			if change.From.Name == "" {
				// addition
				addedFiles[change.To.Name] = change.To.TreeEntry.Hash.String()
			}
		}
	}

	// second pass: match deletions with additions by content hash
	for _, change := range changes {
		if change.To.Name == "" && deletedFiles[change.From.Name] {
			// this is a deleted file, look for matching addition
			oldHash := change.From.TreeEntry.Hash.String()

			for addedPath, addedHash := range addedFiles {
				if oldHash == addedHash {
					// found a match - this is a rename
					renames = append(renames, FileMove{
						OldPath: change.From.Name,
						NewPath: addedPath,
						Commit:  toCommit.Hash.String(),
					})

					// remove from tracking to avoid duplicate matches
					delete(addedFiles, addedPath)
					deletedFiles[change.From.Name] = false
					break
				}
			}
		}
	}

	return renames, nil
}

// -----------------------------------------------
// ----------- Remote sync (optional) -----------
// -----------------------------------------------

var (
	gitLogOnce sync.Once
	gitLogInst *log.Logger
)

func gitLog() *log.Logger {
	gitLogOnce.Do(func() {
		gitLogInst = logging.LogBuilder("git-remote")
	})
	return gitLogInst
}

// remoteEnabled returns true when a remote is configured.
func remoteEnabled() bool {
	return configmanager.GetGitRemote() != ""
}

// buildAuth returns the appropriate auth method based on the remote URL.
// For SSH URLs: uses the system SSH agent.
// For HTTPS URLs: uses BasicAuth with user/token or user/password.
// Returns nil if no credentials are configured (e.g. public repo or SSH with key file).
func buildAuth() (transport.AuthMethod, error) {
	remote := configmanager.GetGitRemote()

	// SSH URL (git@host:user/repo or ssh://...)
	if strings.HasPrefix(remote, "git@") || strings.HasPrefix(remote, "ssh://") {
		// if explicit key path is configured, use it directly
		if keyPath := configmanager.GetGitSSHKey(); keyPath != "" {
			fileAuth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
			if err != nil {
				return nil, fmt.Errorf("git: failed to load SSH key %s: %w", keyPath, err)
			}
			return fileAuth, nil
		}

		// try SSH agent first
		auth, err := ssh.NewSSHAgentAuth("git")
		if err != nil {
			// SSH agent not available — fall back to default key files
			gitLog().Printf("SSH agent not available (%v), trying default keys", err)
			home, _ := os.UserHomeDir()
			for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
				keyPath := filepath.Join(home, ".ssh", name)
				if _, err2 := os.Stat(keyPath); err2 == nil {
					fileAuth, err2 := ssh.NewPublicKeysFromFile("git", keyPath, "")
					if err2 == nil {
						return fileAuth, nil
					}
				}
			}
			gitLog().Printf("no SSH auth available — set KNOV_GIT_SSH_KEY to your key path")
			return nil, nil
		}
		return auth, nil
	}

	// HTTPS URL — use token or password
	user, password := configmanager.GetGitAuth()
	if user == "" && password == "" {
		return nil, nil
	}
	return &githttp.BasicAuth{Username: user, Password: password}, nil
}

// parsePushTimeout parses the configured timeout string, defaulting to 10s.
func parsePushTimeout() time.Duration {
	d, err := time.ParseDuration(configmanager.GetGitPushTimeout())
	if err != nil || d <= 0 {
		return 10 * time.Second
	}
	return d
}

// PullRebase fetches and rebases the local branch onto the remote.
// Returns nil if remote is not configured or already up to date.
// Returns errConflict if a merge conflict is detected.
var errConflict = errors.New("merge conflict")

// PullRebase fetches the remote and fast-forwards the local branch.
// Used by the cronjob (no dirty working tree).
// Returns errConflict on merge conflicts, nil if already up to date or timed out.
func PullRebase() error {
	if !remoteEnabled() {
		return nil
	}

	repo, err := openRepo()
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	timeout := parsePushTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	branch := configmanager.GetGitRemoteBranch()
	remote := configmanager.GetGitRemote()

	auth, err := buildAuth()
	if err != nil {
		gitLog().Printf("pull: failed to build auth: %v", err)
	}

	err = worktree.PullContext(ctx, &git.PullOptions{
		RemoteName:    "origin",
		RemoteURL:     remote,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          auth,
		Force:         false,
	})

	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			gitLog().Printf("already up to date")
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			gitLog().Printf("pull timed out after %s — continuing with local commit", timeout)
			return nil // non-fatal: commit locally, push on next cycle
		}
		// check if it's a merge conflict
		if isConflicted() {
			return errConflict
		}
		return fmt.Errorf("git pull failed: %w", err)
	}

	gitLog().Printf("pulled from %s/%s", remote, branch)
	return nil
}

// fetchAndReset fetches remote changes and hard-resets to remote HEAD.
// Used by SyncBeforeCommit when the working tree may be dirty.
// Returns the set of relative paths changed by the incoming commits.
func fetchAndReset() (map[string]struct{}, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	branch := configmanager.GetGitRemoteBranch()
	remote := configmanager.GetGitRemote()

	auth, err := buildAuth()
	if err != nil {
		gitLog().Printf("fetch: failed to build auth: %v", err)
	}

	timeout := parsePushTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// fetch remote changes without touching the working tree
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RemoteURL:  remote,
		RefSpecs:   []gitcfg.RefSpec{gitcfg.RefSpec("refs/heads/" + branch + ":refs/remotes/origin/" + branch)},
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		if errors.Is(err, context.DeadlineExceeded) {
			gitLog().Printf("fetch timed out after %s — continuing with local commit", timeout)
			return nil, nil
		}
		return nil, fmt.Errorf("git fetch failed: %w", err)
	}
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		gitLog().Printf("fetch: already up to date")
		return nil, nil
	}

	// find which files changed between local HEAD and remote HEAD
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return nil, fmt.Errorf("fetch: failed to resolve remote ref: %w", err)
	}

	localRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("fetch: failed to resolve local HEAD: %w", err)
	}

	// collect files changed in each new incoming commit (local HEAD..remote HEAD)
	// we walk commit by commit so we only see what actually changed in new commits,
	// not the full tree diff which would include files from older diverged commits.
	changedFiles := map[string]struct{}{}
	if localRef.Hash() != remoteRef.Hash() {
		iter, err := repo.Log(&git.LogOptions{From: remoteRef.Hash()})
		if err == nil {
			_ = iter.ForEach(func(c *object.Commit) error {
				if c.Hash == localRef.Hash() {
					return fmt.Errorf("stop") // reached local HEAD, stop walking
				}
				stats, err := c.Stats()
				if err != nil {
					return nil
				}
				for _, s := range stats {
					changedFiles[s.Name] = struct{}{}
				}
				return nil
			})
			iter.Close()
		}
	}

	// hard reset to remote HEAD so working tree matches remote
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	if err := worktree.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	}); err != nil {
		return nil, fmt.Errorf("fetch: hard reset failed: %w", err)
	}

	gitLog().Printf("fetched and reset to remote %s/%s", remote, branch)
	keys := make([]string, 0, len(changedFiles))
	for k := range changedFiles {
		keys = append(keys, k)
	}
	logging.LogDebug("git: remotely changed files: %v", keys)
	return changedFiles, nil
}

// isConflicted checks if the repo has unresolved merge conflicts by looking for
// the MERGE_HEAD file that git creates during a conflicted merge.
func isConflicted() bool {
	dataDir := configmanager.GetAppConfig().DataPath
	_, err := os.Stat(filepath.Join(dataDir, ".git", "MERGE_HEAD"))
	return err == nil
}

// abortMerge removes the MERGE_HEAD file to clean up a failed merge state.
func abortMerge() {
	dataDir := configmanager.GetAppConfig().DataPath
	mergeHead := filepath.Join(dataDir, ".git", "MERGE_HEAD")
	if err := os.Remove(mergeHead); err != nil && !os.IsNotExist(err) {
		logging.LogWarning("git: failed to remove MERGE_HEAD: %v", err)
	}
	// also reset the index to HEAD to clean up staged conflict markers
	repo, err := openRepo()
	if err != nil {
		return
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return
	}
	_ = worktree.Reset(&git.ResetOptions{Mode: git.HardReset})
	logging.LogDebug("git: merge aborted, reset to HEAD")
}

// saveConflictCopy saves the current (local) content of a file as filename.conflict.YYYYMMDD-HHMMSS.md
// before the remote version overwrites it.
func saveConflictCopy(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	ext := filepath.Ext(filePath)
	base := filePath[:len(filePath)-len(ext)]
	ts := time.Now().Format("20060102-150405")
	conflictPath := base + ".conflict." + ts + ".md"

	if err := os.WriteFile(conflictPath, content, 0644); err != nil {
		return "", err
	}
	return conflictPath, nil
}

// HandleConflict is called when a merge conflict is detected.
// It saves the local conflicted files as .conflict.md copies, aborts the merge,
// and emits a flash notification for each conflict.
func HandleConflict(localFilePaths []string) {
	for _, path := range localFilePaths {
		// remove previous conflict file if one exists
		origMeta := pathutils.ToWithPrefix(path)
		if existingMeta, err := files.MetaDataGet(origMeta); err == nil && existingMeta != nil && existingMeta.ConflictFile != "" {
			existingConflictFull := pathutils.ToFullPath(pathutils.ToRelative(existingMeta.ConflictFile))
			os.Remove(existingConflictFull)
		}
		conflictPath, err := saveConflictCopy(path)
		if err != nil {
			logging.LogError("git conflict: failed to save conflict copy for %s: %v", path, err)
			continue
		}
		logging.LogWarning("git conflict: saved local version as %s", conflictPath)
		notificationStorage.Add("warning",
			fmt.Sprintf("conflict in %s — your version saved as %s", filepath.Base(path), filepath.Base(conflictPath)),
			true)
		conflictMeta := pathutils.ToWithPrefix(conflictPath)
		if err := files.SetConflictFile(origMeta, conflictMeta); err != nil {
			logging.LogWarning("git conflict: failed to update conflict metadata for %s: %v", origMeta, err)
		}
		if err := files.SetConflictOf(conflictMeta, origMeta); err != nil {
			logging.LogWarning("git conflict: failed to set conflictOf metadata for %s: %v", conflictMeta, err)
		}
	}
	abortMerge()
}

// SyncBeforeCommit fetches remote changes and resolves conflicts before committing.
// Uses fetch+reset instead of pull so the dirty working tree is handled safely:
//  1. snapshot our local file contents
//  2. fetch remote (no working tree change)
//  3. for any local file also changed remotely: save a .conflict.md copy, restore ours
//  4. hard-reset to remote HEAD (so our commit will be a fast-forward)
//  5. restore our file content (now on top of remote HEAD, ready to stage+commit)
func SyncBeforeCommit(localFiles []string) {
	if !remoteEnabled() {
		return
	}

	dataDir := configmanager.GetAppConfig().DataPath

	// 1. snapshot local contents before touching anything
	type snapshot struct {
		fullPath string
		relPath  string
		content  []byte
	}
	var snapshots []snapshot
	for _, f := range localFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		absData, _ := filepath.Abs(dataDir)
		absF, _ := filepath.Abs(f)
		rel, err := filepath.Rel(absData, absF)
		if err != nil {
			rel = f
		}
		snapshots = append(snapshots, snapshot{fullPath: f, relPath: filepath.ToSlash(rel), content: data})
	}

	logging.LogDebug("git: sync snapshots: %v", func() []string {
		r := make([]string, len(snapshots))
		for i, s := range snapshots {
			r[i] = s.relPath
		}
		return r
	}())

	// 2. fetch remote, get set of remotely-changed files, hard-reset to remote HEAD
	changedRemotely, err := fetchAndReset()
	if err != nil {
		logging.LogWarning("git sync before commit failed: %v — committing locally", err)
		return
	}
	if changedRemotely == nil {
		// already up to date or timed out — nothing to do
		return
	}

	// 3. for each local file: if remote also changed it, save B's version as .conflict.md.
	//    After fetchAndReset, testAB.md is already A's version (HEAD) on disk.
	//    We only write the conflict copy and leave testAB.md as-is.
	for _, snap := range snapshots {
		_, remoteChanged := changedRemotely[snap.relPath]
		if !remoteChanged {
			continue
		}
		ext := filepath.Ext(snap.fullPath)
		base := snap.fullPath[:len(snap.fullPath)-len(ext)]
		// remove previous conflict file if one exists
		if existingMeta, err := files.MetaDataGet(pathutils.ToWithPrefix(snap.fullPath)); err == nil && existingMeta != nil && existingMeta.ConflictFile != "" {
			existingConflictFull := pathutils.ToFullPath(pathutils.ToRelative(existingMeta.ConflictFile))
			os.Remove(existingConflictFull)
		}
		ts := time.Now().Format("20060102-150405")
		conflictPath := base + ".conflict." + ts + ".md"
		if err := os.WriteFile(conflictPath, snap.content, 0644); err != nil {
			logging.LogError("git: failed to save conflict copy for %s: %v", snap.fullPath, err)
			continue
		}
		logging.LogWarning("git: conflict on %s — local version saved as %s", filepath.Base(snap.fullPath), filepath.Base(conflictPath))
		notificationStorage.Add("warning",
			fmt.Sprintf("conflict in %s — your version saved as %s", filepath.Base(snap.fullPath), filepath.Base(conflictPath)),
			true)
		origMeta := pathutils.ToWithPrefix(snap.fullPath)
		conflictMeta := pathutils.ToWithPrefix(conflictPath)
		if err := files.SetConflictFile(origMeta, conflictMeta); err != nil {
			logging.LogWarning("git: failed to update conflict metadata for %s: %v", origMeta, err)
		}
		if err := files.SetConflictOf(conflictMeta, origMeta); err != nil {
			logging.LogWarning("git: failed to set conflictOf metadata for %s: %v", conflictMeta, err)
		}
	}
}

// Returns immediately; errors are logged but not propagated.
// No-op if remote is not configured or auto-push is disabled.
func Push() {
	if !remoteEnabled() || !configmanager.GetGitAutoPush() {
		return
	}

	go func() {
		repo, err := openRepo()
		if err != nil {
			gitLog().Printf("push: failed to open repo: %v", err)
			return
		}

		timeout := parsePushTimeout()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		branch := configmanager.GetGitRemoteBranch()
		remote := configmanager.GetGitRemote()

		auth, authErr := buildAuth()
		if authErr != nil {
			gitLog().Printf("push: failed to build auth: %v", authErr)
		}

		err = repo.PushContext(ctx, &git.PushOptions{
			RemoteName: "origin",
			RemoteURL:  remote,
			RefSpecs:   []gitcfg.RefSpec{gitcfg.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch)},
			Auth:       auth,
		})

		if err != nil {
			if errors.Is(err, git.NoErrAlreadyUpToDate) {
				gitLog().Printf("nothing to push")
				return
			}
			gitLog().Printf("push failed: %v", err)
			return
		}

		gitLog().Printf("pushed to %s/%s", remote, branch)
	}()
}

// EnsureRemote creates or updates the "origin" remote in .git/config
// to match KNOV_GIT_REMOTE. Called on startup and after the remote is changed.
func EnsureRemote() error {
	if !remoteEnabled() {
		return nil
	}

	repo, err := openRepo()
	if err != nil {
		return err
	}

	remote := configmanager.GetGitRemote()

	// check if origin already exists with the correct URL
	existing, err := repo.Remote("origin")
	if err == nil {
		urls := existing.Config().URLs
		if len(urls) > 0 && urls[0] == remote {
			logging.LogDebug("git remote origin already set to %s", remote)
			return nil
		}
		// exists but wrong URL — delete and recreate
		if err := repo.DeleteRemote("origin"); err != nil {
			return fmt.Errorf("failed to update remote: %w", err)
		}
	}

	_, err = repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{remote},
	})
	if err != nil {
		return fmt.Errorf("failed to create remote origin: %w", err)
	}

	logging.LogInfo("git remote origin set to %s", remote)
	return nil
}

// TestAuth attempts to build auth and connect to the remote for debugging.
// Returns a descriptive result string and logs details to logs/git-remote.log.
func TestAuth() (string, error) {
	remote := configmanager.GetGitRemote()
	gitLog().Printf("=== auth test start ===")
	gitLog().Printf("remote: %s", remote)
	gitLog().Printf("branch: %s", configmanager.GetGitRemoteBranch())
	gitLog().Printf("ssh key: %q", configmanager.GetGitSSHKey())
	gitLog().Printf("user: %q", configmanager.GetAppConfig().GitUser)

	auth, err := buildAuth()
	if err != nil {
		gitLog().Printf("auth build failed: %v", err)
		return "", fmt.Errorf("auth build failed: %w", err)
	}
	if auth == nil {
		gitLog().Printf("auth: nil (no credentials configured)")
	} else {
		gitLog().Printf("auth: %T", auth)
	}

	repo, err := openRepo()
	if err != nil {
		gitLog().Printf("open repo failed: %v", err)
		return "", fmt.Errorf("open repo failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), parsePushTimeout())
	defer cancel()

	// ls-remote is the lightest possible check — doesn't change anything
	rem, err := repo.Remote("origin")
	if err != nil {
		gitLog().Printf("no origin remote: %v", err)
		return "", fmt.Errorf("no origin remote: %w", err)
	}

	refs, err := rem.ListContext(ctx, &git.ListOptions{Auth: auth})
	if err != nil {
		gitLog().Printf("list remote failed: %v", err)
		return "", fmt.Errorf("remote auth failed: %w", err)
	}

	result := fmt.Sprintf("connected to %s — %d refs found", remote, len(refs))
	gitLog().Printf("%s", result)
	gitLog().Printf("=== auth test end ===")
	return result, nil
}

// -----------------------------------------------
// ----------- History Search -------------------
// -----------------------------------------------

// SearchGitByTitle searches git history for files whose filename contains the query string.
// When deletedOnly is true only files deleted in a commit are returned (for the "search history" feature).
// When deletedOnly is false all touched files are returned (for the latestchanges search).
func SearchGitByTitle(query string, limit int, deletedOnly bool) ([]GitHistoryFile, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	queryLower := strings.ToLower(query)
	seen := make(map[string]bool)
	dataDir := configmanager.GetAppConfig().DataPath
	dataDirName := filepath.Base(dataDir)
	var results []GitHistoryFile

	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && len(results) >= limit {
			return storer.ErrStop
		}
		if c.NumParents() == 0 {
			return nil
		}
		parent, err := c.Parent(0)
		if err != nil {
			return nil
		}
		parentTree, err := parent.Tree()
		if err != nil {
			return nil
		}
		currentTree, err := c.Tree()
		if err != nil {
			return nil
		}
		changes, err := object.DiffTree(parentTree, currentTree)
		if err != nil {
			return nil
		}
		for _, change := range changes {
			if deletedOnly && (change.To.Name != "" || change.From.Name == "") {
				continue
			}
			name := change.To.Name
			if name == "" {
				name = change.From.Name
			}
			if name == "" || seen[name] {
				continue
			}
			relPath := name
			if strings.HasPrefix(relPath, dataDirName+string(filepath.Separator)) {
				relPath = strings.TrimPrefix(relPath, dataDirName+string(filepath.Separator))
			}
			if !strings.Contains(strings.ToLower(filepath.Base(relPath)), queryLower) {
				continue
			}
			seen[name] = true
			results = append(results, GitHistoryFile{
				Name:    filepath.Base(relPath),
				Path:    relPath,
				Commit:  c.Hash.String()[:7],
				Date:    c.Author.When,
				Message: strings.TrimSpace(c.Message),
			})
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return nil, err
	}
	logging.LogDebug("git title search '%s' (deletedOnly=%v) found %d files", query, deletedOnly, len(results))
	return results, nil
}

// SearchDeletedFilesByContent finds deleted files whose content contained the query string.
func SearchDeletedFilesByContent(query string, limit int) ([]GitHistoryFile, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	queryLower := strings.ToLower(query)
	seen := make(map[string]bool)
	var results []GitHistoryFile

	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && len(results) >= limit {
			return storer.ErrStop
		}
		if c.NumParents() == 0 {
			return nil
		}
		parent, err := c.Parent(0)
		if err != nil {
			return nil
		}
		parentTree, err := parent.Tree()
		if err != nil {
			return nil
		}
		currentTree, err := c.Tree()
		if err != nil {
			return nil
		}
		changes, err := object.DiffTree(parentTree, currentTree)
		if err != nil {
			return nil
		}
		for _, change := range changes {
			if change.To.Name != "" || change.From.Name == "" {
				continue
			}
			if seen[change.From.Name] {
				continue
			}
			f, err := parentTree.File(change.From.Name)
			if err != nil {
				continue
			}
			content, err := f.Contents()
			if err != nil {
				continue
			}
			if !strings.Contains(strings.ToLower(content), queryLower) {
				continue
			}
			seen[change.From.Name] = true
			results = append(results, GitHistoryFile{
				Name:    filepath.Base(change.From.Name),
				Path:    change.From.Name,
				Commit:  c.Hash.String()[:7],
				Date:    c.Author.When,
				Message: strings.TrimSpace(c.Message),
			})
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return nil, err
	}
	logging.LogDebug("git content search '%s' found %d deleted files", query, len(results))
	return results, nil
}
