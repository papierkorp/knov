// Package git - Git operations for file management
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitHistoryFile represents a file in git history
type GitHistoryFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// FileVersion represents a single version of a file
type FileVersion struct {
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	IsCurrent bool   `json:"is_current"`
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

// GetRecentlyChangedFiles returns list of recently changed files
func GetRecentlyChangedFiles(count int) ([]GitHistoryFile, error) {
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
	dataDir := configmanager.GetAppConfig().DataPath
	commitCount := 0

	err = iter.ForEach(func(c *object.Commit) error {
		if commitCount >= count {
			return nil
		}

		stats, err := c.Stats()
		if err != nil {
			return err
		}

		for _, stat := range stats {
			// stat.Name is relative to repo root
			// if repo is above data dir, paths will include data dir name
			// strip it to get path relative to data directory
			relPath := stat.Name
			dataDirName := filepath.Base(dataDir)
			if strings.HasPrefix(relPath, dataDirName+string(filepath.Separator)) {
				relPath = strings.TrimPrefix(relPath, dataDirName+string(filepath.Separator))
			}

			files = append(files, GitHistoryFile{
				Name:    filepath.Base(relPath),
				Path:    relPath,
				Commit:  c.Hash.String()[:7],
				Date:    c.Author.When.Format("02-01-2006 - 15:04"),
				Message: c.Message,
			})
		}
		commitCount++
		return nil
	})

	return files, err
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
	return untrackedFiles, nil
}

// CommitDeletedFiles commits all deleted files
func CommitDeletedFiles(deletedFiles []string) error {
	if len(deletedFiles) == 0 {
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
	return nil
}

// CommitModifiedFiles commits all modified files
func CommitModifiedFiles(modifiedFiles []string) error {
	if len(modifiedFiles) == 0 {
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
	return nil
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

	iter, err := repo.Log(&git.LogOptions{
		From:     ref.Hash(),
		FileName: &relPath,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var versions []FileVersion
	err = iter.ForEach(func(c *object.Commit) error {
		versions = append(versions, FileVersion{
			Commit:    c.Hash.String()[:7],
			Date:      c.Author.When.Format("02-01-2006 - 15:04"),
			Message:   c.Message,
			Author:    c.Author.Name,
			IsCurrent: false,
		})
		return nil
	})

	// mark the first (most recent) as current
	if len(versions) > 0 {
		versions[0].IsCurrent = true
	}

	return versions, err
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
		logging.LogError("failed to get file %s from commit %s: %v", relPath, commit, err)
		return err
	}

	content, err := file.Contents()
	if err != nil {
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
func GetCommitDetails(commit string) (string, string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", "", err
	}

	commitHash, err := expandCommitHash(repo, commit)
	if err != nil {
		logging.LogError("failed to find commit %s: %v", commit, err)
		return "", "", err
	}

	commitObj, err := repo.CommitObject(commitHash)
	if err != nil {
		logging.LogError("failed to get commit details for %s: %v", commit, err)
		return "", "", err
	}

	date := commitObj.Author.When.Format("02-01-2006 - 15:04")
	message := strings.TrimSpace(commitObj.Message)

	return date, message, nil
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

// GetLastProcessedCommit returns the last commit that was processed for metadata
func GetLastProcessedCommit() (string, error) {
	commitFile := filepath.Join(configmanager.GetAppConfig().StoragePath, ".last_processed_commit")

	data, err := os.ReadFile(commitFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// SetLastProcessedCommit saves the last processed commit hash
func SetLastProcessedCommit(commitHash string) error {
	storagePath := configmanager.GetAppConfig().StoragePath
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return err
	}

	commitFile := filepath.Join(storagePath, ".last_processed_commit")
	return os.WriteFile(commitFile, []byte(commitHash), 0644)
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
		logging.LogError("failed to get file %s at commit %s: %v", relPath, commit, err)
		return "", err
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
