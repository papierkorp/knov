// Package server ..
package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/utils"
)

// @Summary Get all files
// @Tags files
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, "failed to get files", http.StatusInternalServerError)
		return
	}

	format := r.URL.Query().Get("format")

	if format == "options" {
		var html strings.Builder
		html.WriteString(`<option value="">select a file...</option>`)
		for _, file := range allFiles {
			path := strings.TrimPrefix(file.Path, "data/")
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, path, path))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range allFiles {
		html.WriteString(fmt.Sprintf(`
			<li>
				<a href="#"
					hx-get="/files/%s?snippet=true"
					hx-target="#file-content"
					hx-on::after-request="htmx.ajax('GET', '/api/files/header?filepath=%s', {target: '#file-header'})"
				>%s</a>
			</li>`,
			file.Path,
			file.Path,
			file.Path))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, allFiles, html.String())
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
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(content.HTML))
}

// @Summary Filter files by metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param metadata[] formData []string false "Metadata fields to filter on"
// @Param operator[] formData []string false "Filter operators (equals, contains, greater, less, in)"
// @Param value[] formData []string false "Filter values"
// @Param action[] formData []string false "Filter actions (include, exclude)"
// @Param logic formData string false "Logic operator for combining criteria (and, or)" default(and)
// @Success 200 {array} files.File
// @Router /api/files/filter [post]
func handleAPIFilterFiles(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter request received")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	logic := r.FormValue("logic")
	if logic == "" {
		logic = "and"
	}

	var criteria []files.FilterCriteria
	metadata := r.Form["metadata[]"]
	operators := r.Form["operator[]"]
	values := r.Form["value[]"]
	actions := r.Form["action[]"]

	maxLen := len(metadata)

	for i := range maxLen {
		if i < len(operators) && i < len(values) && metadata[i] != "" && operators[i] != "" {
			criteria = append(criteria, files.FilterCriteria{
				Metadata: metadata[i],
				Operator: operators[i],
				Value:    values[i],
				Action:   getFormValue(actions, i),
			})
		}
	}

	logging.LogDebug("built %d filter criteria: %+v", len(criteria), criteria)

	filteredFiles, err := files.FilterFilesByMetadata(criteria, logic)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		http.Error(w, "failed to filter files", http.StatusInternalServerError)
		return
	}

	logging.LogDebug("filtered %d files", len(filteredFiles))

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>found %d files</p>", len(filteredFiles)))
	html.WriteString("<ul>")
	for _, file := range filteredFiles {
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`,
			strings.TrimPrefix(file.Path, "data/"),
			strings.TrimPrefix(file.Path, "data/")))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, filteredFiles, html.String())
}

func getFormValue(slice []string, index int) string {
	if index < len(slice) {
		return slice[index]
	}
	return ""
}

// @Summary Get file header with link and breadcrumb
// @Tags files
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/files/header [get]
func handleAPIGetFileHeader(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	data := map[string]string{
		"filepath": filepath,
		"link":     "/files/" + filepath,
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div id="current-file-breadcrumb"><a href="/files/%s">â†’ %s</a></div>`, filepath, filepath))

	writeResponse(w, r, data, html.String())
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
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		logging.LogError("failed to get raw content: %v", err)
		http.Error(w, "failed to get raw content", http.StatusInternalServerError)
		return
	}

	data := map[string]string{"content": content}
	writeResponse(w, r, data, content)
}

