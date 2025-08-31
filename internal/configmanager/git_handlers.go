// Package configmanager - Git configuration handlers
package configmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"knov/internal/logging"
)

// HandleGetRepositoryURL returns git repository information
func HandleGetRepositoryURL(w http.ResponseWriter, r *http.Request) {
	config := GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	// Get remote URL from git config
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = dataDir
	output, err := cmd.Output()
	var repositoryURL string

	if err != nil {
		logging.LogError("error in git config get remote.origin.url command - using config repositoryURL instead")
		repositoryURL = config.RepositoryURL
		if repositoryURL == "" {
			repositoryURL = "local"
		}
	} else {
		repositoryURL = strings.TrimSpace(string(output))
	}

	response := map[string]string{
		"repositoryUrl": repositoryURL,
		"dataPath":      dataDir,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleSetDataPath sets the git data path from form data
func HandleSetDataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	dataPath := r.FormValue("dataPath")

	if dataPath != "" {
		config := GetConfigGit()
		config.DataPath = dataPath
		SetConfigGit(config)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved"))
}

// HandleSetRepositoryURL sets the git repository URL from form data
func HandleSetRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	logging.LogDebug("received repositoryUrl: '%s'", repositoryURL)

	if repositoryURL == "" {
		logging.LogError("empty repositoryUrl")
		http.Error(w, "repositoryUrl cannot be empty", http.StatusBadRequest)
		return
	}

	config := GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	logging.LogDebug("using datadir: '%s'", dataDir)

	// Check if dataDir exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		logging.LogError("data directory doesn't exist: %s", dataDir)
		http.Error(w, fmt.Sprintf("data directory doesn't exist: %s", dataDir), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("attempting to set git remote URL...")

	cmd := exec.Command("git", "remote", "set-url", "origin", repositoryURL)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		logging.LogError("set-url failed with error: %v, output: %s", err, string(output))
		logging.LogDebug("trying to add remote instead...")

		cmd = exec.Command("git", "remote", "add", "origin", repositoryURL)
		cmd.Dir = dataDir
		output, err = cmd.CombinedOutput()

		if err != nil {
			logging.LogError("add remote failed with error: %v - %s", err, string(output))
			http.Error(w, fmt.Sprintf("Git command failed: %v - %s", err, string(output)), http.StatusInternalServerError)
			return
		}
	}

	logging.LogInfo("git remote set successfully")

	config.RepositoryURL = repositoryURL
	SetConfigGit(config)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved"))
}
