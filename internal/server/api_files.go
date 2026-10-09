// Package server ..
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/mapping"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/search"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get folder path suggestions
// @Description Returns folder paths matching a query string for use in path autocomplete
// @Tags files
// @Param q query string false "search query"
// @Produce json,html
// @Success 200 {array} object "array of {value, label, detail}"
// @Router /api/files/folder-suggestions [get]
func handleAPIGetFolderSuggestions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	// get cached folder paths, fallback to live data if needed
	folderPaths, err := files.GetAllFolderPathsFromCache()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get cached folder paths, fallback to live data: %v", err)
		// fallback to live data
		folderPaths, err = files.GetAllFolderPaths()
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to get folder paths: %v", err)
			writeResponse(w, r, []render.AutocompleteItem{}, "")
			return
		}
	}

	matches := files.RankAutocompleteMatches(folderPaths, q, 20)
	results := make([]render.AutocompleteItem, len(matches))
	for i, folderPath := range matches {
		results[i] = render.AutocompleteItem{
			Value:  folderPath,
			Label:  filepath.Base(strings.TrimSuffix(folderPath, "/")),
			Detail: folderPath,
		}
	}

	writeResponse(w, r, results, render.RenderAutocompleteList(results))
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
	if !pathutils.PathContains(dataPath, fullPath) {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid path"))
		return
	}

	// read directory
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to read folder %s: %v", fullPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read folder"))
		return
	}

	var folders []render.FolderEntry
	var filesInDir []render.FolderEntry
	hide := configmanager.NewHideMatcher(configmanager.HideScopeBrowse)

	for _, entry := range entries {
		// skip hidden files/folders (dot-prefixed) unless configured to show them
		if !configmanager.GetShowHiddenFiles() && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryPath := path.Join(folderPath, entry.Name())
		item := render.FolderEntry{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			if hide.PathHidden(pathutils.ToSlash(entryPath)) {
				continue // skip this folder if its path matches a configured hide-path pattern
			}
			folders = append(folders, item)
		} else {
			metadata, _ := files.MetaDataGet(pathutils.GuessMeta(entryPath))
			if files.IsHidden(files.File{Path: pathutils.GuessMeta(pathutils.ToSlash(entryPath)), Metadata: metadata}, hide) {
				continue
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
// @Produce json,html
// @Failure 404 {string} string "file not found"
// @Failure 500 {string} string "failed to get file content"
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := pathutils.DocsPath(strings.TrimPrefix(r.URL.Path, "/api/files/content/")).String()
	fullPath := pathutils.ToDocsPath(filePath)

	content, err := files.GetFileContent(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file not found"))
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	writeResponse(w, r, content, content.HTML)
}

// @Summary Get file header with link and breadcrumb
// @Tags files
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/files/header [get]
func handleAPIGetFileHeader(w http.ResponseWriter, r *http.Request) {
	filepath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filepath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	data := map[string]string{
		"filepath": filepath.String(),
		"link":     pathutils.ToFileURL(filepath),
	}

	html := render.RenderFileHeader(filepath.String())
	writeResponse(w, r, data, html)
}

// @Summary Get file view links
// @Description Returns a link per view of the file (e.g. rendered/raw, or a tracker's statistics/counters), marking the active one
// @Tags files
// @Param filepath query string true "File path"
// @Param view query string false "Active view id (default: the file type's first view)"
// @Produce json,html
// @Success 200 {array} render.FileViewLink
// @Router /api/files/views [get]
func handleAPIGetFileViews(w http.ResponseWriter, r *http.Request) {
	fp, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	links := render.FileViewLinks(pathutils.ToRelative(fp.String()), r.URL.Query().Get("view"))
	writeResponse(w, r, links, render.RenderFileViewLinks(links))
}

// @Summary Get file overview (dates, hierarchy, links, related files)
// @Description Returns every metadata/link fragment used on a file's detail page (created/edited
// @Description dates, collection, folders, ancestors, kids, grandchildren, used/media/inbound
// @Description links, related files, files in the same folder, files sharing a tag) in a single
// @Description response, replacing the ~11 separate round trips that page used to fire on every
// @Description load. Keys are semantic field names, not theme-specific DOM ids — the theme's own
// @Description JS maps them onto its markup.
// @Tags files
// @Param filepath query string true "File path"
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/files/overview [get]
func handleAPIGetFileOverview(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	lang := configmanager.GetLanguage()
	result := map[string]string{}

	metadata, err := files.MetaDataGet(filePath)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(lang, "failed to get metadata"))
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
			kidMeta, err := files.MetaDataGet(pathutils.GuessMeta(kid))
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

	relatedPaths, err := search.GetRelatedFiles(filePath.String(), 5)
	if err != nil || len(relatedPaths) == 0 {
		result["related"] = render.RenderRelatedFiles(nil)
	} else {
		result["related"] = render.RenderRelatedFiles(relatedPaths)
	}

	sameFolderPaths, err := files.GetFilesInSameFolder(filePath, 5)
	if err != nil || len(sameFolderPaths) == 0 {
		result["sameFolder"] = render.RenderSameFolderFiles(nil)
	} else {
		result["sameFolder"] = render.RenderSameFolderFiles(sameFolderPaths)
	}

	sameTagPaths, err := files.GetFilesWithSameTags(filePath, 5)
	if err != nil || len(sameTagPaths) == 0 {
		result["sameTags"] = render.RenderSameTagFiles(nil)
	} else {
		result["sameTags"] = render.RenderSameTagFiles(sameTagPaths)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// @Summary Get raw file content
// @Description Returns unprocessed file content for editing
// @Tags files
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string "raw content"
// @Router /api/files/raw [get]
func handleAPIGetRawContent(w http.ResponseWriter, r *http.Request) {
	filepath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filepath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	fullPath := pathutils.ToDocsPath(filepath.String())
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get raw content: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get raw content"))
		return
	}

	data := map[string]string{"content": string(content)}
	writeResponse(w, r, data, render.RenderRawContent(content))
}

// @Summary Save file content
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "File path"
// @Param content formData string true "File content"
// @Produce json,html
// @Router /api/files/save [post]
func handleAPIFileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	formEditor := r.FormValue("editor")
	content := r.FormValue("content")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}
	filePath = pathutils.DocsPath(filePath).String()

	// new files need a markdown extension, a dot in the name (e.g. "v1.2 notes") is not one
	if _, err := os.Stat(pathutils.ToDocsPath(filePath)); os.IsNotExist(err) && !parser.IsMarkdownExtension(filePath) {
		filePath = filePath + configmanager.ExtensionForEditor(formEditor)
	}

	if writeNewPathError(w, r, pathutils.CheckNewDocsPath(filePath)) {
		return
	}
	fullPath := pathutils.ToDocsPath(filePath)

	// check if file exists (to determine if this is creation or update)
	_, statErr := os.Stat(fullPath)
	isNewFile := os.IsNotExist(statErr)

	if err := contentStorage.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to save file %s: %v", fullPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}
	go git.CommitFile(fullPath)

	logging.LogInfo(logging.KeyApp, "saved file: %s", filePath)

	// create metadata for new files
	if isNewFile {
		editor := files.EditorType(formEditor)
		if editor == "" {
			editor = files.EditorType(configmanager.DefaultMarkdownEditor.Get())
		}

		normalizedPath := pathutils.GuessMeta(filePath)
		if err := files.MetaDataSync(normalizedPath); err != nil {
			logging.LogError(logging.KeyApp, "failed to save metadata for new file %s: %v", filePath, err)
		} else if err := files.SetEditor(normalizedPath, editor); err != nil {
			logging.LogError(logging.KeyApp, "failed to set editor for new file %s: %v", filePath, err)
		} else {
			logging.LogInfo(logging.KeyApp, "created metadata for new file: %s (editor: %s)", filePath, editor)
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
				if err := files.SetTags(normalizedPath, tagsToApply); err != nil {
					logging.LogError(logging.KeyApp, "failed to apply auto-create tags for %s: %v", filePath, err)
				} else {
					logging.LogInfo(logging.KeyApp, "applied auto-create tags %v to new file: %s", tagsToApply, filePath)
				}
			}
		}
	} else {
		// update links for existing files
		normalizedPath := pathutils.GuessMeta(filePath)
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
		w.Header().Set("HX-Redirect", pathutils.ToFileURL(pathutils.GuessMeta(filePath)))
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file created"))
		writeResponse(w, r, map[string]string{"filepath": filePath}, "")
		return
	}

	// for existing file updates, send notify toast
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file saved"))
	writeResponse(w, r, map[string]string{"filepath": filePath}, render.RenderStatusMessageWithLink(render.StatusOK,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file saved"),
		pathutils.ToFileURL(pathutils.GuessMeta(filePath)),
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file")))
}

// @Summary Cycle a todo checkbox's state in place from the rendered file view
// @Description Advances open -> done -> cancelled -> waiting -> open for the checkbox on the given line and persists it; the client applies the state change itself and only uses this to save. Returns the " (YYYY-MM-DD)" date stamp text applied (empty if date stamping is off), so the client never has to compute "today" itself
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param line formData int true "0-indexed source line of the checkbox"
// @Produce json,html
// @Router /api/files/todo-toggle [post]
func handleAPIToggleTodoState(w http.ResponseWriter, r *http.Request) {
	// htmx processes HX-Trigger toasts on every response, success or error, so notify
	// the user even though the failed request leaves the rendered view untouched.
	fail := func(status int, message string) {
		writeAPIError(w, r, status, message)
	}

	if err := r.ParseForm(); err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filePath == "" {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}

	line, err := strconv.Atoi(r.FormValue("line"))
	if err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid line"))
		return
	}

	fullPath := pathutils.ToDocsPath(filePath.String())

	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	updated, date, err := parser.CycleTodoStateAtLine(content, line)
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

	// the date stamp (if any) is the authoritative source for the client to display -
	// it applies the state change optimistically but only stamps the date once it has
	// this, so it never has to guess "today" itself in the browser's own timezone
	dateText := ""
	if date != "" {
		dateText = " (" + date + ")"
	}
	writeResponse(w, r, map[string]string{"filepath": filePath.String(), "date": dateText}, dateText)
}

// @Summary Remove a todo checkbox's date stamp from the rendered file view
// @Description Removes any trailing " (YYYY-MM-DD)" date stamp from the checkbox on the given line without changing its state
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param line formData int true "0-indexed source line of the checkbox"
// @Produce json,html
// @Router /api/files/todo-cleardate [post]
func handleAPIClearTodoDate(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, message string) {
		writeAPIError(w, r, status, message)
	}

	if err := r.ParseForm(); err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filePath == "" {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}

	line, err := strconv.Atoi(r.FormValue("line"))
	if err != nil {
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid line"))
		return
	}

	fullPath := pathutils.ToDocsPath(filePath.String())

	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	updated, err := parser.ClearTodoDateAtLine(content, line)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to clear todo date for %s at line %d: %v", filePath, line, err)
		fail(http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update todo state"))
		return
	}

	if err := contentStorage.WriteFile(fullPath, updated, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write file %s: %v", fullPath, err)
		fail(http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}
	go git.CommitFile(fullPath)

	writeResponse(w, r, map[string]string{"filepath": filePath.String()}, "")
}

// @Summary Export file to markdown
// @Description Convert a dokuwiki file to markdown, or compose a `.book` file into one document, and download it
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce text/markdown
// @Param filepath query string true "File path"
// @Success 200 {file} file "markdown file"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/markdown [get]
func handleAPIExportToMarkdown(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	var markdown string
	if files.IsBook(filePath) {
		// a book exports as its composed document, not its raw entry list
		composed, err := book.Compose(filePath.String())
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to compose book %s: %v", filePath, err)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "export failed"))
			return
		}
		markdown = composed
	} else {
		content, err := os.ReadFile(pathutils.ToDocsPath(filePath.String()))
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to read file %s: %v", filePath, err)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
			return
		}
		markdown = dokuwikiconverter.NewWithFilePath(filePath.String()).ConvertToMarkdown(string(content))
	}

	// prepare download
	filename := filepath.Base(filePath.String())
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".md"

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	setAttachmentFilename(w, filename)
	w.Write([]byte(markdown))

	logging.LogInfo(logging.KeyApp, "exported file to markdown: %s", filePath)
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
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing metadata or value parameter"))
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
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to browse files"))
		return
	}

	logging.LogDebug(logging.KeyApp, "browsed %d files for %s=%s", len(browsedFiles), metadata, value)

	html := render.RenderBrowseFilesHTML(browsedFiles, r.URL.Query().Get("actions") == "true")
	writeResponse(w, r, browsedFiles, html)
}

