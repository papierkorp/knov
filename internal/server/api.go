// Package server - Clean API handlers that delegate to business logic
package server

import (
	"encoding/json"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/thememanager"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------------- health ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Health check
// @Tags health
// @Router /api/health [get]
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// ----------------------------------------------------------------------------------------
// ---------------------------------------- config ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get current configuration
// @Tags config
// @Router /api/config/getConfig [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleGetConfig(w, r)
}

// @Summary Set configuration
// @Tags config
// @Router /api/config/setConfig [post]
func handleAPISetConfig(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetConfig(w, r)
}

// @Summary Set language
// @Tags config
// @Router /api/config/setLanguage [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetLanguage(w, r)
}

// @Summary Get git repository URL
// @Tags config
// @Produce json
// @Success 200 {object} string
// @Router /api/config/getRepositoryURL [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleGetRepositoryURL(w, r)
}

// @Summary Set git data path
// @Tags config
// @Router /api/config/setDataPath [post]
func handleAPISetGitDataPath(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetDataPath(w, r)
}

// @Summary Set git repository URL
// @Tags config
// @Router /api/config/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetRepositoryURL(w, r)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------------- themes ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get themes
// @Description Get current theme and available themes
// @Tags themes
// @Produce json
// @Success 200 {string} string "{"current":"themename","available":["theme1","theme2"]}"
// @Router /api/themes/getAllThemes [get]
func handleAPIGetThemes(w http.ResponseWriter, r *http.Request) {
	thememanager.HandleGetThemes(w, r)
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Success 303 "Redirect to settings page"
// @Router /api/themes/setTheme [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	thememanager.HandleSetTheme(w, r)
}

// ----------------------------------------------------------------------------------------
// ----------------------------------------- files -----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get all files
// @Tags files
// @Produce json
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetAllFiles(w, r)
}

// @Summary Get file content as html
// @Tags files
// @Produce json
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetFileContent(w, r)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------------ git ------------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get recently changed files
// @Tags git
// @Router /api/git/history [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	git.HandleAPIGetRecentlyChanged(w, r)
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- METADATA ---------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get metadata for a single file
// @Description Get metadata for a file by providing filepath as query parameter
// @Tags files
// @Produce json
// @Param filepath query string true "File path"
// @Success 200 {object} files.Metadata
// @Failure 400 {string} string "missing filepath parameter"
// @Failure 404 {string} string "metadata not found"
// @Failure 500 {string} string "failed to get metadata"
// @Router /api/files/metadata [get]
func handleAPIGetMetadata(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// @Summary Set metadata for a single file
// @Description Set metadata for a file using JSON payload
// @Tags files
// @Accept json
// @Produce json
// @Param metadata body files.Metadata true "Metadata object"
// @Success 200 {string} string "metadata saved"
// @Failure 400 {string} string "invalid json or missing path"
// @Failure 500 {string} string "failed to save metadata"
// @Router /api/files/metadata [post]
func handleAPISetMetadata(w http.ResponseWriter, r *http.Request) {
	// example json
	// {
	//   "path": "example_markdown.md",
	//   "project": "my-project",
	//   "tags": ["important", "work"],
	//   "type": "todo",
	//   "status": "draft",
	//   "priority": "high"
	// }
	var metadata files.Metadata

	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if metadata.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	err := files.MetaDataSave(&metadata)
	if err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("metadata saved"))
}

// @Summary Initialize metadata for all files
// @Description Creates metadata for all files that don't have metadata yet
// @Tags files
// @Produce json
// @Success 200 {string} string "metadata initialized"
// @Failure 500 {string} string "failed to initialize metadata"
// @Router /api/files/metadata/init [post]
func handleAPIInitMetadata(w http.ResponseWriter, r *http.Request) {
	if err := files.MetaDataInitializeAll(); err != nil {
		http.Error(w, "failed to initialize metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "metadata initialized"})
}
