// Package server ..
package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/mapping"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get folder path suggestions for datalist
// @Description Returns folder path suggestions for file creation form
// @Tags files
// @Produce html
// @Success 200 {string} string "datalist options html"
// @Router /api/files/folder-suggestions [get]
func handleAPIGetFolderSuggestions(w http.ResponseWriter, r *http.Request) {
	// get cached folder paths, fallback to live data if needed
	folderPaths, err := files.GetAllFolderPathsFromSystemData()
	if err != nil {
		logging.LogError("failed to get cached folder paths, fallback to live data: %v", err)
		// fallback to live data
		folderPaths, err = files.GetAllFolderPaths()
		if err != nil {
			logging.LogError("failed to get folder paths: %v", err)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(""))
			return
		}
	}

	var html strings.Builder
	for _, folderPath := range folderPaths {
		// add suggestion with placeholder filename
		suggestion := folderPath
		html.WriteString(fmt.Sprintf(`<option value="%s"></option>`, suggestion))
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html.String()))
}

// @Summary Get folder structure
// @Tags files
// @Param path query string false "folder path (root if empty)"
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/files/folder [get]
func handleAPIGetFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("path")
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "#folder-content"
	}

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
		// skip hidden files/folders (dot-prefixed) unless configured to show them
		if !configmanager.GetAppConfig().ShowHiddenFiles && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryPath := filepath.Join(folderPath, entry.Name())
		item := render.FolderEntry{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			folders = append(folders, item)
		} else {
			// check if file type should be hidden
			metadata, _ := files.MetaDataGet(entryPath)
			if metadata != nil && configmanager.IsFileTypeHidden(string(metadata.Editor)) {
				continue // skip this file if its type is hidden
			}
			filesInDir = append(filesInDir, item)
		}
	}

	html := render.RenderFolderContent(folderPath, folders, filesInDir, target)
	writeResponse(w, r, map[string]interface{}{
		"path":    folderPath,
		"folders": folders,
		"files":   filesInDir,
	}, html)
}

// @Summary Get file tree overview
// @Description Returns all files as an indented folder tree structure
// @Tags files
// @Produce json,html
// @Router /api/files/tree [get]
func handleAPIGetFileTree(w http.ResponseWriter, r *http.Request) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}
	allFiles = files.FilterByVisibility(allFiles)
	tree := files.BuildFileTree(allFiles)
	html := render.RenderTreeOverview(tree, r.URL.Query().Get("actions") == "true")
	writeResponse(w, r, allFiles, html)
}

// @Summary Get all files
// @Tags files
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		cachedFilePaths, err := files.GetAllFilePathsFromSystemData()
		if err != nil {
			logging.LogError("failed to get cached file paths: %v", err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
			return
		}
		html := render.RenderFilesOptionsFromPaths(cachedFilePaths)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
		return
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}

	allFiles = files.FilterByVisibility(allFiles)

	if format == "datalist" {
		html := render.RenderFilesDatalist(allFiles)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
		return
	}

	html := render.RenderFilesList(allFiles, r.URL.Query().Get("actions") == "true")
	writeResponse(w, r, allFiles, html)
}

// @Summary Get file content as html
// @Tags files
// @Param filepath path string true "File path"
// @Produce text/html
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/content/")
	fullPath := pathutils.ToDocsPath(filePath)

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

	fullPath := pathutils.ToDocsPath(filepath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to get raw content: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get raw content"), http.StatusInternalServerError)
		return
	}

	data := map[string]string{"content": string(content)}
	writeResponse(w, r, data, string(content))
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
	formEditor := r.FormValue("editor")
	content := r.FormValue("content")

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	if filepath.Ext(filePath) == "" {
		filePath = filePath + configmanager.ExtensionForEditor(formEditor)
	}

	fullPath := pathutils.ToDocsPath(filePath)

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
	go git.CommitFile(fullPath)

	logging.LogInfo("saved file: %s", filePath)

	// create metadata for new files
	if isNewFile {
		editor := files.EditorType(formEditor)
		if editor == "" {
			editor = files.EditorTypeMarkdown
		}

		metadata := &files.Metadata{
			Path:   pathutils.ToWithPrefix(filePath),
			Editor: editor,
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save metadata for new file %s: %v", filePath, err)
		} else {
			logging.LogInfo("created metadata for new file: %s (editor: %s)", filePath, editor)
		}
	} else {
		// update links for existing files
		normalizedPath := pathutils.ToWithPrefix(filePath)
		if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
			logging.LogWarning("failed to update links for file %s: %v", filePath, err)
		}

		// update orphaned media cache for affected media files
		if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
			logging.LogWarning("failed to update orphaned media cache: %v", err)
		}
	}

	// if this was a new file creation, redirect to the file view
	if isNewFile {
		w.Header().Set("HX-Redirect", "/files/"+filePath)
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file created"))
		writeResponse(w, r, map[string]string{"filepath": filePath}, "")
		return
	}

	// for existing file updates, send notify toast
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file saved"))
	writeResponse(w, r, map[string]string{"filepath": filePath}, "")
}

