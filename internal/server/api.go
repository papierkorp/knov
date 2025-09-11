// Package server - Clean API handlers that delegate to business logic
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/testdata"
	"knov/internal/thememanager"
	"knov/internal/translation"
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
	config := configmanager.GetConfig()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// @Summary Set configuration
// @Tags config
// @Router /api/config/setConfig [post]
func handleAPISetConfig(w http.ResponseWriter, r *http.Request) {
	var config configmanager.ConfigManager

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	configmanager.SetConfig(config)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// @Summary Set language
// @Tags config
// @Router /api/config/setLanguage [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("language")

	logging.LogDebug("language set to: %s", lang)

	if lang != "" {
		configmanager.SetLanguage(lang)
		translation.SetLanguage(lang)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get git repository URL
// @Tags config
// @Produce json
// @Success 200 {object} string
// @Router /api/config/getRepositoryURL [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	config := configmanager.GetConfigGit()

	// Get remote URL from git config
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = configmanager.DataPath
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

	response := repositoryURL

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Summary Set git repository URL
// @Tags config
// @Router /api/config/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	logging.LogDebug("received repositoryUrl: '%s'", repositoryURL)

	if repositoryURL == "" {
		logging.LogError("empty repositoryUrl")
		http.Error(w, "repositoryUrl cannot be empty", http.StatusBadRequest)
		return
	}

	config := configmanager.GetConfigGit()
	dataDir := configmanager.DataPath

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
	configmanager.SetConfigGit(config)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved"))
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
	tm := thememanager.GetThemeManager()

	response := struct {
		Current   string   `json:"current"`
		Available []string `json:"available"`
	}{
		Current:   tm.GetCurrentThemeName(),
		Available: tm.GetAvailableThemes(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Success 303 "Redirect to settings page"
// @Router /api/themes/setTheme [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	theme := r.FormValue("theme")

	if theme != "" {
		tm := thememanager.GetThemeManager()
		err := tm.LoadTheme(theme)
		if err == nil {
			tm.SetCurrentTheme(theme)
			newConfig := configmanager.ConfigThemes{CurrentTheme: theme}
			configmanager.SetConfigThemes(newConfig)
		}
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// ----------------------------------------------------------------------------------------
// ----------------------------------------- files -----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get all files
// @Tags files
// @Produce json
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	files, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, "failed to get files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// @Summary Get file content as html
// @Tags files
// @Param filepath path string true "File path"
// @Produce text/html
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {

	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/content/")

	dataDir := configmanager.DataPath

	fullPath := filepath.Join(dataDir, filePath)

	html, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

// @Summary Filter files by metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param metadata[] formData []string false "Metadata fields to filter on"
// @Param operator[] formData []string false "Filter operators (equals, contains, greater, less, in)"
// @Param value[] formData []string false "Filter values"
// @Param logic[] formData []string false "Logic operators (and, or)"
// @Param action[] formData []string false "Filter actions (include, exclude)"
// @Success 200 {array} files.File
// @Router /api/files/filter [post]
func handleAPIFilterFiles(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter request received")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	logging.LogDebug("form data: %+v", r.Form)

	var criteria []files.FilterCriteria
	metadata := r.Form["metadata[]"]
	operators := r.Form["operator[]"]
	values := r.Form["value[]"]
	logic := r.Form["logic[]"]
	actions := r.Form["action[]"]

	logging.LogDebug("metadata: %v, operators: %v, values: %v", metadata, operators, values)

	maxLen := len(metadata)
	for i := 0; i < maxLen; i++ {
		if i < len(operators) && i < len(values) && metadata[i] != "" && operators[i] != "" {
			criteria = append(criteria, files.FilterCriteria{
				Metadata: metadata[i],
				Operator: operators[i],
				Value:    values[i],
				Logic:    getFormValue(logic, i),
				Action:   getFormValue(actions, i),
			})
		}
	}

	logging.LogDebug("built %d filter criteria: %+v", len(criteria), criteria)

	filteredFiles, err := files.FilterFilesByMetadata(criteria)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		http.Error(w, "failed to filter files", http.StatusInternalServerError)
		return
	}

	logging.LogDebug("filtered %d files", len(filteredFiles))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredFiles)
}

func getFormValue(slice []string, index int) string {
	if index < len(slice) {
		return slice[index]
	}
	return ""
}

// ----------------------------------------------------------------------------------------
// ------------------------------------------ git ------------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get recently changed files
// @Tags git
// @Router /api/git/history [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 10 // default
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	files, err := git.GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, "failed to get recent files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
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

// @Summary Initialize/Rebuild metadata for all files
// @Description Creates metadata for all files that don't have metadata yet
// @Tags files
// @Produce json
// @Success 200 {string} string "metadata initialized"
// @Failure 500 {string} string "failed to initialize metadata"
// @Router /api/files/metadata/rebuild [post]
func handleAPIRebuildMetadata(w http.ResponseWriter, r *http.Request) {
	if err := files.MetaDataInitializeAll(); err != nil {
		http.Error(w, "failed to initialize metadata", http.StatusInternalServerError)
		return
	}

	err := files.MetaDataLinksRebuild()
	if err != nil {
		http.Error(w, "failed to rebuild metadata links", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "metadata initialized"})
}

// ----------------------------------------------------------------------------------------
// --------------------------------------- TESTDATA ---------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Setup test data
// @Description Creates test files, git operations, and metadata for testing
// @Tags testdata
// @Produce json
// @Success 200 {object} string "{"status":"ok","message":"test data setup completed"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/setup [post]
func handleAPISetupTestData(w http.ResponseWriter, r *http.Request) {
	err := testdata.SetupTestData()
	if err != nil {
		http.Error(w, "failed to setup test data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","message":"test data setup completed"}`))
}

// @Summary Clean test data
// @Description Removes all test data files and metadata
// @Tags testdata
// @Produce json
// @Success 200 {object} string "{"status":"ok","message":"test data cleaned"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/clean [post]
func handleAPICleanTestData(w http.ResponseWriter, r *http.Request) {
	err := testdata.CleanTestData()
	if err != nil {
		http.Error(w, "failed to clean test data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","message":"test data cleaned"}`))
}
