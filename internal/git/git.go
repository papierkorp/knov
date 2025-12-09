// Package git - Git operations for file management
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
)

// GitHistoryFile represents a file in git history
type GitHistoryFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// GetRecentlyChangedFiles returns list of recently changed files
func GetRecentlyChangedFiles(count int) ([]GitHistoryFile, error) {
	dataDir := configmanager.GetAppConfig().DataPath
	cmd := exec.Command("git", "log", "--oneline", "--name-only", "--pretty=format:%h|%ad|%s", "--date=short", "-n", strconv.Itoa(count))
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get git history: %v, output: %s", err, string(output))
		return nil, err
	}

	var files []GitHistoryFile
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	var currentCommit, currentDate, currentMessage string
	for _, line := range lines {
		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) == 3 {
				currentCommit = parts[0]
				currentDate = parts[1]
				currentMessage = parts[2]
			}
		} else if line != "" {
			files = append(files, GitHistoryFile{
				Name:    strings.Split(line, "/")[len(strings.Split(line, "/"))-1],
				Path:    dataDir + "/" + line,
				Commit:  currentCommit,
				Date:    currentDate,
				Message: currentMessage,
			})
		}
	}

	return files, nil
}

// GetUntrackedFiles returns list of untracked files in git
func GetUntrackedFiles() ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// check if git repo exists
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	// git ls-files --others --exclude-standard
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to list untracked files: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var untrackedFiles []string
	for _, file := range files {
		if file != "" {
			untrackedFiles = append(untrackedFiles, file)
		}
	}

	return untrackedFiles, nil
}

// AddNewFiles adds all untracked files in the data directory to git
func AddNewFiles() ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// get list of untracked files before adding
	untrackedFiles, err := GetUntrackedFiles()
	if err != nil {
		return nil, err
	}

	if len(untrackedFiles) == 0 {
		logging.LogDebug("no new files to add")
		return nil, nil
	}

	logging.LogInfo("found %d untracked files", len(untrackedFiles))

	// git add .
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to add files to git: %v, output: %s", err, string(output))
		return nil, err
	}

	// commit changes
	cmd = exec.Command("git", "commit", "-m", "auto-commit: new files added")
	cmd.Dir = dataDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to commit files: %v, output: %s", err, string(output))
		return nil, err
	}

	logging.LogInfo("auto-committed %d new files to git", len(untrackedFiles))
	return untrackedFiles, nil
}

// GetLastProcessedCommit returns the last commit that was processed for metadata
func GetLastProcessedCommit() (string, error) {
	commitFile := filepath.Join("config", ".last_processed_commit")

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
	commitFile := filepath.Join("config", ".last_processed_commit")
	return os.WriteFile(commitFile, []byte(commitHash), 0644)
}

// GetCurrentCommit returns the current HEAD commit hash
func GetCurrentCommit() (string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// check if git repo exists
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", nil
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get current commit: %v, output: %s", err, string(output))
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetFilesChangedSinceCommit returns files that changed since a specific commit
func GetFilesChangedSinceCommit(lastCommit string) ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	if lastCommit == "" {
		// if no last commit, return all files
		cmd := exec.Command("git", "ls-files")
		cmd.Dir = dataDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			logging.LogError("failed to list all files: %v, output: %s", err, string(output))
			return nil, err
		}

		if len(output) == 0 {
			return nil, nil
		}

		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		var allFiles []string
		for _, file := range files {
			if file != "" {
				allFiles = append(allFiles, file)
			}
		}
		return allFiles, nil
	}

	// validate that the commit exists before using it
	if !CommitExists(lastCommit) {
		logging.LogWarning("commit %s no longer exists, resetting to process all files", lastCommit)
		// reset the last processed commit since it's invalid
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		// return all files to reprocess everything
		return GetFilesChangedSinceCommit("")
	}

	// get files changed between lastCommit and HEAD
	cmd := exec.Command("git", "diff", "--name-only", lastCommit+"..HEAD")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get changed files: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var changedFiles []string
	for _, file := range files {
		if file != "" {
			changedFiles = append(changedFiles, file)
		}
	}

	return changedFiles, nil
}

// GetDeletedFilesSinceCommit returns files that were deleted since a specific commit
func GetDeletedFilesSinceCommit(lastCommit string) ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	if lastCommit == "" {
		// no deleted files if no previous commit
		return nil, nil
	}

	// validate that the commit exists before using it
	if !CommitExists(lastCommit) {
		logging.LogWarning("commit %s no longer exists, cannot check for deleted files", lastCommit)
		// reset the last processed commit since it's invalid
		if err := SetLastProcessedCommit(""); err != nil {
			logging.LogError("failed to reset last processed commit: %v", err)
		}
		return nil, nil
	}

	// get deleted files between lastCommit and HEAD
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=D", lastCommit+"..HEAD")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get deleted files: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var deletedFiles []string
	for _, file := range files {
		if file != "" {
			deletedFiles = append(deletedFiles, file)
		}
	}

	return deletedFiles, nil
}

// GetUncommittedDeletedFiles returns files that are deleted but not yet committed
func GetUncommittedDeletedFiles() ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// check if git repo exists
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	// git ls-files --deleted
	cmd := exec.Command("git", "ls-files", "--deleted")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to list deleted files: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var deletedFiles []string
	for _, file := range files {
		if file != "" {
			deletedFiles = append(deletedFiles, file)
		}
	}

	return deletedFiles, nil
}