// @Summary Get metadata form HTML for file editing
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Produce json,html
// @Router /api/files/metadata/form [get]
func handleAPIGetMetadataFormHTML(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}

	html, err := render.RenderMetadataForm(filePath.String(), "")
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to generate metadata form: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to generate metadata form"))
		return
	}

	writeResponse(w, r, map[string]string{"filepath": filePath.String()}, html)
}

// @Summary Get file form HTML
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Produce json,html
// @Router /api/files/form [get]
func handleAPIFileForm(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	html := render.RenderFileForm(filePath.String())
	writeResponse(w, r, map[string]string{"filepath": filePath.String()}, html)
}

// @Summary Get metadata form HTML
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Param filetype query string false "Default file type (optional for new files)"
// @Produce json,html
// @Router /api/files/metadata-form [get]
func handleAPIMetadataForm(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	defaultFiletype := r.URL.Query().Get("editor")

	html, err := render.RenderMetadataForm(filePath.String(), defaultFiletype)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to generate metadata form: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to generate metadata form"))
		return
	}

	writeResponse(w, r, map[string]string{"filepath": filePath.String(), "editor": defaultFiletype}, html)
}

// @Summary Rename a file
// @Description Renames a file and updates all links pointing to it. Sends HX-Redirect to the new file only when the HX-Current-URL header shows the renamed file, otherwise the result is an HX-Trigger notify toast
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "Current file path"
// @Param name formData string true "New file name"
// @Produce json,html
// @Success 200 {string} string "success message"
// @Router /api/files/rename/{filepath} [post]
func handleAPIRenameFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		return
	}

	// get current file path from URL
	currentRel := strings.TrimPrefix(r.URL.Path, "/api/files/rename/")
	if currentRel == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}
	currentPath := pathutils.DocsPath(currentRel)

	// get new name from form (can be full path or just filename)
	newName := r.FormValue("name")
	if newName == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "new file path is required"))
		return
	}

	// use the new name as the new path (allows for directory moves)
	newPath := pathutils.DocsPath(path.Clean(newName))

	logging.LogInfo(logging.KeyApp, "renaming file: %s -> %s", currentPath, newPath)

	err := files.MoveFileNoRefresh(logging.KeyApp, currentPath, newPath)
	msgs := moveErrorMessages{
		sourceMissing: translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"),
		targetExists:  translation.SprintfForRequest(configmanager.GetLanguage(), "file with new name already exists"),
		moveFailed:    translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rename file"),
	}
	if handleMoveError(err, "rename file", currentPath.String(), newPath.String(), msgs, func(status int, message string) {
		writeAPIError(w, r, status, message)
	}) {
		return
	}
	files.RefreshCaches()

	if err := git.InvalidateFileHistoryCache(currentPath.String()); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to invalidate file history cache for %s: %v", currentPath, err)
	}

	logging.LogInfo(logging.KeyApp, "successfully renamed file: %s -> %s", currentPath, newPath)

	message := translation.SprintfForRequest(configmanager.GetLanguage(), "file renamed")
	if files.FolderFromPath(currentPath.String()) != files.FolderFromPath(newPath.String()) {
		message = translation.SprintfForRequest(configmanager.GetLanguage(), "file moved")
	}
	// only navigate away when the current page shows the moved file, otherwise toast in place
	if viewedFile(r) == currentPath.String() {
		w.Header().Set("HX-Redirect", pathutils.ToFileURL(newPath))
		notify.SetFlash(notify.LevelSuccess, message)
	} else {
		notify.SetHeader(w, notify.LevelSuccess, message)
	}
	writeResponse(w, r, map[string]string{"filepath": newPath.String()}, "")
}

