package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// editorType defines the type of editor to be used — now uses files.EditorType directly

// @Summary Get appropriate editor for file
// @Description Returns the appropriate editor based on file metadata or editor query param
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Param editor query string false "editor type (optional for new files)"
// @Produce html
// @Router /api/editor [get]
func handleAPIGetEditorHandler(w http.ResponseWriter, r *http.Request) {
	fp := r.URL.Query().Get("filepath")
	editorParam := r.URL.Query().Get("editor")
	sectionID := r.URL.Query().Get("section")

	var html string

	// if section is specified, use section editor regardless of editor type
	if sectionID != "" && fp != "" {
		html = render.RenderMarkdownSectionEditorForm(fp, sectionID)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	// resolve editor type: from param (new files) or metadata (existing files)
	var et files.EditorType
	if editorParam != "" {
		et = files.EditorType(editorParam)
	} else if fp == "" {
		// no filepath and no editor provided — default to markdown for new files
		html = render.RenderMarkdownEditorForm("", "")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	} else {
		// existing file: read editor from metadata, fall back to handler detection
		metadata, _ := files.MetaDataGet(fp)
		if metadata != nil && metadata.Editor != "" {
			et = metadata.Editor
		} else {
			handler := parser.GetParserRegistry().GetHandler(fp)
			if handler != nil && handler.Name() != "markdown" {
				et = files.EditorTypeTextarea
			} else {
				et = files.EditorTypeMarkdown
			}
		}
	}

	// get file content if editing existing file
	var content string
	if fp != "" {
		fullPath := pathutils.ToDocsPath(fp)
		if rawContent, err := contentStorage.ReadFile(fullPath); err == nil {
			content = string(rawContent)
		}
	}

	// render the appropriate editor
	switch et {
	case files.EditorTypeMarkdown:
		html = render.RenderMarkdownEditorForm(fp, editorParam)
	case files.EditorTypeTextarea:
		html = render.RenderTextareaEditorComponent(fp, content)
	case files.EditorTypeList:
		html = render.RenderListEditor(fp)
	case files.EditorTypeTodo:
		html = render.RenderTodoEditor(fp)
	case files.EditorTypeFilter:
		var renderErr error
		html, renderErr = render.RenderFilterEditor(fp)
		if renderErr != nil {
			logging.LogError("failed to render filter editor: %v", renderErr)
			html = render.RenderTextareaEditorComponent(fp, content)
		}
	case files.EditorTypeIndex:
		var renderErr error
		html, renderErr = render.RenderIndexEditor(fp)
		if renderErr != nil {
			logging.LogError("failed to render index editor: %v", renderErr)
			html = render.RenderTextareaEditorComponent(fp, content)
		}
	default:
		html = render.RenderMarkdownEditorForm(fp, "")
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get markdown editor form HTML
// @Description Returns a markdown editor form for creating or editing files
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Produce html
// @Router /api/editor/markdown-form [get]
func handleAPIMarkdownEditorForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")

	html := render.RenderMarkdownEditorForm(filePath)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get textarea editor component
// @Description Returns a simple textarea editor component for editing file content
// @Tags editor
// @Param filepath query string true "file path"
// @Produce html
// @Router /api/editor/textarea [get]
func handleAPIGetTextareaEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fullPath := pathutils.ToDocsPath(filepath)
	content, err := contentStorage.ReadFile(fullPath)
	var contentStr string
	if err != nil {
		contentStr = "" // empty for new files
	} else {
		contentStr = string(content)
	}

	html := render.RenderTextareaEditorComponent(filepath, contentStr)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Save index editor
// @Description Saves an index/MOC file
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param entries[][type] formData string false "entry type"
// @Param entries[][value] formData string false "entry value"
// @Produce html
// @Router /api/editor/indexeditor [post]
func handleAPISaveIndexEditor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	// dont rename to filepath otherwise filepath.join will not work anymore because of the import
	filezpath := r.FormValue("filepath")
	if filezpath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	// ensure .index or .moc extension (but not both)
	if !strings.HasSuffix(filezpath, ".index") && !strings.HasSuffix(filezpath, ".moc") {
		filezpath = filezpath + ".index"
	}

	// convert to full path
	fullPath := pathutils.ToDocsPath(filezpath)

	// parse entries
	var config render.IndexConfig
	config.Entries = []render.IndexEntry{}

	// parse entries[i][type] and entries[i][value]
	i := 0
	for {
		typeKey := fmt.Sprintf("entries[%d][type]", i)
		valueKey := fmt.Sprintf("entries[%d][value]", i)

		entryType := r.FormValue(typeKey)
		if entryType == "" {
			break // no more entries
		}

		entryValue := r.FormValue(valueKey)
		config.Entries = append(config.Entries, render.IndexEntry{
			Type:  entryType,
			Value: entryValue,
		})
		i++
	}

	// convert to markdown format with links (so existing link detection works)
	var markdown strings.Builder
	for _, entry := range config.Entries {
		switch entry.Type {
		case "separator":
			markdown.WriteString("\n---\n\n")
		case "file":
			if entry.Value != "" {
				// create markdown link: [filename](filename)
				markdown.WriteString(fmt.Sprintf("- [%s](%s)\n", entry.Value, entry.Value))
			}
		case "title":
			if entry.Value != "" {
				markdown.WriteString(fmt.Sprintf("\n## %s\n\n", entry.Value))
			}
		}
	}

	// save as markdown file
	if err := contentStorage.WriteFile(fullPath, []byte(markdown.String()), 0644); err != nil {
		logging.LogError("failed to write index file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save index"), http.StatusInternalServerError)
		return
	}

	// create/update metadata with filetype "moc" and collection based on filename
	collectionName := filezpath
	collectionName = strings.TrimSuffix(collectionName, ".index")
	collectionName = strings.TrimSuffix(collectionName, ".moc")

	metadata := &files.Metadata{
		Path:       filepath.Join("docs", filezpath),
		Editor:     files.EditorTypeIndex,
		Collection: collectionName,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for index file %s: %v", filezpath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("saved metadata for index file: %s (collection: %s)", filezpath, collectionName)
	}

	// update links for this file
	normalizedPath := filepath.Join("docs", filezpath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update links for file %s: %v", filezpath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo("saved index file: %s", filezpath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "index saved successfully"),
		filezpath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(render.RenderStatusMessage(render.StatusOK, successMsg)))
}

// @Summary Add index entry
// @Description Adds a new entry to the index editor
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param type formData string true "entry type (separator, file, title)"
// @Produce html
// @Router /api/editor/indexeditor/add-entry [post]
func handleAPIAddIndexEntry(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	entryType := r.FormValue("type")
	if entryType == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing type"), http.StatusBadRequest)
		return
	}

	// get current count of entries
	container := r.FormValue("container")
	_ = container // not used, we'll use JavaScript to count

	// create new entry with next index (JavaScript will handle proper indexing)
	entry := render.IndexEntry{
		Type:  entryType,
		Value: "",
	}

	// render entry row with index 999 (will be reindexed by JavaScript)
	html := render.RenderIndexEntryRowHelper(999, entry)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Save filter editor
// @Description Saves a filter file (redirects to existing filter save endpoint)
// @Tags editor
// @Accept x-www-form-urlencoded
// @Produce html
// @Router /api/editor/filtereditor [post]
func handleAPISaveFilterEditor(w http.ResponseWriter, r *http.Request) {
	// this is just a redirect to the existing filter save endpoint
	handleAPIFilterSave(w, r)
}

// @Summary Save list editor
// @Description Saves a list file for todo and journaling file types
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param content formData string true "list content as json"
// @Produce html
// @Router /api/editor/listeditor [post]
func handleAPISaveListEditor(w http.ResponseWriter, r *http.Request) {
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

	// ensure .list extension
	if !strings.HasSuffix(filePath, ".list") {
		filePath = filePath + ".list"
	}

	// parse JSON content from frontend
	var listItems []render.ListItem
	if err := json.Unmarshal([]byte(content), &listItems); err != nil {
		logging.LogError("failed to parse list items: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse list content"), http.StatusBadRequest)
		return
	}

	// convert to markdown format
	markdown := render.ConvertListItemsToMarkdown(listItems, 0)

	// convert to full path
	fullPath := pathutils.ToDocsPath(filePath)

	// create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
		return
	}

	// save content as markdown
	if err := contentStorage.WriteFile(fullPath, []byte(markdown), 0644); err != nil {
		logging.LogError("failed to write list file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save list"), http.StatusInternalServerError)
		return
	}

	// determine filetype from path or default to todo
	filetype := files.EditorTypeTodo
	if strings.Contains(strings.ToLower(filePath), "journal") {
		filetype = files.EditorTypeList
	}

	// create/update metadata
	metadata := &files.Metadata{
		Path:   filepath.Join("docs", filePath),
		Editor: filetype,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for list file %s: %v", filePath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("saved metadata for list file: %s (filetype: %s)", filePath, filetype)
	}

	// update links for this file
	normalizedPath := filepath.Join("docs", filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo("saved list file: %s", filePath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "list saved successfully"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(render.RenderStatusMessage(render.StatusOK, successMsg)))
}

