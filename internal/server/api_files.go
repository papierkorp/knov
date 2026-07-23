// Package server ..
package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/search"
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
	folderPaths, err := files.GetAllFolderPathsFromCache()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get cached folder paths, fallback to live data: %v", err)
		// fallback to live data
		folderPaths, err = files.GetAllFolderPaths()
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to get folder paths: %v", err)
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
		logging.LogError(logging.KeyApp, "failed to read folder %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read folder"), http.StatusInternalServerError)
		return
	}

	var folders []render.FolderEntry
	var filesInDir []render.FolderEntry

	for _, entry := range entries {
		// skip hidden files/folders (dot-prefixed) unless configured to show them
		if !configmanager.GetShowHiddenFiles() && strings.HasPrefix(entry.Name(), ".") {
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

// @Summary Get file overview (dates, hierarchy, links, related files)
// @Description Returns every metadata/link fragment used on a file's detail page (created/edited
// @Description dates, collection, folders, ancestors, kids, grandchildren, used/media/inbound
// @Description links, related files) in a single response, replacing the ~11 separate round trips
// @Description that page used to fire on every load. Keys are semantic field names, not
// @Description theme-specific DOM ids — the theme's own JS maps them onto its markup.
// @Tags files
// @Param filepath query string true "File path"
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/files/overview [get]
func handleAPIGetFileOverview(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	lang := configmanager.GetLanguage()
	result := map[string]string{}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		http.Error(w, translation.SprintfForRequest(lang, "failed to get metadata"), http.StatusInternalServerError)
		return
	}

	if metadata != nil {
		result["created"] = fmt.Sprintf(`<span class="createdat">%s</span>`, configmanager.FormatDateTime(metadata.CreatedAt))
		result["edited"] = fmt.Sprintf(`<span class="lastedited">%s</span>`, configmanager.FormatDateTime(metadata.LastEdited))
		result["collection"] = render.RenderMetadataLinkHTML(metadata.Collection, "collection")
		result["folders"] = render.RenderMetadataLinksHTML(metadata.Folders, "folders")

		if len(metadata.Ancestor) == 0 {
			result["ancestors"] = render.RenderNoLinksMessage("no ancestors")
		} else {
			result["ancestors"] = render.RenderLinksList(metadata.Ancestor, false)
		}

		if len(metadata.Kids) == 0 {
			result["kids"] = render.RenderNoLinksMessage(translation.SprintfForRequest(lang, "no children"))
		} else {
			result["kids"] = render.RenderKidsLinks(metadata.Kids)
		}

		var grandchildren []string
		for _, kid := range metadata.Kids {
			kidMeta, err := files.MetaDataGet(kid)
			if err != nil || kidMeta == nil {
				continue
			}
			grandchildren = append(grandchildren, kidMeta.Kids...)
		}
		if len(grandchildren) == 0 {
			result["grandchildren"] = render.RenderNoLinksMessage(translation.SprintfForRequest(lang, "no grandchildren"))
		} else {
			result["grandchildren"] = render.RenderLinksList(grandchildren, false)
		}

		if len(metadata.UsedLinks) == 0 {
			result["usedLinks"] = render.RenderNoLinksMessage(translation.SprintfForRequest(lang, "no outbound links"))
		} else {
			result["usedLinks"] = render.RenderUsedLinks(metadata.UsedLinks)
		}

		result["mediaLinks"] = render.RenderMediaLinks(metadata.UsedLinks)

		if len(metadata.LinksToHere) == 0 {
			result["linksFrom"] = render.RenderNoLinksMessage("no inbound links")
		} else {
			result["linksFrom"] = render.RenderLinksList(metadata.LinksToHere, false)
		}
	}

	relatedPaths, err := search.GetRelatedFiles(filePath, 5)
	if err != nil || len(relatedPaths) == 0 {
		result["related"] = render.RenderRelatedFiles(nil)
	} else {
		result["related"] = render.RenderRelatedFiles(relatedPaths)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
		logging.LogError(logging.KeyApp, "failed to get raw content: %v", err)
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
			logging.LogError(logging.KeyApp, "failed to create directory %s: %v", dir, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
			return
		}
	}

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to save file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}
	go git.CommitFile(fullPath)

	logging.LogInfo(logging.KeyApp, "saved file: %s", filePath)

	// create metadata for new files
	if isNewFile {
		editor := files.EditorType(formEditor)
		if editor == "" {
			editor = files.EditorTypeToastUI
		}

		metadata := &files.Metadata{
			Path:   pathutils.ToWithPrefix(filePath),
			Editor: editor,
		}

		// apply auto-create tags if configured
		if autoTags := configmanager.GetAutoCreateTags(); len(autoTags) > 0 {
			dir := files.FolderFromPath(filePath)
			var tagsToApply []string
			for _, at := range autoTags {
				if at.FolderPath == "" || pathutils.FolderContains(dir, at.FolderPath) {
					tagsToApply = append(tagsToApply, at.Tag)
				}
			}
			if len(tagsToApply) > 0 {
				metadata.Tags = append(metadata.Tags, tagsToApply...)
				logging.LogInfo(logging.KeyApp, "applied auto-create tags %v to new file: %s", tagsToApply, filePath)
			}
		}

		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError(logging.KeyApp, "failed to save metadata for new file %s: %v", filePath, err)
		} else {
			logging.LogInfo(logging.KeyApp, "created metadata for new file: %s (editor: %s)", filePath, editor)
		}
	} else {
		// update links for existing files
		normalizedPath := pathutils.ToWithPrefix(filePath)
		if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to update links for file %s: %v", filePath, err)
		}

		// update orphaned media cache for affected media files
		if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to update orphaned media cache: %v", err)
		}
	}

	// if this was a new file creation, redirect to the file view
	if isNewFile {
		w.Header().Set("HX-Redirect", pathutils.ToFileURL(filePath))
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file created"))
		writeResponse(w, r, map[string]string{"filepath": filePath}, "")
		return
	}

	// for existing file updates, send notify toast
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file saved"))
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file saved"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	writeResponse(w, r, map[string]string{"filepath": filePath}, render.RenderStatusMessage(render.StatusOK, successMsg))
}

