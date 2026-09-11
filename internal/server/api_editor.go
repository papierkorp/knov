package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// editorType defines the type of editor to be used — now uses files.EditorType directly

// @Summary Get appropriate editor for file
// @Description Returns the appropriate editor based on file metadata or editor query param
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Param editor query string false "editor type (optional for new files)"
// @Produce json,html
// @Router /api/editor [get]
func handleAPIGetEditorHandler(w http.ResponseWriter, r *http.Request) {
	fp := r.URL.Query().Get("filepath")
	editorParam := r.URL.Query().Get("editor")
	sectionID := r.URL.Query().Get("section")
	prefillPath := r.URL.Query().Get("prefillpath")

	var html string

	// if section is specified, use section editor with the editor type from metadata
	if sectionID != "" && fp != "" {
		html = render.RenderCodeMirrorSectionEditorForm(fp, sectionID)
		writeResponse(w, r, map[string]string{"filepath": fp, "section": sectionID}, html)
		return
	}

	// resolve editor type: from param (new files) or metadata (existing files)
	var et files.EditorType
	if editorParam != "" {
		et = files.EditorType(editorParam)
	} else if fp == "" {
		// no filepath and no editor provided — use configured default for new files
		et = defaultMarkdownEditor()
	} else if et = files.ResolveEditor(fp); et == "" {
		// existing file with no metadata and a generic extension (e.g. .md)
		et = defaultMarkdownEditor()
	}

	// render the appropriate editor
	switch et {
	case files.EditorTypeList:
		html = render.RenderListEditor(fp, false)
	case files.EditorTypeTodo:
		html = render.RenderListEditor(fp, true)
	case files.EditorTypeFilter:
		var renderErr error
		html, renderErr = render.RenderFilterEditor(fp)
		if renderErr != nil {
			logging.LogError(logging.KeyApp, "failed to render filter editor: %v", renderErr)
			html = render.RenderCodeMirrorEditorForm(fp, prefillPath, editorParam)
		}
	case files.EditorTypeTracker:
		var renderErr error
		if html, renderErr = render.RenderTrackerEditor(fp); renderErr != nil {
			logging.LogError(logging.KeyApp, "failed to render tracker editor: %v", renderErr)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load tracker"))
			return
		}
	case files.EditorTypeIndex:
		var renderErr error
		if html, renderErr = render.RenderIndexEditor(fp); renderErr != nil {
			logging.LogError(logging.KeyApp, "failed to render index editor: %v", renderErr)
			html = render.RenderCodeMirrorEditorForm(fp, prefillPath, editorParam)
		}
	case files.EditorTypeBook:
		var renderErr error
		if html, renderErr = render.RenderBookEditor(fp); renderErr != nil {
			logging.LogError(logging.KeyApp, "failed to render book editor: %v", renderErr)
			html = render.RenderCodeMirrorEditorForm(fp, prefillPath, editorParam)
		}
	default:
		html = render.RenderCodeMirrorEditorForm(fp, prefillPath, editorParam)
	}

	writeResponse(w, r, map[string]string{"filepath": fp, "editor": editorParam}, html)
}

// @Summary Save index editor
// @Description Saves an index/MOC file: an ordered list of file/title/separator entries.
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param entries[][type] formData string false "entry type (file, title, separator)"
// @Param entries[][value] formData string false "file path or title text"
// @Produce html
// @Router /api/editor/indexeditor [post]
func handleAPISaveIndexEditor(w http.ResponseWriter, r *http.Request) {
	saveEntryEditorFile(w, r, false)
}

// @Summary Save book editor
// @Description Saves a .book file: an ordered list of file/title/separator entries. A file
// @Description entry value may carry a #section suffix and a "subheaders" flag.
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param entries[][type] formData string false "entry type (file, title, separator)"
// @Param entries[][value] formData string false "file path (path or path#section) or title text"
// @Param entries[][subheaders] formData string false "include subheaders for a section entry"
// @Produce html
// @Router /api/editor/bookeditor [post]
func handleAPISaveBookEditor(w http.ResponseWriter, r *http.Request) {
	saveEntryEditorFile(w, r, true)
}

// entryEditorKind is the per-editor config for the shared index/book save path: which
// editor posted decides index vs book (not any form field), so the file is always tagged
// to match the editor it was opened in.
type entryEditorKind struct {
	editor            files.EditorType
	extKey            string // configmanager.ExtensionForEditor key
	includeSubheaders bool   // whether file entries carry a subheaders flag
	savedMsg          string
	failMsg           string
}

