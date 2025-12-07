// Package server ..
package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/utils"
)

// @Summary Get folder structure
// @Tags files
// @Param path query string false "folder path (root if empty)"
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/files/folder [get]
func handleAPIGetFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("path")

	dataPath := configmanager.GetAppConfig().DataPath
	fullPath := filepath.Join(dataPath, folderPath)

	// read directory
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		logging.LogError("failed to read folder %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read folder"), http.StatusInternalServerError)
		return
	}

	var folders []render.FolderEntry
	var filesInDir []render.FolderEntry

	for _, entry := range entries {
		entryPath := filepath.Join(folderPath, entry.Name())
		item := render.FolderEntry{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			folders = append(folders, item)
		} else {
			filesInDir = append(filesInDir, item)
		}
	}

	html := render.RenderFolderContent(folderPath, folders, filesInDir)
	writeResponse(w, r, map[string]interface{}{
		"path":    folderPath,
		"folders": folders,
		"files":   filesInDir,
	}, html)
}

// @Summary Get all files
// @Tags files
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}

	format := r.URL.Query().Get("format")

	if format == "options" {
		html := render.RenderFilesOptions(allFiles)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	if format == "datalist" {
		html := render.RenderFilesDatalist(allFiles)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	html := render.RenderFilesList(allFiles)
	writeResponse(w, r, allFiles, html)
}

// @Summary Get file content as html
// @Tags files
// @Param filepath path string true "File path"
// @Produce text/html
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/content/")
	fullPath := utils.ToFullPath(filePath)

	content, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(content.HTML))
}

// @Summary Get file header with link and breadcrumb
// @Tags files
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/files/header [get]
func handleAPIGetFileHeader(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	data := map[string]string{
		"filepath": filepath,
		"link":     "/files/" + filepath,
	}

	html := render.RenderFileHeader(filepath)
	writeResponse(w, r, data, html)
}

// @Summary Get raw file content
// @Description Returns unprocessed file content for editing
// @Tags files
// @Param filepath query string true "File path"
// @Produce json,plain
// @Success 200 {string} string "raw content"
// @Router /api/files/raw [get]
func handleAPIGetRawContent(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		logging.LogError("failed to get raw content: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get raw content"), http.StatusInternalServerError)
		return
	}

	data := map[string]string{"content": content}
	writeResponse(w, r, data, content)
}

// @Summary Save file content
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "File path"
// @Param content formData string true "File content"
// @Produce html
// @Router /api/files/save [post]
func handleAPIFileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	fullPath := utils.ToFullPath(filePath)

	// check if file exists (to determine if this is creation or update)
	_, statErr := os.Stat(fullPath)
	isNewFile := os.IsNotExist(statErr)

	// create directories if they don't exist
	if isNewFile {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logging.LogError("failed to create directory %s: %v", dir, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
			return
		}
	}

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to save file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved file: %s", filePath)

	// if this was a new file creation, redirect to the file view
	if isNewFile {
		w.Header().Set("HX-Redirect", "/files/"+filePath)
		w.WriteHeader(http.StatusOK)
		return
	}

	// for existing file updates, show success message with link to file view
	successMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully")
	html := render.RenderStatusMessageWithLink(render.StatusOK, successMsg, "/files/"+filePath, filePath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Browse files by single metadata field
// @Tags files
// @Produce json,html
// @Param metadata query string true "Metadata field name"
// @Param value query string true "Metadata field value"
// @Success 200 {array} files.File
// @Router /api/files/browse [get]
func handleAPIBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadata := r.URL.Query().Get("metadata")
	value := r.URL.Query().Get("value")

	if metadata == "" || value == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing metadata or value parameter"), http.StatusBadRequest)
		return
	}

	logging.LogDebug("browse request: %s=%s", metadata, value)

	// Map URL-friendly field names to actual filter field names
	actualMetadata := metadata
	switch metadata {
	case "projects":
		actualMetadata = "para_projects"
	case "areas":
		actualMetadata = "para_areas"
	case "resources":
		actualMetadata = "para_resources"
	case "archive":
		actualMetadata = "para_archive"
	}

	// Set operator based on field type - arrays use "contains", simple fields use "equals"
	operator := "equals"
	if metadata == "tags" || metadata == "folders" ||
		metadata == "projects" || metadata == "areas" ||
		metadata == "resources" || metadata == "archive" {
		operator = "contains"
	}

	criteria := []filter.Criteria{
		{
			Metadata: actualMetadata,
			Operator: operator,
			Value:    value,
			Action:   "include",
		},
	}

	logging.LogDebug("browse criteria: metadata=%s (mapped to %s), operator=%s, value=%s", metadata, actualMetadata, operator, value)

	browsedFiles, err := filter.FilterFiles(criteria, "and")
	if err != nil {
		logging.LogError("failed to browse files: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to browse files"), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("browsed %d files for %s=%s", len(browsedFiles), metadata, value)

	html := render.RenderBrowseFilesHTML(browsedFiles)
	writeResponse(w, r, browsedFiles, html)
}

// @Summary Get metadata form HTML for file editing
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Produce html
// @Router /api/files/metadata/form [get]
func handleAPIGetMetadataFormHTML(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")

	html, err := render.RenderMetadataForm(filePath, "")
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to generate metadata form"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get file form HTML
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Produce html
// @Router /api/files/form [get]
func handleAPIFileForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	html := render.RenderFileForm(filePath)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get metadata form HTML
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Param filetype query string false "Default file type (optional for new files)"
// @Produce html
// @Router /api/files/metadata-form [get]
func handleAPIMetadataForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	defaultFiletype := r.URL.Query().Get("filetype")

	html, err := render.RenderMetadataForm(filePath, defaultFiletype)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to generate metadata form"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