// @Summary Move a folder into another folder
// @Description Moves a folder to a new parent, updating all internal links. Sends HX-Redirect when the HX-Current-URL header shows a file inside the moved folder, otherwise the result is an HX-Trigger notify toast
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param folderpath path string true "Current folder path (relative, no docs/ prefix)"
// @Param target formData string true "Target parent folder path"
// @Produce json,html
// @Success 200 {object} map[string]string
// @Router /api/files/move-folder/{folderpath} [post]
func handleAPIMoveFolderFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		return
	}

	currentRel := strings.TrimPrefix(r.URL.Path, "/api/files/move-folder/")
	if currentRel == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing folder path"))
		return
	}
	currentPath := pathutils.DocsPath(currentRel).String()

	targetParent := r.FormValue("target")
	if targetParent == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "target folder is required"))
		return
	}

	folderName := r.FormValue("name")
	if folderName == "" {
		folderName = filepath.Base(currentPath)
	} else if strings.Contains(folderName, "/") || strings.Contains(folderName, "\\") {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "folder name must not contain path separators"))
		return
	}
	newRel := path.Clean(targetParent + "/" + folderName)
	newPath := pathutils.DocsPath(newRel).String()

	if newRel == currentRel {
		writeResponse(w, r, map[string]string{"folderpath": newPath}, "")
		return
	}

	// prevent moving a folder into itself or a descendant
	if strings.HasPrefix(newRel+"/", currentRel+"/") {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "cannot move folder into itself"))
		return
	}

	currentFullPath := pathutils.ToDocsPath(currentPath)
	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "folder does not exist"))
		return
	}

	newFullPath := pathutils.ToDocsPath(newPath)
	if _, err := os.Stat(newFullPath); err == nil {
		writeAPIError(w, r, http.StatusConflict, translation.SprintfForRequest(configmanager.GetLanguage(), "folder with new name already exists"))
		return
	}

	result, err := job.RunMoveFolder(currentPath, newPath)
	if writeNewPathError(w, r, err) {
		return
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to move folder %s -> %s: %v", currentPath, newPath, err)
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move folder"))
		return
	}

	logging.LogInfo(logging.KeyApp, "successfully moved folder: %s -> %s (%d files updated)", currentPath, newPath, result.Updated)
	message := translation.SprintfForRequest(configmanager.GetLanguage(), "folder moved")
	// follow a file of the moved folder that the current page shows, otherwise toast in place
	if rel, ok := strings.CutPrefix(viewedFile(r), currentPath+"/"); ok {
		w.Header().Set("HX-Redirect", pathutils.ToFileURL(pathutils.GuessMeta(newPath+"/"+rel)))
		notify.SetFlash(notify.LevelSuccess, message)
	} else {
		notify.SetHeader(w, notify.LevelSuccess, message)
	}
	writeResponse(w, r, map[string]string{"folderpath": newPath}, "")
}