// @Summary Export file to markdown
// @Description Export dokuwiki file to markdown format
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce text/markdown
// @Param filepath query string true "File path"
// @Success 200 {file} file "markdown file"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/markdown [get]
func handleAPIExportToMarkdown(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fullPath := pathutils.ToDocsPath(filePath)

	// read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	// convert to markdown
	markdown := dokuwikiconverter.NewWithFilePath(filePath).ConvertToMarkdown(string(content))

	// prepare download
	filename := filepath.Base(filePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".md"

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte(markdown))

	logging.LogInfo("exported file to markdown: %s", filePath)
}

// @Summary Export all files as zip
// @Description Export all files from data directory as a zip archive
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce application/zip
// @Success 200 {file} file "zip archive"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/zip [post]
func handleAPIExportAllFiles(w http.ResponseWriter, r *http.Request) {
	dataPath := configmanager.GetAppConfig().DataPath

	// create zip in memory
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// walk through data directory
	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// skip directories
		if info.IsDir() {
			return nil
		}

		// get relative path
		relPath, err := filepath.Rel(dataPath, path)
		if err != nil {
			return err
		}

		// read file content
		content, err := os.ReadFile(path)
		if err != nil {
			logging.LogWarning("failed to read file %s: %v", path, err)
			return nil // skip this file but continue
		}

		// add file to zip
		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		_, err = zipFile.Write(content)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logging.LogError("failed to create zip archive: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// close zip writer
	err = zipWriter.Close()
	if err != nil {
		logging.LogError("failed to close zip writer: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// prepare download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("knov-export_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(buf.Bytes())

	logging.LogInfo("exported all files as zip: %s", filename)
}

// @Summary Export all files with dokuwiki to markdown conversion
// @Description Export all files from data directory as a zip archive, converting dokuwiki files to markdown
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce application/zip
// @Success 200 {file} file "zip archive"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/markdown-converted [post]
func handleAPIExportAllFilesWithMarkdownConversion(w http.ResponseWriter, r *http.Request) {
	exportLog := logging.LogBuilder("dokuwiki_export")
	dataPath := configmanager.GetAppConfig().DataPath

	// create zip in memory
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	exportLog.Printf("=== export started: %s ===", dataPath)

	// walk through data directory
	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// skip directories
		if info.IsDir() {
			return nil
		}

		// get relative path
		relPath, err := filepath.Rel(dataPath, path)
		if err != nil {
			return err
		}

		// read file content
		content, err := os.ReadFile(path)
		if err != nil {
			logging.LogWarning("failed to read file %s: %v", path, err)
			exportLog.Printf("skip (read error): %s — %v", relPath, err)
			return nil // skip this file but continue
		}

		// convert dokuwiki files to markdown
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".dokuwiki" || ext == ".txt" {
			markdown := dokuwikiconverter.NewWithFilePath(relPath).ConvertToMarkdown(string(content))
			content = []byte(markdown)
			oldRelPath := relPath
			relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".md"
			exportLog.Printf("converted: %s -> %s", oldRelPath, relPath)
		}

		// add file to zip
		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		_, err = zipFile.Write(content)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logging.LogError("failed to create zip archive: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// close zip writer
	err = zipWriter.Close()
	if err != nil {
		logging.LogError("failed to close zip writer: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// prepare download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("knov-export-markdown_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(buf.Bytes())

	logging.LogInfo("exported all files as zip with markdown conversion: %s", filename)
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

	// map URL-friendly field names to database field names
	actualMetadata := mapping.URLToDatabase(metadata)

	// set operator based on field type - arrays use "contains", simple fields use "equals"
	operator := "equals"
	if mapping.IsArrayField(metadata) {
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

	html := render.RenderBrowseFilesHTML(browsedFiles, r.URL.Query().Get("actions") == "true")
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
	defaultFiletype := r.URL.Query().Get("editor")

	html, err := render.RenderMetadataForm(filePath, defaultFiletype)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to generate metadata form"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Rename a file
// @Description Renames a file and updates all links pointing to it
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "Current file path"
// @Param name formData string true "New file name"
// @Produce html
// @Success 200 {string} string "success message"
// @Router /api/files/rename/{filepath} [post]
func handleAPIRenameFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(html))
		return
	}

	// get current file path from URL
	currentPath := strings.TrimPrefix(r.URL.Path, "/api/files/rename/")
	if currentPath == "" {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(html))
		return
	}

	// get new name from form (can be full path or just filename)
	newName := r.FormValue("name")
	if newName == "" {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "new file path is required"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(html))
		return
	}

	// use the new name as the new path (allows for directory moves)
	newPath := filepath.Clean(newName)

	logging.LogInfo("renaming file: %s -> %s", currentPath, newPath)

	// check if current file exists
	currentFullPath := pathutils.ToDocsPath(currentPath)
	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(html))
		return
	}

	// check if new path already exists
	newFullPath := pathutils.ToDocsPath(newPath)
	if _, err := os.Stat(newFullPath); err == nil {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "file with new name already exists"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(html))
		return
	}

	// create directory for new path if needed
	newDir := filepath.Dir(newFullPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", newDir, err)
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(html))
		return
	}

	// rename the file
	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		logging.LogError("failed to rename file %s -> %s: %v", currentPath, newPath, err)
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rename file"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(html))
		return
	}

	// update links in other files that reference this file
	if err := files.UpdateLinksForMovedFile(currentPath, newPath); err != nil {
		logging.LogWarning("failed to update links for renamed file %s -> %s: %v", currentPath, newPath, err)
		// don't fail the operation for this, just log a warning
	}

	logging.LogInfo("successfully renamed file: %s -> %s", currentPath, newPath)

	// redirect to the new file location
	w.Header().Set("HX-Redirect", "/files/"+newPath)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file renamed"))
	writeResponse(w, r, map[string]string{"filepath": newPath}, "")
}