// entryEditorKindFor selects the index or book config once; the message args stay literal
// so the gotext extractor still sees them.
func entryEditorKindFor(bookMode bool, lang string) entryEditorKind {
	if bookMode {
		return entryEditorKind{
			editor:            files.EditorTypeBook,
			extKey:            "book",
			includeSubheaders: true,
			savedMsg:          translation.SprintfForRequest(lang, "book saved successfully"),
			failMsg:           translation.SprintfForRequest(lang, "failed to save book"),
		}
	}
	return entryEditorKind{
		editor:   files.EditorTypeIndex,
		extKey:   "index",
		savedMsg: translation.SprintfForRequest(lang, "index saved successfully"),
		failMsg:  translation.SprintfForRequest(lang, "failed to save index"),
	}
}

// saveEntryEditorFile is the shared save path for the index and book entry editors.
func saveEntryEditorFile(w http.ResponseWriter, r *http.Request, bookMode bool) {
	lang := configmanager.GetLanguage()
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "failed to parse form"))
		return
	}

	// dont rename to filepath otherwise filepath.join will not work anymore because of the import
	filezpath := r.FormValue("filepath")
	if filezpath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "missing filepath"))
		return
	}

	kind := entryEditorKindFor(bookMode, lang)

	// ensure a markdown-recognized extension, else the file falls through to the plaintext
	// handler on view and shows its raw source instead of the composed index/book
	if !parser.IsMarkdownExtension(filezpath) {
		filezpath = filezpath + configmanager.ExtensionForEditor(kind.extKey)
	}
	fullPath := pathutils.ToDocsPath(filezpath)

	// parse entries[i][type] / [value] / [subheaders]
	var entries []book.Entry
	for i := 0; ; i++ {
		entryType := r.FormValue(fmt.Sprintf("entries[%d][type]", i))
		if entryType == "" {
			break
		}
		entry := book.Entry{
			Type: entryType,
			// a multiline unknown block comes from a <textarea>, so the browser sends \r\n
			Value:             strings.ReplaceAll(r.FormValue(fmt.Sprintf("entries[%d][value]", i)), "\r\n", "\n"),
			IncludeSubheaders: kind.includeSubheaders && r.FormValue(fmt.Sprintf("entries[%d][subheaders]", i)) == "true",
		}
		if entryType == book.EntryTitle {
			lvl, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("entries[%d][level]", i)))
			entry.Level = book.ClampLevel(lvl)
		}
		entries = append(entries, entry)
	}

	// serialize back to markdown - a file entry is stored as a [[wikilink]] so link detection works
	if err := contentStorage.WriteFile(fullPath, []byte(book.ToMarkdown(entries)), 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write %s file: %v", kind.extKey, err)
		writeAPIError(w, r, http.StatusInternalServerError, kind.failMsg)
		return
	}
	go git.CommitFile(fullPath)

	normalizedPath := pathutils.ToWithPrefix(filezpath)
	if err := files.MetaDataSync(normalizedPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to save metadata for %s file %s: %v", kind.extKey, filezpath, err)
	} else if err := files.SetEditor(normalizedPath, kind.editor); err != nil {
		logging.LogError(logging.KeyApp, "failed to set editor for %s file %s: %v", kind.extKey, filezpath, err)
	} else {
		logging.LogInfo(logging.KeyApp, "saved metadata for %s file: %s", kind.extKey, filezpath)
	}

	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update links for file %s: %v", filezpath, err)
	}
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo(logging.KeyApp, "saved %s file: %s", kind.extKey, filezpath)
	notify.SetHeader(w, notify.LevelSuccess, kind.savedMsg)
	writeResponse(w, r, map[string]string{"status": "ok", "filepath": filezpath}, render.RenderStatusMessageWithLink(render.StatusOK,
		kind.savedMsg, pathutils.ToFileURL(filezpath), translation.SprintfForRequest(lang, "view file")))
}