// removeFileAndMetadata removes a single file from disk, deletes its metadata, refreshes the
// aggregate caches, and commits the deletion to git. For a single deleted file (the common
// case) - folder delete and bulk delete run as tracked jobs instead (see internal/job).
func removeFileAndMetadata(fullPath string) error {
	if err := files.DeleteFileNoRefresh(fullPath); err != nil {
		return err
	}
	relPath := pathutils.ToRelative(fullPath)
	metaPath := pathutils.GuessMeta(fullPath)
	// a filter index file carries a paired config in configStorage - drop it too, or
	// RegenerateAllIndexes recreates the file on the next metadata change
	if filter.GetFilterConfigForFile(relPath) != nil {
		id := strings.TrimSuffix(relPath, configmanager.ExtensionForEditor("index"))
		if err := filter.DeleteFilterConfig(id); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to delete filter config for %s: %v", relPath, err)
		}
	}
	if err := files.MetaDataDeleteNoRefresh(logging.KeyApp, metaPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to delete metadata for %s: %v", relPath, err)
	}
	if err := git.InvalidateFileHistoryCache(metaPath.String()); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to invalidate file history cache for %s: %v", relPath, err)
	}
	files.RefreshCaches()
	go git.CommitDeletedFile(fullPath)
	return nil
}

