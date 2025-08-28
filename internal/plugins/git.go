// Package plugins - Git API handlers
package plugins

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"knov/internal/configmanager"
)

// HandleGetRepositoryURL returns git repository information
func HandleGetRepositoryURL(w http.ResponseWriter, r *http.Request) {
	config := configmanager.GetConfigGit()
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
		log.Printf("error in git config get remote.origin.url command - using config repositoryURL instead")
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
		config := configmanager.GetConfigGit()
		config.DataPath = dataPath
		configmanager.SetConfigGit(config)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved"))
}

// HandleSetRepositoryURL sets the git repository URL from form data
func HandleSetRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	log.Printf("DEBUG: Received repositoryUrl: '%s'", repositoryURL)

	if repositoryURL == "" {
		log.Printf("DEBUG: Empty repositoryUrl")
		http.Error(w, "repositoryUrl cannot be empty", http.StatusBadRequest)
		return
	}

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	log.Printf("DEBUG: Using dataDir: '%s'", dataDir)

	// Check if dataDir exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		log.Printf("DEBUG: Data directory doesn't exist: %s", dataDir)
		http.Error(w, fmt.Sprintf("Data directory doesn't exist: %s", dataDir), http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG: Attempting to set git remote URL...")

	cmd := exec.Command("git", "remote", "set-url", "origin", repositoryURL)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("DEBUG: set-url failed with error: %v, output: %s", err, string(output))
		log.Printf("DEBUG: Trying to add remote instead...")

		cmd = exec.Command("git", "remote", "add", "origin", repositoryURL)
		cmd.Dir = dataDir
		output, err = cmd.CombinedOutput()

		if err != nil {
			log.Printf("DEBUG: add remote also failed with error: %v, output: %s", err, string(output))
			http.Error(w, fmt.Sprintf("Git command failed: %v - %s", err, string(output)), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("DEBUG: Git remote set successfully")

	config.RepositoryURL = repositoryURL
	configmanager.SetConfigGit(config)

	log.Printf("DEBUG: Config saved successfully")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved"))
}
