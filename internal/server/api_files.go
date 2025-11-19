// Package server ..
package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
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
		html := render.RenderFilesOptions(allFiles)
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
				Action:   render.GetFormValue(actions, i),
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

	html := render.RenderFilteredFiles(filteredFiles)
	writeResponse(w, r, filteredFiles, html)
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

	// Prevent saving filter files
	if strings.HasSuffix(strings.ToLower(filepath), ".filter") {
		http.Error(w, "filter files cannot be edited", http.StatusBadRequest)
		return
	}

	// Also check by metadata
	metadata, err := files.MetaDataGet(filepath)
	if err == nil && metadata != nil && metadata.FileType == files.FileTypeFilter {
		http.Error(w, "filter files cannot be edited", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	fullPath := utils.ToFullPath(filepath)

	err = os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to save file %s: %v", fullPath, err)
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	logging.LogInfo("saved file: %s", filepath)
	successMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully")
	html := render.RenderStatusMessage(render.StatusOK, successMsg)

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
		http.Error(w, "missing metadata or value parameter", http.StatusBadRequest)
		return
	}

	logging.LogDebug("browse request: %s=%s", metadata, value)

	// Set operator based on field type - arrays use "contains", simple fields use "equals"
	operator := "equals"
	if metadata == "tags" || metadata == "folders" ||
		metadata == "projects" || metadata == "areas" ||
		metadata == "resources" || metadata == "archive" {
		operator = "contains"
	}

	criteria := []files.FilterCriteria{
		{
			Metadata: metadata,
			Operator: operator,
			Value:    value,
			Action:   "include",
		},
	}

	logging.LogDebug("browse criteria: %+v", criteria)

	browsedFiles, err := files.FilterFilesByMetadata(criteria, "and")
	if err != nil {
		logging.LogError("failed to browse files: %v", err)
		http.Error(w, "failed to browse files", http.StatusInternalServerError)
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

	html, err := render.RenderMetadataForm(filePath)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, "failed to generate metadata form", http.StatusInternalServerError)
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
// @Param type query string false "File type to pre-select"
// @Produce html
// @Router /api/files/metadata-form [get]
func handleAPIMetadataForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	fileType := r.URL.Query().Get("type")

	html, err := render.RenderMetadataForm(filePath)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, "failed to generate metadata form", http.StatusInternalServerError)
		return
	}

	// if file type is specified, inject JavaScript to pre-select it
	if fileType != "" {
		html += fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				setTimeout(function() {
					const fileTypeSelect = document.querySelector('select[name="filetype"], select[hx-trigger*="filetype"]');
					if (fileTypeSelect) {
						fileTypeSelect.value = '%s';
						fileTypeSelect.dispatchEvent(new Event('change'));
					}
				}, 100);
			});
		</script>
		`, fileType)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Create new file
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Param filepath formData string true "File path"
// @Param content formData string true "File content"
// @Param filetype formData string false "File type (todo, fleeting, literature, moc, permanent, filter)"
// @Router /api/files/create [post]
func handleAPIFileCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	content := r.FormValue("content")
	fileType := r.FormValue("filetype")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filePath)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to create file %s: %v", fullPath, err)
		http.Error(w, "failed to create file", http.StatusInternalServerError)
		return
	}

	// create metadata with file type if specified
	if fileType != "" {
		metadata := &files.Metadata{
			Path:     filePath,
			FileType: files.Filetype(fileType),
		}
		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to save file type metadata: %v", err)
		} else {
			logging.LogDebug("saved file type metadata: %s -> %s", filePath, fileType)
		}
	}

	logging.LogInfo("created file: %s", filePath)
	w.Header().Set("HX-Redirect", "/files/"+filePath)
	w.WriteHeader(http.StatusOK)
}

// @Summary Create new filter file directly
// @Description Creates a new filter file with default name and redirects to it
// @Tags files
// @Produce html
// @Success 302 {string} string "redirect to new filter file"
// @Failure 500 {string} string "failed to create filter file"
// @Router /api/files/create-filter [get]
func handleAPICreateFilter(w http.ResponseWriter, r *http.Request) {
	// generate unique filter filename
	timestamp := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("filter-%s.filter", timestamp)
	filePath := fileName
	fullPath := utils.ToFullPath(filePath)

	// create minimal filter file content
	content := fmt.Sprintf("# Filter - %s\n\nThis is a filter file. The content below will be replaced with an interactive filter form.\n", timestamp)

	// create the file
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		logging.LogError("failed to create filter file %s: %v", fullPath, err)
		http.Error(w, "failed to create filter file", http.StatusInternalServerError)
		return
	}

	// create metadata with filter file type
	metadata := &files.Metadata{
		Path:     filePath,
		FileType: files.FileTypeFilter,
	}
	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogWarning("failed to save filter metadata: %v", err)
	}

	logging.LogInfo("created filter file: %s", filePath)
	http.Redirect(w, r, "/files/"+filePath, http.StatusSeeOther)
}

// @Summary Get markdown editor form HTML
// @Description Returns a markdown editor form for creating or editing files
// @Tags files
// @Param filepath query string false "File path (optional for new files)"
// @Param type query string false "File type for new files"
// @Produce html
// @Router /api/files/markdown-form [get]
func handleAPIMarkdownEditorForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	fileType := r.URL.Query().Get("type")

	// Don't show markdown editor for filter files
	if strings.HasSuffix(strings.ToLower(filePath), ".filter") {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="info-message">Filter files display an interactive filter form and cannot be edited as text.</div>`))
		return
	}

	// Also check by file type
	if filePath != "" {
		metadata, err := files.MetaDataGet(filePath)
		if err == nil && metadata != nil && metadata.FileType == files.FileTypeFilter {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="info-message">Filter files display an interactive filter form and cannot be edited as text.</div>`))
			return
		}
	}

	html := render.RenderMarkdownEditorForm(filePath)

	// if file type is specified, inject it into the form
	if fileType != "" && filePath == "" {
		// add hidden field for file type in new file creation
		hiddenField := fmt.Sprintf(`<input type="hidden" name="filetype" value="%s">`, fileType)
		// insert the hidden field after the first input tag
		if strings.Contains(html, `<input type="hidden" name="filepath"`) {
			html = strings.Replace(html, `<input type="hidden" name="filepath"`, hiddenField+`
			<input type="hidden" name="filepath"`, 1)
		}
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