// @Summary Delete a file
// @Description Deletes a file and its metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "File path to delete"
// @Produce json,html
// @Success 200 {string} string "success message"
// @Router /api/files/delete/{filepath} [delete]
func handleAPIDeleteFile(w http.ResponseWriter, r *http.Request) {
	// get file path from URL
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/delete/")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}
	filePath = pathutils.DocsPath(filePath).String()

	logging.LogInfo(logging.KeyApp, "deleting file: %s", filePath)

	// check if file exists
	fullPath := pathutils.ToDocsPath(filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"))
		return
	}

	// delete the file, its metadata, and commit the deletion to git
	if err := removeFileAndMetadata(fullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete file %s: %v", filePath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete file"))
		return
	}

	logging.LogInfo(logging.KeyApp, "successfully deleted file: %s", filePath)

	// browse-list rows delete themselves in place (hx-target="closest li") and pass
	// inline=true for an immediate toast; the file panel's delete-form has no such
	// target and needs the HX-Redirect since the page it's on no longer exists.
	if r.URL.Query().Get("inline") == "true" {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file deleted"))
		writeResponse(w, r, map[string]string{"status": "deleted"}, "")
		return
	}

	w.Header().Set("HX-Redirect", "/browse")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file deleted"))
	writeResponse(w, r, map[string]string{"status": "deleted"}, "")
}