// @Summary Add index/book entry
// @Description Adds a new empty entry row to the index or book editor
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param type formData string true "entry type (separator, file, title)"
// @Param mode formData string false "index (default) or book"
// @Produce json,html
// @Router /api/editor/entry/add-entry [post]
func handleAPIAddEntry(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	entryType := r.FormValue("type")
	if entryType == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing type"))
		return
	}

	entry := book.Entry{Type: entryType}
	// index 999 is a placeholder; the editor's reindex script fixes it after the swap
	html := render.RenderEntryRowHelper(999, entry, r.FormValue("mode") == "book")
	writeResponse(w, r, entry, html)
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
// @Description Saves a list/todo file; mode selects plain bullets ("list", default) or
// @Description GFM checkbox syntax (- [ ] / - [X] / - [-] / - [O]) for "todo"
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param content formData string true "list content as json"
// @Param mode formData string false "list or todo"
// @Produce json,html
// @Router /api/editor/listeditor [post]
func handleAPISaveListEditor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}

	content := r.FormValue("content")
	todoMode := r.FormValue("mode") == "todo"
	editorType := files.EditorTypeList
	extensionKey := "list"
	if todoMode {
		editorType = files.EditorTypeTodo
		extensionKey = "todo"
	}

	// ensure the default extension for the selected mode
	if filepath.Ext(filePath) == "" {
		filePath = filePath + configmanager.ExtensionForEditor(extensionKey)
	}

	// parse JSON content from frontend
	var listItems []render.ListItem
	if err := json.Unmarshal([]byte(content), &listItems); err != nil {
		logging.LogError(logging.KeyApp, "failed to parse list items: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse list content"))
		return
	}

	// convert to markdown format
	markdown := render.ConvertListItemsToMarkdown(listItems, 0)

	// convert to full path
	fullPath := pathutils.ToDocsPath(filePath)

	// create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "failed to create directory %s: %v", dir, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		return
	}

	// save content as markdown
	if err := contentStorage.WriteFile(fullPath, []byte(markdown), 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write list file: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save list"))
		return
	}
	go git.CommitFile(fullPath)

	normalizedPath := pathutils.ToWithPrefix(filePath)
	if err := files.MetaDataSync(normalizedPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to save metadata for list file %s: %v", filePath, err)
	} else if err := files.SetEditor(normalizedPath, editorType); err != nil {
		logging.LogError(logging.KeyApp, "failed to set editor for list file %s: %v", filePath, err)
	} else {
		logging.LogInfo(logging.KeyApp, "saved metadata for list file: %s (filetype: %s)", filePath, editorType)
	}
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update orphaned media cache: %v", err)
	}

	logging.LogInfo(logging.KeyApp, "saved list file: %s", filePath)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "list saved successfully"))
	writeResponse(w, r, map[string]string{"status": "ok", "filepath": filePath}, render.RenderStatusMessageWithLink(render.StatusOK,
		translation.SprintfForRequest(configmanager.GetLanguage(), "list saved successfully"),
		pathutils.ToFileURL(filePath),
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file")))
}

// @Summary Save table data
// @Description Saves table data back to markdown file
// @Tags editor
// @Accept multipart/form-data
// @Param filepath formData string true "file path"
// @Param headers formData string true "table headers as JSON array"
// @Param rows formData string true "table rows as JSON array"
// @Param aligns formData string false "per-column alignment as JSON array (left/center/right)"
// @Param tableIndex formData string true "table index in document"
// @Produce json,html
// @Success 200 {string} string "success message"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "server error"
// @Router /api/editor/tableeditor [post]
func handleAPITableEditorSave(w http.ResponseWriter, r *http.Request) {
	// parse multipart form data (FormData from JavaScript)
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		logging.LogError(logging.KeyApp, "failed to parse multipart form: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	logging.LogDebug(logging.KeyApp, "received filepath: '%s'", filePath)
	if filePath == "" {
		logging.LogError(logging.KeyApp, "missing filepath in form data")
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}

	headersJSON := r.FormValue("headers")
	rowsJSON := r.FormValue("rows")
	tableIndexStr := r.FormValue("tableIndex")

	logging.LogDebug(logging.KeyApp, "received headers: %d bytes, rows: %d bytes, tableIndex: %s", len(headersJSON), len(rowsJSON), tableIndexStr)

	if headersJSON == "" || rowsJSON == "" || tableIndexStr == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing data"))
		return
	}

	// parse headers
	var headers []string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		logging.LogError(logging.KeyApp, "failed to parse headers: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid data format"))
		return
	}

	// parse rows
	var rows [][]string
	if err := json.Unmarshal([]byte(rowsJSON), &rows); err != nil {
		logging.LogError(logging.KeyApp, "failed to parse rows: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid data format"))
		return
	}

	// parse aligns (optional, defaults to left for every column when absent)
	var aligns []string
	if alignsJSON := r.FormValue("aligns"); alignsJSON != "" {
		if err := json.Unmarshal([]byte(alignsJSON), &aligns); err != nil {
			logging.LogError(logging.KeyApp, "failed to parse aligns: %v", err)
			writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid data format"))
			return
		}
	}

	// parse table index
	tableIndex := 0
	if tableIndexStr != "" {
		var err error
		tableIndex, err = strconv.Atoi(tableIndexStr)
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to parse table index: %v", err)
			tableIndex = 0
		}
	}

	// debug log the parsed data
	logging.LogDebug(logging.KeyApp, "parsed data - tableIndex: %d, headers: %v, rows count: %d", tableIndex, headers, len(rows))
	for i, row := range rows {
		logging.LogDebug(logging.KeyApp, "row %d: %v", i, row)
	}

	// save table using contenthandler
	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveTable(filePath, tableIndex, headers, rows, aligns); err != nil {
		logging.LogError(logging.KeyApp, "failed to save table in file %s: %v", filePath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}
	go git.CommitFile(pathutils.ToFullPath(filePath))

	logging.LogInfo(logging.KeyApp, "saved table in file: %s", filePath)

	// update links for this file
	normalizedPath := pathutils.ToWithPrefix(filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update orphaned media cache: %v", err)
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully"))
	successMsg := fmt.Sprintf(`<div class="status-ok">%s <a href="/files/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	writeResponse(w, r, map[string]string{"status": "ok", "filepath": filePath}, successMsg)
}

// @Summary Get table editor form
// @Description Returns table editor component with Tabulator
// @Tags editor
// @Param filepath query string true "file path"
// @Param tableIndex query string false "table index (default 0)"
// @Produce json,html
// @Router /api/editor/tableeditor [get]
func handleAPITableEditorForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	tableIndex := 0
	if tableIndexStr := r.URL.Query().Get("tableIndex"); tableIndexStr != "" {
		if idx, err := strconv.Atoi(tableIndexStr); err == nil {
			tableIndex = idx
		}
	}

	html := render.RenderTableEditorForm(filePath, tableIndex)

	writeResponse(w, r, map[string]any{"filepath": filePath, "tableIndex": tableIndex}, html)
}

// @Summary Save section content
// @Description Saves section content back to markdown file
// @Tags editor
// @Accept x-www-form-urlencoded
// @Param filepath formData string true "file path"
// @Param sectionid formData string true "section id"
// @Param content formData string true "section content"
// @Produce json,html
// @Router /api/files/section/save [post]
func handleAPISaveSectionEditor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	sectionID := r.FormValue("sectionid")
	content := r.FormValue("content")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}

	if sectionID == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing section id"))
		return
	}

	// save section content using contenthandler
	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveSection(filePath, sectionID, content); err != nil {
		logging.LogError(logging.KeyApp, "failed to save section %s in file %s: %v", sectionID, filePath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}
	go git.CommitFile(pathutils.ToFullPath(filePath))

	logging.LogInfo(logging.KeyApp, "saved section %s in file: %s", sectionID, filePath)

	// update links for this file
	normalizedPath := pathutils.ToWithPrefix(filePath)
	if err := files.UpdateLinksForSingleFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update links for file %s: %v", filePath, err)
		// don't fail the request, just log the error
	}

	// update orphaned media cache
	if err := files.UpdateOrphanedMediaCacheForFile(normalizedPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to update orphaned media cache: %v", err)
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "section saved successfully"))
	successMsg := fmt.Sprintf(`<div class="status-ok">%s <a href="/files/%s#%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section saved successfully"),
		filePath,
		sectionID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))

	writeResponse(w, r, map[string]string{"status": "ok", "filepath": filePath, "section": sectionID}, successMsg)
}