// @Summary Cycle a todo checkbox's state in place from the rendered file view
// @Description Advances open -> done -> cancelled -> waiting -> open for the checkbox on the given line and returns the re-rendered file content
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param line formData int true "0-indexed source line of the checkbox"
// @Produce html
// @Router /api/files/todo-toggle [post]
func handleAPIToggleTodoState(w http.ResponseWriter, r *http.Request) {
	// htmx processes HX-Trigger toasts on every response, success or error, so notify
	// the user even though the failed request leaves the rendered view untouched.
	fail := func(status int, message string) {
		notify.SetHeader(w, notify.LevelError, message)
		http.Error(w, message, status)
	}

	if err := r.ParseForm(); err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	if filePath == "" {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}

	line, err := strconv.Atoi(r.FormValue("line"))
	if err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid line"))
		return
	}

	fullPath := pathutils.ToDocsPath(filePath)

	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	updated, err := parser.CycleTodoStateAtLine(content, line)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to cycle todo state for %s at line %d: %v", filePath, line, err)
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update todo state"))
		return
	}

	if err := contentStorage.WriteFile(fullPath, updated, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write file %s: %v", fullPath, err)
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}
	go git.CommitFile(fullPath)

	rendered, err := files.GetFileContent(fullPath)
	if err != nil {
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(rendered.HTML))
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
		logging.LogError(logging.KeyApp, "failed to read file %s: %v", fullPath, err)
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

	logging.LogInfo(logging.KeyApp, "exported file to markdown: %s", filePath)
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
			logging.LogWarning(logging.KeyApp, "failed to read file %s: %v", path, err)
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
		logging.LogError(logging.KeyApp, "failed to create zip archive: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// close zip writer
	err = zipWriter.Close()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to close zip writer: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// prepare download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("knov-export_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(buf.Bytes())

	logging.LogInfo(logging.KeyApp, "exported all files as zip: %s", filename)
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

	logging.LogInfo(logging.KeyDokuwikiExport, "export started: %s", dataPath)

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
			logging.LogWarning(logging.KeyDokuwikiExport, "skip (read error): %s — %v", relPath, err)
			return nil // skip this file but continue
		}

		// convert dokuwiki files to markdown
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".dokuwiki" || ext == ".txt" {
			markdown := dokuwikiconverter.NewWithFilePath(relPath).ConvertToMarkdown(string(content))
			content = []byte(markdown)
			oldRelPath := relPath
			relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".md"
			logging.LogDebug(logging.KeyDokuwikiExport, "converted: %s -> %s", oldRelPath, relPath)
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
		logging.LogError(logging.KeyApp, "failed to create zip archive: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// close zip writer
	err = zipWriter.Close()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to close zip writer: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export files"), http.StatusInternalServerError)
		return
	}

	// prepare download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("knov-export-markdown_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(buf.Bytes())

	logging.LogInfo(logging.KeyApp, "exported all files as zip with markdown conversion: %s", filename)
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

	logging.LogDebug(logging.KeyApp, "browse request: %s=%s", metadata, value)

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

	logging.LogDebug(logging.KeyApp, "browse criteria: metadata=%s (mapped to %s), operator=%s, value=%s", metadata, actualMetadata, operator, value)

	browsedFiles, err := filter.FilterFiles(criteria, "and")
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to browse files: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to browse files"), http.StatusInternalServerError)
		return
	}

	logging.LogDebug(logging.KeyApp, "browsed %d files for %s=%s", len(browsedFiles), metadata, value)

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
		logging.LogError(logging.KeyApp, "failed to generate metadata form: %v", err)
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
		logging.LogError(logging.KeyApp, "failed to generate metadata form: %v", err)
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
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		return
	}

	// get current file path from URL
	currentPath := strings.TrimPrefix(r.URL.Path, "/api/files/rename/")
	if currentPath == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}

	// get new name from form (can be full path or just filename)
	newName := r.FormValue("name")
	if newName == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "new file path is required"))
		return
	}

	// use the new name as the new path (allows for directory moves)
	newPath := filepath.Clean(newName)

	logging.LogInfo(logging.KeyApp, "renaming file: %s -> %s", currentPath, newPath)

	// check if current file exists
	currentFullPath := pathutils.ToDocsPath(currentPath)
	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"))
		return
	}

	// check if new path already exists
	newFullPath := pathutils.ToDocsPath(newPath)
	if _, err := os.Stat(newFullPath); err == nil {
		writeAPIError(w, http.StatusConflict, translation.SprintfForRequest(configmanager.GetLanguage(), "file with new name already exists"))
		return
	}

	// create directory for new path if needed
	newDir := filepath.Dir(newFullPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "failed to create directory %s: %v", newDir, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		return
	}

	// rename the file
	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to rename file %s -> %s: %v", currentPath, newPath, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rename file"))
		return
	}

	// update links in other files that reference this file
	if err := files.UpdateLinksForMovedFile(logging.KeyApp, currentPath, newPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update links for renamed file %s -> %s: %v", currentPath, newPath, err)
		// don't fail the operation for this, just log a warning
	}

	if err := git.InvalidateFileHistoryCache(currentPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to invalidate file history cache for %s: %v", currentPath, err)
	}

	logging.LogInfo(logging.KeyApp, "successfully renamed file: %s -> %s", currentPath, newPath)

	// redirect to the new file location
	w.Header().Set("HX-Redirect", pathutils.ToFileURL(newPath))
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file renamed"))
	writeResponse(w, r, map[string]string{"filepath": newPath}, "")
}

