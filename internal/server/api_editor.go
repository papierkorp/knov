package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/utils"
)

// editorType defines the type of editor to be used
type editorType string

const (
	editorTypeMarkdown editorType = "markdown-editor"
	editorTypeTextarea editorType = "textarea-editor"
	editorTypeFilter   editorType = "filter-editor"
	editorTypeList     editorType = "list-editor"
	editorTypeIndex    editorType = "index-editor"
)

// GetEditor determines the appropriate editor type for a file based on its metadata
func GetEditor(filepath string) (editorType, error) {
	metadata, err := files.MetaDataGet(filepath)

	// always detect syntax from file type handler
	handler := files.GetParserRegistry().GetHandler(filepath)
	var handlerName string
	if handler != nil {
		handlerName = handler.Name()
	} else {
		handlerName = "markdown" // fallback
	}

	// if metadata exists, use FileType to determine editor
	if err == nil && metadata != nil {
		switch metadata.FileType {
		case files.FileTypeTodo, files.FileTypeJournaling:
			return editorTypeList, nil
		case files.FileTypeFilter:
			return editorTypeFilter, nil
		case files.FileTypeMOC:
			return editorTypeIndex, nil
		case files.FileTypeFleeting, files.FileTypePermanent, files.FileTypeLiterature:
			if handlerName == "markdown" {
				return editorTypeMarkdown, nil
			}
			// dokuwiki and plaintext both use textarea editor
			return editorTypeTextarea, nil
		}
	}

	// for new files or fallback, use handler name to determine editor
	if handlerName == "markdown" {
		return editorTypeMarkdown, nil
	}
	// dokuwiki and plaintext both use textarea editor
	return editorTypeTextarea, nil
}