// @Summary Delete a folder
// @Description Recursively deletes a folder, all files inside it, and their metadata
// @Tags files
// @Param folderpath path string true "Folder path to delete (relative, no docs/ prefix)"
// @Produce json,html
// @Success 200 {string} string "success message"
// @Router /api/files/delete-folder/{folderpath} [delete]
func handleAPIDeleteFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := strings.TrimPrefix(r.URL.Path, "/api/files/delete-folder/")
	if folderPath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing folder path"))
		return
	}
	folderPath = pathutils.DocsPath(folderPath).String()

	fullPath := pathutils.ToDocsPath(folderPath)
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) || !info.IsDir() {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "folder does not exist"))
		return
	}

	logging.LogInfo(logging.KeyApp, "deleting folder: %s", folderPath)

	id, err := job.StartDeleteFolder(folderPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to start folder delete %s: %v", folderPath, err)
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete folder"))
		return
	}

	respondJobStarted(w, r, id, job.JobTypeDeleteFolder)
}

// @Summary Delete all files in a collection or folder
// @Description Deletes all files belonging to a specific collection or folder, including their metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param type query string true "Type to delete by: collection or folder"
// @Param value query string true "Collection or folder name"
// @Produce json,html
// @Success 200 {string} string "deleted N files"
// @Failure 400 {string} string "missing parameters"
// @Failure 500 {string} string "delete failed"
// @Router /api/files/bulk [delete]
func handleAPIDeleteFilesBulk(w http.ResponseWriter, r *http.Request) {
	groupType := r.URL.Query().Get("type")
	value := r.URL.Query().Get("value")

	if groupType == "" || value == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing type or value parameter"))
		return
	}

	if groupType != "collection" && groupType != "folder" && groupType != "tag" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "type must be collection, folder or tag"))
		return
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get all files: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"))
		return
	}

	var toDelete []string
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

		toDelete = append(toDelete, pathutils.ToDocsPath(file.Path.String()))
	}

	id, err := job.StartBulkDeleteFiles(toDelete, groupType, value)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		return
	}

	logging.LogInfo(logging.KeyApp, "started bulk delete of %d files from %s=%s (job %s)", len(toDelete), groupType, value, id)

	respondJobStarted(w, r, id, job.JobTypeBulkDeleteFiles)
}