// @Summary Move a folder into another folder
// @Description Moves a folder to a new parent, updating all internal links
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param folderpath path string true "Current folder path (relative, no docs/ prefix)"
// @Param target formData string true "Target parent folder path"
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/files/move-folder/{folderpath} [post]
func handleAPIMoveFolderFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		return
	}

	currentPath := strings.TrimPrefix(r.URL.Path, "/api/files/move-folder/")
	if currentPath == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing folder path"))
		return
	}

	targetParent := r.FormValue("target")
	if targetParent == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "target folder is required"))
		return
	}

	folderName := r.FormValue("name")
	if folderName == "" {
		folderName = filepath.Base(currentPath)
	} else if strings.Contains(folderName, "/") || strings.Contains(folderName, "\\") {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "folder name must not contain path separators"))
		return
	}
	newPath := filepath.Clean(targetParent + "/" + folderName)

	if newPath == currentPath {
		writeResponse(w, r, map[string]string{"folderpath": newPath}, "")
		return
	}

	// prevent moving a folder into itself or a descendant
	if strings.HasPrefix(newPath+"/", currentPath+"/") {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "cannot move folder into itself"))
		return
	}

	currentFullPath := pathutils.ToDocsPath(currentPath)
	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "folder does not exist"))
		return
	}

	newFullPath := pathutils.ToDocsPath(newPath)
	if _, err := os.Stat(newFullPath); err == nil {
		writeAPIError(w, http.StatusConflict, translation.SprintfForRequest(configmanager.GetLanguage(), "folder with new name already exists"))
		return
	}

	// collect all files before the move so we can update their links
	var filesToUpdate []struct{ oldRel, newRel string }
	_ = filepath.Walk(currentFullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		oldRel := pathutils.ToRelative(p)
		suffix := strings.TrimPrefix(p, currentFullPath)
		newRel := pathutils.ToRelative(newFullPath + suffix)
		filesToUpdate = append(filesToUpdate, struct{ oldRel, newRel string }{oldRel, newRel})
		return nil
	})

	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		logging.LogError(logging.KeyApp, "failed to create parent directory for %s: %v", newFullPath, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		return
	}

	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to move folder %s -> %s: %v", currentPath, newPath, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move folder"))
		return
	}

	for _, f := range filesToUpdate {
		if err := files.UpdateLinksForMovedFileNoRefresh(logging.KeyApp, f.oldRel, f.newRel); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to update links for %s -> %s: %v", f.oldRel, f.newRel, err)
		}
	}
	if len(filesToUpdate) > 0 {
		files.RefreshCaches()
	}

	logging.LogInfo(logging.KeyApp, "successfully moved folder: %s -> %s (%d files updated)", currentPath, newPath, len(filesToUpdate))
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "folder moved"))
	writeResponse(w, r, map[string]string{"folderpath": newPath}, "")
}

