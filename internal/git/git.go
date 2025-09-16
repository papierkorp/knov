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
	dataDir := configmanager.GetAppConfig().DataPath
	cmd := exec.Command("git", "log", "--oneline", "--name-only", "--pretty=format:%h|%ad|%s", "--date=short", "-n", strconv.Itoa(count), "--", "*.md")
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
