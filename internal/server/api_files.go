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
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/mapping"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/utils"
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
			// check if file type should be hidden
			metadata, _ := files.MetaDataGet(entryPath)
			if metadata != nil && configmanager.IsFileTypeHidden(string(metadata.FileType)) {
				continue // skip this file if its type is hidden
			}
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

	// filter out hidden file types
	allFiles = files.FilterFilesByHiddenTypes(allFiles)

	format := r.URL.Query().Get("format")

	if format == "options" {
		cachedFilePaths, err := files.GetAllFilePathsFromSystemData()
		if err != nil {
			logging.LogError("failed to get cached file paths, fallback to live data: %v", err)
			// fallback to live data
			cachedFilePaths = make([]string, len(allFiles))
			for i, file := range allFiles {
				cachedFilePaths[i] = file.Path
			}
		}

		html := render.RenderFilesOptionsFromPaths(cachedFilePaths)
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
	formFiletype := r.FormValue("filetype")
	content := r.FormValue("content")

	// auto-generate filepath for fleeting files
	if filePath == "" && formFiletype == "fleeting" {
		// generate unique filename from first line of content
		filename := generateUniqueFleetingFilename(content)
		filePath = fmt.Sprintf("fleeting/%s.md", filename)
		logging.LogInfo("auto-generated fleeting filepath: %s", filePath)
	}

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	// check file extension and add .md if missing for markdown files
	if !strings.Contains(filePath, ".") {
		// no extension provided, add appropriate extension based on filetype
		switch formFiletype {
		case "todo", "fleeting", "literature", "permanent", "moc":
			filePath = filePath + ".md"
		case "filter":
			filePath = filePath + ".filter"
		case "journaling":
			filePath = filePath + ".list"
		default:
			filePath = filePath + ".md"
		}
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

	logging.LogInfo("saved file: %s", filePath)

	// create metadata for new files
	if isNewFile {
		// determine filetype based on form parameter or default to permanent
		var filetype files.Filetype = files.FileTypePermanent // default

		formFiletype := r.FormValue("filetype")
		if formFiletype != "" {
			switch formFiletype {
			case "todo":
				filetype = files.FileTypeTodo
			case "fleeting":
				filetype = files.FileTypeFleeting
			case "literature":
				filetype = files.FileTypeLiterature
			case "permanent":
				filetype = files.FileTypePermanent
			case "moc":
				filetype = files.FileTypeMOC
			case "filter":
				filetype = files.FileTypeFilter
			case "journaling":
				filetype = files.FileTypeJournaling
			default:
				filetype = files.FileTypePermanent
			}
		}

		metadata := &files.Metadata{
			Path:     pathutils.ToWithPrefix(filePath),
			FileType: filetype,
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save metadata for new file %s: %v", filePath, err)
			// don't fail the whole request, just log the error
		} else {
			logging.LogInfo("created metadata for new file: %s (filetype: %s)", filePath, filetype)
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
		w.WriteHeader(http.StatusOK)
		return
	}

	// for existing file updates, show success message with link to file view
	successMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully")
	html := render.RenderStatusMessageWithLink(render.StatusOK, successMsg, "/files/"+filePath, filePath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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

	// get parser handler
	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusBadRequest)
		return
	}

	// check if it's a dokuwiki handler
	dokuwikiHandler, ok := handler.(*parser.DokuwikiHandler)
	if !ok {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "markdown export only supported for dokuwiki files"), http.StatusBadRequest)
		return
	}

	// read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	// convert to markdown
	markdown := dokuwikiHandler.ConvertToMarkdown(string(content))

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

		// check if file is dokuwiki and convert to markdown
		handler := parser.GetParserRegistry().GetHandler(path)
		if handler != nil && handler.Name() == "dokuwiki" {
			dokuwikiHandler, ok := handler.(*parser.DokuwikiHandler)
			if ok {
				// convert to markdown
				markdown := dokuwikiHandler.ConvertToMarkdown(string(content))
				content = []byte(markdown)

				// change extension to .md
				relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".md"

				logging.LogDebug("converted dokuwiki file to markdown: %s", relPath)
			}
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
	w.WriteHeader(http.StatusOK)
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
	w.WriteHeader(http.StatusOK)
}

// generateUniqueFleetingFilename creates a unique sanitized filename, adding numbers if duplicates exist
func generateUniqueFleetingFilename(content string) string {
	// get base filename from content
	baseFilename := utils.SanitizeFilename(content, 20, false, true)
	filename := baseFilename
	counter := 2

	// check if file exists and increment counter until we find a unique name
	for {
		testPath := fmt.Sprintf("fleeting/%s.md", filename)
		fullTestPath := pathutils.ToDocsPath(testPath)

		// check if file exists
		if _, err := os.Stat(fullTestPath); os.IsNotExist(err) {
			// file doesn't exist, we can use this filename
			break
		}

		// file exists, try with counter
		filename = fmt.Sprintf("%s-%d", baseFilename, counter)
		counter++

		// safety check to prevent infinite loop
		if counter > 100 {
			// fallback to timestamp if we somehow can't find a unique name
			logging.LogWarning("could not generate unique fleeting filename after 100 attempts, falling back to timestamp")
			return time.Now().Format("2006-01-02-150405")
		}
	}

	return filename
}
