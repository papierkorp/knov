// Package files handles file operations and metadata
package files

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
)

// HandleAPIGetAllFiles returns list of all files
func HandleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	files, err := GetAllFiles()
	if err != nil {
		http.Error(w, "failed to get files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// HandleAPIGetFileContent returns html content for specific file
func HandleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/content/")

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	fullPath := filepath.Join(dataDir, filePath)

	html, err := GetFileContent(fullPath)
	if err != nil {
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

// HandleAPIGetAllFilesWithMetadata returns all files with metadata
func HandleAPIGetAllFilesWithMetadata(w http.ResponseWriter, r *http.Request) {
	files, err := GetAllFilesWithMetadata()
	if err != nil {
		http.Error(w, "failed to get files with metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// HandleAPIGetFileMetadata returns metadata for specific file
func HandleAPIGetFileMetadata(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/metadata/")

	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	fullPath := filepath.Join(dataDir, filePath)

	metadata, err := GetFileMetadata(fullPath)
	if err != nil {
		http.Error(w, "failed to get file metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}
