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

// HandleAPIGetFileContent returns html content
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