// @Summary Get headers (TOC) for a file
// @Description Returns headings from a file, optionally filtered, for use in wiki link anchor autocomplete
// @Tags files
// @Param typed query string true "the link destination typed so far, as written (path#heading text)"
// @Param link query string true "wiki or markdown - the kind of link typed, each heading is also returned as ready-to-insert link text"
// @Param current query string false "docs-relative path of the file being edited, whose headings a destination without path (#heading) lists"
// @Produce json,html
// @Success 200 {array} object "array of {id, text, level, link}"
// @Failure 400 {string} string "missing filepath"
// @Failure 404 {string} string "file not found"
// @Router /api/files/headers [get]
func handleAPIFilesHeaders(w http.ResponseWriter, r *http.Request) {
	linkKind, withLink := linkKindParam(r)
	if !withLink {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid link kind"))
		return
	}
	filePath, q, _ := parser.TypedLinkPath(r.URL.Query().Get("typed"), linkKind)
	q = strings.ToLower(strings.TrimSpace(q))
	bare := filePath == ""
	if bare {
		filePath = strings.TrimSpace(r.URL.Query().Get("current"))
	}
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, "missing filepath")
		return
	}

	fullPath := pathutils.ToDocsPath(pathutils.DocsPath(filePath).String())
	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, err.Error())
		return
	}

	type headerResult struct {
		ID    string `json:"id"`
		Text  string `json:"text"`
		Level int    `json:"level"`
		Link  string `json:"link,omitempty"`
	}
	results := make([]headerResult, 0)
	items := make([]render.AutocompleteItem, 0)

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil || handler.Name() != "markdown" {
		writeResponse(w, r, results, "")
		return
	}

	for _, item := range parser.TOCFromMarkdown(strings.Split(string(content), "\n")) {
		if q != "" && !strings.Contains(strings.ToLower(item.Text), q) && !strings.Contains(strings.ToLower(item.ID), q) {
			continue
		}
		value := filePath + "#" + item.ID
		if bare {
			value = "#" + item.ID
		}
		autocompleteItem := render.AutocompleteItem{
			Value:  value,
			Label:  strings.Repeat("#", item.Level) + " " + item.Text,
			Detail: value,
		}
		if withLink {
			linkPath := filePath
			if bare {
				linkPath = ""
			}
			autocompleteItem.Link = parser.FileLinkDest(linkPath, "#"+item.ID, linkKind)
		}
		results = append(results, headerResult{ID: item.ID, Text: item.Text, Level: item.Level, Link: autocompleteItem.Link})
		items = append(items, autocompleteItem)
	}

	writeResponse(w, r, results, render.RenderAutocompleteList(items))
}

// @Summary Autocomplete file paths
// @Description Returns files matching a query string for use in wiki link autocomplete
// @Tags files
// @Param q query string false "search query, with link set the link destination typed so far, as written"
// @Param link query string false "wiki or markdown - also return each file as ready-to-insert link text"
// @Produce json,html
// @Success 200 {array} object "array of {value, label, detail, link}"
// @Router /api/files/autocomplete [get]
func handleAPIFilesAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	linkKind, withLink := linkKindParam(r)
	if withLink {
		q, _, _ = parser.TypedLinkPath(q, linkKind)
	}

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	paths := make([]string, len(allFiles))
	for i, f := range allFiles {
		paths[i] = pathutils.ToRelative(f.Path.String())
	}

	matches := files.RankAutocompleteMatches(paths, q, 20)
	results := make([]render.AutocompleteItem, len(matches))
	for i, rel := range matches {
		results[i] = render.AutocompleteItem{Value: rel, Label: filepath.Base(rel), Detail: rel}
		if withLink {
			results[i].Link = parser.FileLinkDest(rel, "", linkKind)
		}
	}

	writeResponse(w, r, results, render.RenderAutocompleteList(results))
}