// @Summary Save todo editor
// @Description Saves a todo file using GFM checkbox syntax (- [ ] / - [X] / - [-] / - [O])
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param content formData string true "todo content as json"
// @Produce html
// @Router /api/editor/todoeditor [post]
func handleAPISaveTodoEditor(w http.ResponseWriter, r *http.Request) {
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

	// ensure .todo extension
	if !strings.HasSuffix(filePath, ".todo") {
		filePath = filePath + ".todo"
	}

	// parse JSON content from frontend
	var listItems []render.ListItem
	if err := json.Unmarshal([]byte(content), &listItems); err != nil {
		logging.LogError("failed to parse todo items: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse todo content"), http.StatusBadRequest)
		return
	}

	// convert to GFM checkbox markdown
	markdown := render.ConvertTodoItemsToMarkdown(listItems, 0)

	fullPath := pathutils.ToDocsPath(filePath)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
		return
	}

	if err := contentStorage.WriteFile(fullPath, []byte(markdown), 0644); err != nil {
		logging.LogError("failed to write todo file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save todo"), http.StatusInternalServerError)
		return
	}

	metadata := &files.Metadata{
		Path:   filepath.Join("docs", filePath),
		Editor: files.EditorTypeTodo,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for todo file %s: %v", filePath, err)
	} else {
		logging.LogInfo("saved metadata for todo file: %s", filePath)
	}

	normalizedPath := filepath.Join("docs", filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update links for file %s: %v", filePath, err)
	}
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo("saved todo file: %s", filePath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "todo saved successfully"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(render.RenderStatusMessage(render.StatusOK, successMsg)))
}

