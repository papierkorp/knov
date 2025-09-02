// Package git - Git operations for file management
package git

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
)

// HandleAPIGetRecentlyChanged returns recently changed files
func HandleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 10 // default
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	files, err := GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, "failed to get recent files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// HandleAPIGetFileDiff returns diff for specific file
func HandleAPIGetFileDiff(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/git/diff/")

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	fullPath := dataDir + "/" + filePath

	diff, err := GetFileDiff(fullPath)
	if err != nil {
		http.Error(w, "failed to get file diff", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"diff": diff})
}

// HandleAPIAddFile adds a file to git
func HandleAPIAddFile(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/git/add/")

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	fullPath := dataDir + "/" + filePath

	err := AddFile(fullPath)
	if err != nil {
		http.Error(w, "failed to add file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("file added successfully"))
}

// HandleAPIAddAllFiles adds all files to git
func HandleAPIAddAllFiles(w http.ResponseWriter, r *http.Request) {
	err := AddAllFiles()
	if err != nil {
		http.Error(w, "failed to add all files", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("all files added successfully"))
}

// HandleAPIDeleteFile deletes a file from git
func HandleAPIDeleteFile(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/git/delete/")

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	fullPath := dataDir + "/" + filePath

	err := DeleteFile(fullPath)
	if err != nil {
		http.Error(w, "failed to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("file deleted successfully"))
}
