// Package git - Git operations for file management
package git

import (
	"os/exec"
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
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	cmd := exec.Command("git", "log", "--oneline", "--name-only", "--pretty=format:%h|%ad|%s", "--date=short", "-n", strconv.Itoa(count), "--", "*.md")
	cmd.Dir = dataDir
	output, err := cmd.Output()
	if err != nil {
		logging.LogError("failed to get git history: %v", err)
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
		} else if line != "" && strings.HasSuffix(line, ".md") {
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

// GetFileDiff returns git diff for a specific file
func GetFileDiff(filePath string) (string, error) {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	relativePath := strings.TrimPrefix(filePath, dataDir+"/")
	cmd := exec.Command("git", "diff", "HEAD~1", "HEAD", "--", relativePath)
	cmd.Dir = dataDir
	output, err := cmd.Output()
	if err != nil {
		logging.LogError("failed to get git diff: %v", err)
		return "", err
	}

	return string(output), nil
}

// AddFile adds a single file to git
func AddFile(filePath string) error {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	relativePath := strings.TrimPrefix(filePath, dataDir+"/")

	// Add file
	cmd := exec.Command("git", "add", relativePath)
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to add file: %v", err)
		return err
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", "add file: "+relativePath)
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to commit file: %v", err)
		return err
	}

	return nil
}

// AddAllFiles adds all files in data directory to git
func AddAllFiles() error {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	// Add all
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to add all files: %v", err)
		return err
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", "add all files")
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to commit all files: %v", err)
		return err
	}

	return nil
}

// DeleteFile removes a file from git
func DeleteFile(filePath string) error {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	relativePath := strings.TrimPrefix(filePath, dataDir+"/")

	// Remove file
	cmd := exec.Command("git", "rm", relativePath)
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to remove file: %v", err)
		return err
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", "delete file: "+relativePath)
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		logging.LogError("failed to commit deletion: %v", err)
		return err
	}

	return nil
}