// @Summary Save table data
// @Description Saves table data back to markdown file
// @Tags editor
// @Accept multipart/form-data
// @Param filepath formData string true "file path"
// @Param headers formData string true "table headers as JSON array"
// @Param rows formData string true "table rows as JSON array"
// @Param tableIndex formData string true "table index in document"
// @Produce text/html
// @Success 200 {string} string "success message"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "server error"
// @Router /api/editor/tableeditor [post]
func handleAPITableEditorSave(w http.ResponseWriter, r *http.Request) {
	// parse multipart form data (FormData from JavaScript)
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		logging.LogError("failed to parse multipart form: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	logging.LogDebug("received filepath: '%s'", filePath)
	if filePath == "" {
		logging.LogError("missing filepath in form data")
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"), http.StatusBadRequest)
		return
	}

	headersJSON := r.FormValue("headers")
	rowsJSON := r.FormValue("rows")
	tableIndexStr := r.FormValue("tableIndex")

	logging.LogDebug("received headers: %d bytes, rows: %d bytes, tableIndex: %s", len(headersJSON), len(rowsJSON), tableIndexStr)

	if headersJSON == "" || rowsJSON == "" || tableIndexStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing data"), http.StatusBadRequest)
		return
	}

	// parse headers
	var headers []string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		logging.LogError("failed to parse headers: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid data format"), http.StatusBadRequest)
		return
	}

	// parse rows
	var rows [][]string
	if err := json.Unmarshal([]byte(rowsJSON), &rows); err != nil {
		logging.LogError("failed to parse rows: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid data format"), http.StatusBadRequest)
		return
	}

	// parse table index
	tableIndex := 0
	if tableIndexStr != "" {
		var err error
		tableIndex, err = strconv.Atoi(tableIndexStr)
		if err != nil {
			logging.LogError("failed to parse table index: %v", err)
			tableIndex = 0
		}
	}

	// debug log the parsed data
	logging.LogDebug("parsed data - tableIndex: %d, headers: %v, rows count: %d", tableIndex, headers, len(rows))
	for i, row := range rows {
		logging.LogDebug("row %d: %v", i, row)
	}

	// save table using contenthandler
	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveTable(filePath, tableIndex, headers, rows); err != nil {
		logging.LogError("failed to save table in file %s: %v", filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved table in file: %s", filePath)

	// update links for this file
	normalizedPath := pathutils.ToWithPrefix(filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update orphaned media cache: %v", err)
	}

	successMsg := fmt.Sprintf(`<div class="status-success">%s <a href="/files/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(successMsg))
}

// @Summary Get table editor form
// @Description Returns table editor component with Handsontable
// @Tags editor
// @Param filepath query string true "file path"
// @Param tableIndex query string false "table index (default 0)"
// @Produce html
// @Router /api/editor/tableeditor [get]
func handleAPITableEditorForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	tableIndex := 0
	if tableIndexStr := r.URL.Query().Get("tableIndex"); tableIndexStr != "" {
		if idx, err := strconv.Atoi(tableIndexStr); err == nil {
			tableIndex = idx
		}
	}

	html := render.RenderTableEditorForm(filePath, tableIndex)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Save section content
// @Description Saves section content back to markdown file
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param sectionid formData string true "section id"
// @Param content formData string true "section content"
// @Produce html
// @Router /api/files/section/save [post]
func handleAPISaveSectionEditor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	sectionID := r.FormValue("sectionid")
	content := r.FormValue("content")

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"), http.StatusBadRequest)
		return
	}

	if sectionID == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing section id"), http.StatusBadRequest)
		return
	}

	// save section content using contenthandler
	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveSection(filePath, sectionID, content); err != nil {
		logging.LogError("failed to save section %s in file %s: %v", sectionID, filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved section %s in file: %s", sectionID, filePath)

	// update links for this file
	normalizedPath := pathutils.ToWithPrefix(filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning("failed to update orphaned media cache: %v", err)
	}

	successMsg := fmt.Sprintf(`<div class="status-success">%s <a href="/files/%s#%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section saved successfully"),
		filePath,
		sectionID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(successMsg))
}

// @Summary Convert single file from DokuWiki to Markdown
// @Description Convert a single DokuWiki file to Markdown format and save as new file
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce text/html
// @Param filepath formData string true "File path"
// @Success 200 {string} string "conversion success message"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "conversion failed"
// @Router /api/files/convert-to-markdown [post]
func handleAPIConvertFileToMarkdown(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "markdown conversion only supported for dokuwiki files"), http.StatusBadRequest)
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

	// determine new filename
	markdownFileName := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".md"
	markdownFullPath := pathutils.ToDocsPath(markdownFileName)

	// save markdown file
	if err := os.WriteFile(markdownFullPath, []byte(markdown), 0644); err != nil {
		logging.LogError("failed to write markdown file %s: %v", markdownFullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save converted file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("converted dokuwiki file to markdown: %s -> %s", filePath, markdownFileName)

	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file converted to markdown successfully"),
		markdownFileName,
		markdownFileName)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="status-success">%s</div>`, successMsg)
}