// @Summary Delete a file
// @Description Deletes a file and its metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "File path to delete"
// @Produce html
// @Success 200 {string} string "success message"
// @Router /api/files/delete/{filepath} [delete]
func handleAPIDeleteFile(w http.ResponseWriter, r *http.Request) {
	// get file path from URL
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/delete/")
	if filePath == "" {
		errorHTML := `<div class="status-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path") + `</div>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(errorHTML))
		return
	}

	logging.LogInfo("deleting file: %s", filePath)

	// check if file exists
	fullPath := pathutils.ToDocsPath(filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		errorHTML := `<div class="status-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist") + `</div>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(errorHTML))
		return
	}

	// delete the file
	if err := os.Remove(fullPath); err != nil {
		logging.LogError("failed to delete file %s: %v", filePath, err)
		errorHTML := `<div class="status-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete file") + `</div>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(errorHTML))
		return
	}

	// delete metadata
	if err := files.MetaDataDelete(filePath); err != nil {
		logging.LogWarning("failed to delete metadata for %s: %v", filePath, err)
		// don't fail the operation for this, just log a warning
	}

	logging.LogInfo("successfully deleted file: %s", filePath)

	// redirect to browse or home page
	w.Header().Set("HX-Redirect", "/browse")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file deleted"))
	writeResponse(w, r, map[string]string{"status": "deleted"}, "")
}

// @Summary Delete all files in a collection or folder
// @Description Deletes all files belonging to a specific collection or folder, including their metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param type query string true "Type to delete by: collection or folder"
// @Param value query string true "Collection or folder name"
// @Produce html
// @Success 200 {string} string "deleted N files"
// @Failure 400 {string} string "missing parameters"
// @Failure 500 {string} string "delete failed"
// @Router /api/files/bulk [delete]
func handleAPIDeleteFilesBulk(w http.ResponseWriter, r *http.Request) {
	groupType := r.URL.Query().Get("type")
	value := r.URL.Query().Get("value")

	if groupType == "" || value == "" {
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError,
			translation.SprintfForRequest(configmanager.GetLanguage(), "missing type or value parameter")))
		return
	}

	if groupType != "collection" && groupType != "folder" && groupType != "tag" {
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError,
			translation.SprintfForRequest(configmanager.GetLanguage(), "type must be collection, folder or tag")))
		return
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		logging.LogError("failed to get all files: %v", err)
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError,
			translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files")))
		return
	}

	deleted := 0
	for _, file := range allFiles {
		meta, err := files.MetaDataGet(file.Path)
		if err != nil || meta == nil {
			continue
		}

		match := false
		switch groupType {
		case "collection":
			match = meta.Collection == value
		case "folder":
			for _, f := range meta.Folders {
				if f == value {
					match = true
					break
				}
			}
		case "tag":
			for _, t := range meta.Tags {
				if t == value {
					match = true
					break
				}
			}
		}

		if !match {
			continue
		}

		relPath := pathutils.ToRelative(file.Path)
		fullPath := pathutils.ToDocsPath(relPath)

		if err := os.Remove(fullPath); err != nil {
			logging.LogWarning("failed to delete file %s: %v", relPath, err)
			continue
		}
		if err := files.MetaDataDelete(file.Path); err != nil {
			logging.LogWarning("failed to delete metadata for %s: %v", file.Path, err)
		}
		deleted++
	}

	logging.LogInfo("bulk deleted %d files from %s=%s", deleted, groupType, value)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "deleted %d files", deleted))
	w.Header().Set("HX-Redirect", "/browse/"+groupType)
	writeResponse(w, r, map[string]int{"deleted": deleted}, "")
}