// @Summary Get appropriate editor for file
// @Description Returns the appropriate editor based on file metadata or filetype parameter
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Param filetype query string false "file type (optional for new files)"
// @Produce html
// @Router /api/editor [get]
func handleAPIGetEditorHandler(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	filetype := r.URL.Query().Get("filetype")

	var html string
	var editorType editorType
	var err error

	// if filetype parameter is provided (for new files), use that to determine editor
	if filetype != "" {
		switch files.Filetype(filetype) {
		case files.FileTypeTodo, files.FileTypeJournaling:
			editorType = editorTypeList
		case files.FileTypeFilter:
			editorType = editorTypeFilter
		case files.FileTypeMOC:
			editorType = editorTypeIndex
		case files.FileTypeFleeting, files.FileTypePermanent, files.FileTypeLiterature:
			editorType = editorTypeMarkdown
		default:
			editorType = editorTypeMarkdown
		}
	} else if filepath == "" {
		// no filepath and no filetype provided, default to markdown editor for new files
		html = render.RenderMarkdownEditorForm("")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	} else {
		// use existing logic for files with path
		editorType, err = GetEditor(filepath)
		if err != nil {
			logging.LogError("failed to determine editor type for %s: %v", filepath, err)
			editorType = editorTypeMarkdown // fallback
		}
	}

	// get file content if editing existing file
	var content string
	if filepath != "" {
		if rawContent, err := files.GetRawContent(filepath); err == nil {
			content = rawContent
		}
	}

	// render the appropriate editor
	switch editorType {
	case editorTypeMarkdown:
		html = render.RenderMarkdownEditorForm(filepath)
	case editorTypeTextarea:
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeList:
		html = render.RenderListEditor(filepath)
	case editorTypeFilter:
		var renderErr error
		html, renderErr = render.RenderFilterEditor(filepath)
		if renderErr != nil {
			logging.LogError("failed to render filter editor: %v", renderErr)
			html = render.RenderTextareaEditorComponent(filepath, content)
		}
	case editorTypeIndex:
		var renderErr error
		html, renderErr = render.RenderIndexEditor(filepath)
		if renderErr != nil {
			logging.LogError("failed to render index editor: %v", renderErr)
			html = render.RenderTextareaEditorComponent(filepath, content)
		}
	default:
		html = render.RenderMarkdownEditorForm(filepath)
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

	content, err := files.GetRawContent(filepath)
	if err != nil {
		content = "" // empty for new files
	}

	html := render.RenderTextareaEditorComponent(filepath, content)

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

	filepath := r.FormValue("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	// ensure .index or .moc extension (but not both)
	if !strings.HasSuffix(filepath, ".index") && !strings.HasSuffix(filepath, ".moc") {
		filepath = filepath + ".index"
	}

	// convert to full path
	fullPath := utils.ToFullPath(filepath)

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
	if err := files.SaveRawContent(fullPath, markdown.String()); err != nil {
		logging.LogError("failed to write index file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save index"), http.StatusInternalServerError)
		return
	}

	// create/update metadata with filetype "moc" and collection based on filename
	collectionName := filepath
	if strings.HasSuffix(collectionName, ".index") {
		collectionName = strings.TrimSuffix(collectionName, ".index")
	}
	if strings.HasSuffix(collectionName, ".moc") {
		collectionName = strings.TrimSuffix(collectionName, ".moc")
	}

	metadata := &files.Metadata{
		Path:       filepath,
		FileType:   files.FileTypeMOC,
		Collection: collectionName,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for index file %s: %v", filepath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("saved metadata for index file: %s (collection: %s)", filepath, collectionName)
	}

	logging.LogInfo("saved index file: %s", filepath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "index saved successfully"),
		filepath,
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
	html := renderIndexEntryRowHelper(999, entry)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// renderIndexEntryRowHelper is a helper to render index entry rows for API responses
func renderIndexEntryRowHelper(index int, entry render.IndexEntry) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="entry-row" data-entry-index="%d">`, index))

	// controls on the left
	html.WriteString(`<div class="entry-controls">`)
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, -1)" class="btn-move">↑</button>`, index))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, 1)" class="btn-move">↓</button>`, index))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="removeEntry(this)" class="btn-remove">×</button>`))
	html.WriteString(`</div>`)

	// content on the right
	html.WriteString(`<div class="entry-content">`)
	html.WriteString(fmt.Sprintf(`<input type="hidden" name="entries[%d][type]" value="%s"/>`, index, entry.Type))

	switch entry.Type {
	case "separator":
		html.WriteString(`<div class="entry-separator">`)
		html.WriteString(fmt.Sprintf(`<span>%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "separator")))
		html.WriteString(`</div>`)

	case "file":
		html.WriteString(`<div class="entry-file">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file")))
		inputID := fmt.Sprintf("entry-file-%d", index)
		html.WriteString(render.GenerateDatalistInput(inputID, fmt.Sprintf("entries[%d][value]", index), entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "search files"), "/api/files/list?format=datalist"))
		html.WriteString(`</div>`)

	case "title":
		html.WriteString(`<div class="entry-title">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "title")))
		html.WriteString(fmt.Sprintf(`<input type="text" name="entries[%d][value]" value="%s" class="form-input" placeholder="%s"/>`, index, entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "enter title")))
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Use HTMX event to trigger reindexing after content is swapped
	html.WriteString(`<script>
document.body.addEventListener('htmx:afterSwap', function(evt) {
	if (evt.detail.target.id === 'entries-container') {
		console.log('htmx:afterSwap triggered for entries-container');
		if (typeof window.reindexEntries === 'function') {
			window.reindexEntries();
		}
	}
});
</script>`)

	return html.String()
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
	fullPath := utils.ToFullPath(filePath)

	// create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
		return
	}

	// save content as markdown
	if err := files.SaveRawContent(fullPath, markdown); err != nil {
		logging.LogError("failed to write list file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save list"), http.StatusInternalServerError)
		return
	}

	// determine filetype from path or default to todo
	filetype := files.FileTypeTodo
	if strings.Contains(strings.ToLower(filePath), "journal") {
		filetype = files.FileTypeJournaling
	}

	// create/update metadata
	metadata := &files.Metadata{
		Path:     filePath,
		FileType: filetype,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for list file %s: %v", filePath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("saved metadata for list file: %s (filetype: %s)", filePath, filetype)
	}

	logging.LogInfo("saved list file: %s", filePath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "list saved successfully"),
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
// @Param tableData formData string true "table data as JSON"
// @Produce text/html
// @Success 200 {string} string "success message"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "server error"
// @Router /api/editor/tableeditor [post]
func handleAPITableEditorSave(w http.ResponseWriter, r *http.Request) {
	filePath := r.FormValue("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid file path"), http.StatusBadRequest)
		return
	}

	tableDataJSON := r.FormValue("tableData")
	if tableDataJSON == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid table data"), http.StatusBadRequest)
		return
	}

	// parse table data
	var tableData render.TableData
	if err := json.Unmarshal([]byte(tableDataJSON), &tableData); err != nil {
		logging.LogError("failed to parse table data: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid table data format"), http.StatusBadRequest)
		return
	}

	// get original content
	originalContent, err := files.GetRawContent(filePath)
	if err != nil {
		logging.LogError("failed to read original file %s: %v", filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read original file"), http.StatusInternalServerError)
		return
	}

	// replace table in original markdown
	updatedContent := render.ReplaceTableInMarkdown(originalContent, &tableData)

	// save updated content
	fullPath := utils.ToFullPath(filePath)
	if err := files.SaveRawContent(fullPath, updatedContent); err != nil {
		logging.LogError("failed to save file %s: %v", filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved table in file: %s", filePath)
	successMsg := fmt.Sprintf(`%s <a href="/files/%s">%s</a>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "table saved successfully"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"))
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(render.RenderStatusMessage(render.StatusOK, successMsg)))
}