// GetModifiedFiles returns files that are modified but not yet committed
func GetModifiedFiles() ([]string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// check if git repo exists
	gitDir := filepath.Join(dataDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		logging.LogDebug("no git repository found")
		return nil, nil
	}

	// git ls-files --modified
	cmd := exec.Command("git", "ls-files", "--modified")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to list modified files: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var modifiedFiles []string
	for _, file := range files {
		if file != "" {
			modifiedFiles = append(modifiedFiles, file)
		}
	}

	return modifiedFiles, nil
}

// CommitDeletedFiles commits all deleted files
func CommitDeletedFiles(deletedFiles []string) error {
	if len(deletedFiles) == 0 {
		return nil
	}

	dataDir := configmanager.GetAppConfig().DataPath

	// git add -u (add all deletions)
	cmd := exec.Command("git", "add", "-u")
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to stage deleted files: %v, output: %s", err, string(output))
		return err
	}

	// commit deletions
	cmd = exec.Command("git", "commit", "-m", "auto-commit: files deleted")
	cmd.Dir = dataDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to commit deleted files: %v, output: %s", err, string(output))
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

	dataDir := configmanager.GetAppConfig().DataPath

	// stage all modified files
	args := append([]string{"add"}, modifiedFiles...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to stage modified files: %v, output: %s", err, string(output))
		return err
	}

	// commit modifications
	cmd = exec.Command("git", "commit", "-m", "auto-commit: files modified")
	cmd.Dir = dataDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to commit modified files: %v, output: %s", err, string(output))
		return err
	}

	logging.LogInfo("auto-committed %d modified files to git", len(modifiedFiles))
	return nil
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

// GetFileHistory returns the git history for a specific file
func GetFileHistory(filePath string) ([]FileVersion, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// get relative path from data directory
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	cmd := exec.Command("git", "log", "--pretty=format:%h|%ad|%s|%an", "--date=short", "--follow", "--", relPath)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get file history for %s: %v, output: %s", relPath, err, string(output))
		return nil, err
	}

	var versions []FileVersion
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	currentCommit, _ := GetCurrentCommit()

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			isCurrent := parts[0] == currentCommit[:len(parts[0])]
			versions = append(versions, FileVersion{
				Commit:    parts[0],
				Date:      parts[1],
				Message:   parts[2],
				Author:    parts[3],
				IsCurrent: isCurrent,
			})
		}
	}

	return versions, nil
}

// GetFileAtCommit returns the content of a file at a specific commit
func GetFileAtCommit(filePath, commit string) (string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// validate that the commit exists before trying to access it
	if !CommitExists(commit) {
		logging.LogWarning("commit %s does not exist for file %s", commit, filePath)
		return "", fmt.Errorf("commit %s not found", commit)
	}

	// get relative path from data directory
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	cmd := exec.Command("git", "show", commit+":"+relPath)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get file at commit %s for %s: %v, output: %s", commit, relPath, err, string(output))
		return "", err
	}

	return string(output), nil
}

// GetFileDiff returns the diff between two commits for a file
func GetFileDiff(filePath, fromCommit, toCommit string) (string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	// get relative path from data directory
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	cmd := exec.Command("git", "diff", fromCommit+".."+toCommit, "--", relPath)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get diff for %s between %s and %s: %v, output: %s", relPath, fromCommit, toCommit, err, string(output))
		return "", err
	}

	return string(output), nil
}

// RestoreFileToCommit restores a file to a specific commit
func RestoreFileToCommit(filePath, commit string) error {
	dataDir := configmanager.GetAppConfig().DataPath

	// get relative path from data directory
	relPath, err := filepath.Rel(dataDir, filePath)
	if err != nil {
		relPath = filePath
	}

	cmd := exec.Command("git", "checkout", commit, "--", relPath)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to restore file %s to commit %s: %v, output: %s", relPath, commit, err, string(output))
		return err
	}

	// add and commit the restored file
	cmd = exec.Command("git", "add", relPath)
	cmd.Dir = dataDir
	addOutput, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to add restored file %s: %v, output: %s", relPath, err, string(addOutput))
		return err
	}

	// commit the restoration with descriptive message
	commitMessage := fmt.Sprintf("restore %s to commit %s", relPath, commit)
	cmd = exec.Command("git", "commit", "-m", commitMessage)
	cmd.Dir = dataDir
	commitOutput, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to commit restored file %s: %v, output: %s", relPath, err, string(commitOutput))
		return err
	}

	logging.LogInfo("restored file %s to commit %s and logged the change", relPath, commit)
	return nil
}

// GetCommitDetails returns details for a specific commit
func GetCommitDetails(commit string) (string, string, error) {
	dataDir := configmanager.GetAppConfig().DataPath

	cmd := exec.Command("git", "log", "--pretty=format:%ad|%s", "--date=short", "-1", commit)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.LogError("failed to get commit details for %s: %v, output: %s", commit, err, string(output))
		return "", "", err
	}

	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], nil // date, message
	}

	return "", "", fmt.Errorf("could not parse commit details")
}

// CommitExists checks if a commit hash exists in the repository
func CommitExists(commit string) bool {
	if commit == "" {
		return false
	}

	dataDir := configmanager.GetAppConfig().DataPath

	cmd := exec.Command("git", "cat-file", "-e", commit)
	cmd.Dir = dataDir
	err := cmd.Run()

	return err == nil
}