// @Summary Save file content
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "File path"
// @Param content formData string true "File content"
// @Produce html
// @Router /api/files/save/{filepath} [post]
func handleAPIFileSave(w http.ResponseWriter, r *http.Request) {
	filepath := strings.TrimPrefix(r.URL.Path, "/api/files/save/")

	if filepath == "" {
		http.Error(w, "missing filepath", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	fullPath := utils.ToFullPath(filepath)

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to save file %s: %v", fullPath, err)
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved file: %s", filepath)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<span style="color: green;">file saved successfully</span>`))
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
		http.Error(w, "missing metadata or value parameter", http.StatusBadRequest)
		return
	}

	logging.LogDebug("browse request: %s=%s", metadata, value)

	operator := "equals"
	if metadata == "tags" || metadata == "folders" {
		operator = "contains"
	}

	criteria := []files.FilterCriteria{
		{
			Metadata: metadata,
			Operator: operator, // dynamically set based on field type
			Value:    value,
			Action:   "include",
		},
	}
	filteredFiles, err := files.FilterFilesByMetadata(criteria, "and")
	if err != nil {
		logging.LogError("failed to browse files: %v", err)
		http.Error(w, "failed to browse files", http.StatusInternalServerError)
		return
	}

	logging.LogDebug("found %d files", len(filteredFiles))

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>found %d files</p>", len(filteredFiles)))
	html.WriteString("<ul>")
	for _, file := range filteredFiles {
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`,
			strings.TrimPrefix(file.Path, "data/"),
			strings.TrimPrefix(file.Path, "data/")))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, filteredFiles, html.String())
}

// @Summary Get file form
// @Description Get file form for create or edit
// @Tags files
// @Param filepath query string false "File path for edit mode"
// @Produce text/html
// @Success 200 {string} string "file form html"
// @Router /api/files/form [get]
func handleAPIFileForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	var content string
	var err error
	isEdit := filePath != ""

	if isEdit {
		fullPath := utils.ToFullPath(filePath)
		content, err = files.GetRawContent(fullPath)
		if err != nil {
			content = ""
		}
	}

	var formAction, method string
	if isEdit {
		formAction = fmt.Sprintf("/api/files/save/%s", filePath)
		method = "hx-post"
	} else {
		formAction = "/api/files/create"
		method = "hx-post"
	}

	theme := "light"
	if configmanager.GetDarkMode() {
		theme = "dark"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<form id="file-form" %s="%s" hx-target="#file-status" hx-swap="innerHTML" class="file-form">`, method, formAction))

	// File path section (only for new files)
	if !isEdit {
		html.WriteString(`<div class="form-section">`)
		html.WriteString(`<h4>file settings</h4>`)
		html.WriteString(`<div class="form-group">`)
		html.WriteString(`<label for="filepath">file path</label>`)
		html.WriteString(`<input type="text" id="filepath" name="filepath" required placeholder="path/to/filename.md" class="form-input"/>`)
		html.WriteString(`<small>enter the full path including filename and extension</small>`)
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)
	}

	// Content section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(`<h4>content</h4>`)
	html.WriteString(fmt.Sprintf(`<textarea id="initial-content" style="display:none;">%s</textarea>`, content))
	html.WriteString(`<div id="markdown-editor"></div>`)
	html.WriteString(`</div>`)

	// Form actions
	html.WriteString(`<div class="form-actions">`)
	submitText := "🚀 create file"
	if isEdit {
		submitText = "💾 save"
	}
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary"><span>%s</span></button>`, submitText))

	// Cancel link
	cancelLink := "/"
	if isEdit {
		cancelLink = fmt.Sprintf("/files/%s", filePath)
	}
	html.WriteString(fmt.Sprintf(`<a href="%s" class="btn-secondary">cancel</a>`, cancelLink))
	html.WriteString(`</div>`)
	html.WriteString(`</form>`)
	html.WriteString(`<div id="file-status"></div>`)

	// JavaScript for Toast UI Editor
	html.WriteString(fmt.Sprintf(`
		<script>
			(function() {
				const initialContent = document.getElementById('initial-content').value;
				const editor = new toastui.Editor({
					el: document.querySelector('#markdown-editor'),
					height: '600px',
					initialEditType: 'markdown',
					previewStyle: 'tab',
					initialValue: initialContent,
					usageStatistics: false,
					theme: '%s'
				});

				const form = document.getElementById('file-form');
				form.addEventListener('htmx:configRequest', function(evt) {
					evt.detail.parameters['content'] = editor.getMarkdown();
				});
			})();
		</script>
	`, theme))

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html.String()))
}

// @Summary Create new file
// @Description Create a new file with content
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "File path"
// @Param content formData string false "File content"
// @Produce html
// @Success 200 {string} string "file created"
// @Router /api/files/create [post]
func handleAPIFileCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	if filePath == "" {
		http.Error(w, "filepath is required", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	fullPath := utils.ToFullPath(filePath)

	// Create directory if it does not exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	// Check if file already exists
	if _, err := os.Stat(fullPath); err == nil {
		http.Error(w, "file already exists", http.StatusConflict)
		return
	}

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to create file %s: %v", fullPath, err)
		http.Error(w, "failed to create file", http.StatusInternalServerError)
		return
	}

	logging.LogInfo("created file: %s", filePath)
	html := fmt.Sprintf(`<div class="success-message">file created successfully! <a href="/files/%s">view file</a> | <a href="/files/edit/%s">edit file</a></div>`, filePath, filePath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