// cleanupDeletedFileMetadata deletes a file's metadata, refreshes the
// aggregate caches, and commits the deletion to git. For a single deleted
// file (the common case).
func cleanupDeletedFileMetadata(fullPath string) {
	cleanupDeletedFileMetadataNoRefresh(fullPath)
	files.RefreshCaches()
	go git.CommitDeletedFile(fullPath)
}

// cleanupDeletedFileMetadataNoRefresh deletes a file's metadata without
// refreshing the aggregate caches or committing to git. Used when deleting
// many files in one request (folder delete, bulk delete) - the caller loops
// this, then does one files.RefreshCaches() and one batched
// git.CommitDeletedFiles() after the loop, instead of paying for a full cache
// rebuild and a separate git commit per file.
func cleanupDeletedFileMetadataNoRefresh(fullPath string) {
	relPath := pathutils.ToRelative(fullPath)
	if err := files.MetaDataDeleteNoRefresh(logging.KeyApp, relPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to delete metadata for %s: %v", relPath, err)
	}
	if err := git.InvalidateFileHistoryCache(relPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to invalidate file history cache for %s: %v", relPath, err)
	}
}

// removeFileAndMetadata removes a single file from disk, then cleans up its
// metadata and git history via cleanupDeletedFileMetadata.
func removeFileAndMetadata(fullPath string) error {
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	cleanupDeletedFileMetadata(fullPath)
	return nil
}

// removeFileAndMetadataNoRefresh is removeFileAndMetadata without the cache
// refresh/git commit. See cleanupDeletedFileMetadataNoRefresh.
func removeFileAndMetadataNoRefresh(fullPath string) error {
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	cleanupDeletedFileMetadataNoRefresh(fullPath)
	return nil
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
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}

	logging.LogInfo(logging.KeyApp, "deleting file: %s", filePath)

	// check if file exists
	fullPath := pathutils.ToDocsPath(filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"))
		return
	}

	// delete the file, its metadata, and commit the deletion to git
	if err := removeFileAndMetadata(fullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete file %s: %v", filePath, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete file"))
		return
	}

	logging.LogInfo(logging.KeyApp, "successfully deleted file: %s", filePath)

	// redirect to browse or home page
	w.Header().Set("HX-Redirect", "/browse")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file deleted"))
	writeResponse(w, r, map[string]string{"status": "deleted"}, "")
}

