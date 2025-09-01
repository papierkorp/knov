// Package server - Clean API handlers that delegate to business logic
package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
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

// @Summary Get all files with metadata
// @Tags files
// @Produce json
// @Router /api/files/metadata [get]
func handleAPIGetAllFilesWithMetadata(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetAllFilesWithMetadata(w, r)
}

// @Summary Get file metadata
// @Tags files
// @Produce json
// @Router /api/files/metadata/{filepath} [get]
func handleAPIGetFileMetadata(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetFileMetadata(w, r)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------------ git ------------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get recently changed files
// @Tags files
// @Param count query int false "Number of recent files" default(10)
// @Produce json
// @Router /api/files/git/history [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetRecentlyChanged(w, r)
}

// @Summary Get file diff
// @Tags files
// @Produce json
// @Router /api/files/git/diff/{filepath} [get]
func handleAPIGetFileDiff(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIGetFileDiff(w, r)
}

// @Summary Add file to git
// @Tags files
// @Router /api/files/git/add/{filepath} [post]
func handleAPIAddFile(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIAddFile(w, r)
}

// @Summary Add all files to git
// @Tags files
// @Router /api/files/git/addall [post]
func handleAPIAddAllFiles(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIAddAllFiles(w, r)
}

// @Summary Delete file from git
// @Tags files
// @Router /api/files/git/delete/{filepath} [delete]
func handleAPIDeleteFile(w http.ResponseWriter, r *http.Request) {
	files.HandleAPIDeleteFile(w, r)
}