// @Summary Convert single file from DokuWiki to Markdown
// @Description Convert a single DokuWiki file to Markdown format and save as new file
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Success 200 {string} string "conversion success message"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "conversion failed"
// @Router /api/files/convert-to-markdown [post]
func handleAPIConvertFileToMarkdown(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	fullPath := pathutils.ToDocsPath(filePath)

	// read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to read file %s: %v", fullPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		return
	}

	// convert to markdown
	markdown := dokuwikiconverter.NewWithFilePath(filePath).ConvertToMarkdown(string(content))

	// determine new filename
	markdownFileName := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".md"
	markdownFullPath := pathutils.ToDocsPath(markdownFileName)

	// save markdown file
	if err := os.WriteFile(markdownFullPath, []byte(markdown), 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write markdown file %s: %v", markdownFullPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save converted file"))
		return
	}
	go git.CommitFile(markdownFullPath)

	logging.LogInfo(logging.KeyApp, "converted dokuwiki file to markdown: %s -> %s", filePath, markdownFileName)

	html := render.RenderStatusMessageWithLink(render.StatusOK,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file converted to markdown successfully"),
		pathutils.ToFileURL(markdownFileName), markdownFileName)
	writeResponse(w, r, map[string]string{"status": "ok", "filepath": markdownFileName}, html)
}

// defaultMarkdownEditor returns the configured default editor for markdown files.
// KNOV_DEFAULT_EDITOR env var takes precedence over the user setting.
func defaultMarkdownEditor() files.EditorType {
	if env := configmanager.GetAppConfig().DefaultEditor; env != "" {
		return files.EditorType(env)
	}
	return files.EditorType(configmanager.DefaultMarkdownEditor.Get())
}