// @Summary Delete a folder
// @Description Recursively deletes a folder, all files inside it, and their metadata
// @Tags files
// @Param folderpath path string true "Folder path to delete (relative, no docs/ prefix)"
// @Produce html
// @Success 200 {string} string "success message"
// @Router /api/files/delete-folder/{folderpath} [delete]
func handleAPIDeleteFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := strings.TrimPrefix(r.URL.Path, "/api/files/delete-folder/")
	if folderPath == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing folder path"))
		return
	}

	fullPath := pathutils.ToDocsPath(folderPath)
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) || !info.IsDir() {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "folder does not exist"))
		return
	}

	logging.LogInfo(logging.KeyApp, "deleting folder: %s", folderPath)

	// collect every file inside so we can clean up metadata and git afterwards,
	// since os.RemoveAll below removes them before we get a chance to look
	var filesInFolder []string
	_ = filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		filesInFolder = append(filesInFolder, p)
		return nil
	})

	if err := os.RemoveAll(fullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete folder %s: %v", folderPath, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete folder"))
		return
	}

	for _, filePath := range filesInFolder {
		cleanupDeletedFileMetadataNoRefresh(filePath)
	}
	if len(filesInFolder) > 0 {
		files.RefreshCaches()
		go func() {
			if err := git.CommitDeletedFiles(filesInFolder); err != nil {
				logging.LogError(logging.KeyApp, "failed to commit deleted folder %s: %v", folderPath, err)
			}
		}()
	}

	logging.LogInfo(logging.KeyApp, "successfully deleted folder: %s (%d files)", folderPath, len(filesInFolder))

	w.Header().Set("HX-Redirect", "/browse")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "folder deleted"))
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
		logging.LogError(logging.KeyApp, "failed to get all files: %v", err)
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError,
			translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files")))
		return
	}

	deleted := 0
	var deletedFullPaths []string
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

		fullPath := pathutils.ToDocsPath(pathutils.ToRelative(file.Path))
		if err := removeFileAndMetadataNoRefresh(fullPath); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to delete file %s: %v", fullPath, err)
			continue
		}
		deletedFullPaths = append(deletedFullPaths, fullPath)
		deleted++
	}

	if deleted > 0 {
		files.RefreshCaches()
		go func() {
			if err := git.CommitDeletedFiles(deletedFullPaths); err != nil {
				logging.LogError(logging.KeyApp, "failed to commit bulk deleted files (%s=%s): %v", groupType, value, err)
			}
		}()
	}

	logging.LogInfo(logging.KeyApp, "bulk deleted %d files from %s=%s", deleted, groupType, value)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "deleted %d files", deleted))
	w.Header().Set("HX-Redirect", "/browse/"+groupType)
	writeResponse(w, r, map[string]int{"deleted": deleted}, "")
}

// @Summary Get headers (TOC) for a file
// @Description Returns all headings from a file for use in wiki link anchor autocomplete
// @Tags files
// @Param filepath query string true "relative file path"
// @Produce json
// @Success 200 {array} object "array of {id, text, level}"
// @Failure 400 {string} string "missing filepath"
// @Failure 404 {string} string "file not found"
// @Router /api/files/headers [get]
func handleAPIFilesHeaders(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimSpace(r.URL.Query().Get("filepath"))
	if filePath == "" {
		http.Error(w, "missing filepath", http.StatusBadRequest)
		return
	}

	fullPath := pathutils.ToDocsPath(filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}

	rendered, err := handler.Render(content, fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	toc := parser.GenerateTOC(string(rendered))

	type headerResult struct {
		ID    string `json:"id"`
		Text  string `json:"text"`
		Level int    `json:"level"`
	}

	results := make([]headerResult, 0, len(toc))
	for _, item := range toc {
		results = append(results, headerResult{ID: item.ID, Text: item.Text, Level: item.Level})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// @Summary Autocomplete file paths
// @Description Returns files matching a query string for use in wiki link autocomplete
// @Tags files
// @Param q query string false "search query"
// @Produce json
// @Success 200 {array} object "array of {path, filename}"
// @Router /api/files/autocomplete [get]
func handleAPIFilesAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type result struct {
		Path     string `json:"path"`
		Filename string `json:"filename"`
	}

	results := make([]result, 0, 20)
	for _, f := range allFiles {
		rel := pathutils.ToRelative(f.Path)
		if q == "" || strings.Contains(strings.ToLower(rel), q) {
			results = append(results, result{
				Path:     rel,
				Filename: filepath.Base(rel),
			})
			if len(results) >= 20 {
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
