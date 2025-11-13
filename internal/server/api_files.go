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
	w.Write([]byte(fmt.Sprintf(`<span style="color: green;">%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file saved successfully"))))
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
		html.WriteString(`<div class="form-group">`)
		html.WriteString(fmt.Sprintf(`<label for="filepath">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file path")))
		html.WriteString(fmt.Sprintf(`<input type="text" id="filepath" name="filepath" required placeholder="%s" class="form-input"/>`, translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/filename.md")))
		html.WriteString(`</div>`)
	}

	// Metadata section
	metadataForm, err := generateMetadataForm(filePath)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
	} else {
		html.WriteString(`<div class="form-section">`)
		html.WriteString(fmt.Sprintf(`<h3>%s</h3>`, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata")))
		html.WriteString(metadataForm)
		html.WriteString(`</div>`)
	}

	// Content section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(fmt.Sprintf(`<h3>%s</h3>`, translation.SprintfForRequest(configmanager.GetLanguage(), "content")))
	html.WriteString(fmt.Sprintf(`<textarea id="initial-content" style="display:none;">%s</textarea>`, content))
	html.WriteString(`<div id="markdown-editor"></div>`)
	html.WriteString(`</div>`)

	// Form actions
	html.WriteString(`<div class="form-actions">`)
	submitText := fmt.Sprintf("%s", translation.SprintfForRequest(configmanager.GetLanguage(), "create file"))
	if isEdit {
		submitText = fmt.Sprintf("%s", translation.SprintfForRequest(configmanager.GetLanguage(), "save"))
	}
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary"><span>%s</span></button>`, submitText))

	// Cancel link
	cancelLink := "/"
	if isEdit {
		cancelLink = fmt.Sprintf("/files/%s", filePath)
	}
	html.WriteString(fmt.Sprintf(`<a href="%s" class="btn-secondary">%s</a>`, cancelLink, translation.SprintfForRequest(configmanager.GetLanguage(), "cancel")))
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
	html := fmt.Sprintf(`<div class="success-message">%s <a href="/files/%s">%s</a> | <a href="/files/edit/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file created successfully!"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view file"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "edit file"))

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get metadata form
// @Description Get standalone metadata form for a file
// @Tags files
// @Param filepath query string false "File path for edit mode"
// @Produce text/html
// @Success 200 {string} string "metadata form html"
// @Router /api/files/metadata-form [get]
func handleAPIMetadataForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")

	html, err := generateMetadataForm(filePath)
	if err != nil {
		logging.LogError("failed to generate metadata form: %v", err)
		http.Error(w, "failed to generate metadata form", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// generateMetadataForm creates HTML form for editing manual metadata fields
func generateMetadataForm(filePath string) (string, error) {
	var metadata *files.Metadata
	var err error

	// get existing metadata if filepath is provided
	if filePath != "" {
		metadata, err = files.MetaDataGet(filePath)
		if err != nil {
			logging.LogError("failed to get metadata for %s: %v", filePath, err)
			// create empty metadata if file doesn't exist
			metadata = &files.Metadata{}
		}
	} else {
		metadata = &files.Metadata{}
	}

	// set defaults for new files
	if metadata.FileType == "" {
		metadata.FileType = files.FileTypeFleeting
	}
	if metadata.Status == "" {
		metadata.Status = files.StatusDraft
	}
	if metadata.Priority == "" {
		metadata.Priority = files.PriorityMedium
	}

	var html strings.Builder
	html.WriteString(`<div id="component-metadata-form" class="metadata-form">`)

	// Add status indicator for saves
	html.WriteString(`<div id="metadata-save-status" class="save-status"></div>`)

	// name field
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-name">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "name")))
	if filePath != "" {
		html.WriteString(fmt.Sprintf(`<input type="text" id="meta-name" name="name" value="%s" class="form-input" placeholder="%s" 
			hx-post="/api/metadata/name" hx-vals='{"filepath": "%s"}' hx-trigger="change delay:500ms" hx-target="#metadata-save-status" hx-swap="innerHTML"/>`,
			metadata.Name, translation.SprintfForRequest(configmanager.GetLanguage(), "filename"), filePath))
	} else {
		html.WriteString(fmt.Sprintf(`<input type="text" id="meta-name" name="name" value="%s" class="form-input" placeholder="%s"/>`,
			metadata.Name, translation.SprintfForRequest(configmanager.GetLanguage(), "filename")))
	}
	html.WriteString(`</div>`)

	// collection field with datalist and auto-save
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-collection">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "collection")))
	if filePath != "" {
		html.WriteString(generateDatalistInputWithSave("meta-collection", "collection", metadata.Collection,
			translation.SprintfForRequest(configmanager.GetLanguage(), "type or select collection"),
			"/api/metadata/options/collections", filePath, "/api/metadata/collection"))
	} else {
		html.WriteString(generateDatalistInput("meta-collection", "collection", metadata.Collection,
			translation.SprintfForRequest(configmanager.GetLanguage(), "type or select collection"),
			"/api/metadata/options/collections"))
	}
	html.WriteString(`</div>`)

	// tags field with tag chips and auto-save
	tagsStr := strings.Join(metadata.Tags, ", ")
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-tags">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "tags")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-tags", "tags", tagsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
			"/api/metadata/options/tags", filePath, "/api/metadata/tags"))
	} else {
		html.WriteString(generateTagChipsInput("meta-tags", "tags", tagsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
			"/api/metadata/options/tags"))
	}
	html.WriteString(`</div>`)

	// parents field with tag chips and auto-save
	parentsStr := strings.Join(metadata.Parents, ", ")
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-parents">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "parents")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-parents", "parents", parentsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add parent file paths"),
			"", filePath, "/api/metadata/parents"))
	} else {
		html.WriteString(generateTagChipsInput("meta-parents", "parents", parentsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add parent file paths"),
			""))
	}
	html.WriteString(`</div>`)

	// filetype field with auto-save
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-filetype">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file type")))
	html.WriteString(`<select id="meta-filetype" name="filetype" class="form-select"`)
	if filePath != "" {
		html.WriteString(fmt.Sprintf(` hx-post="/api/metadata/filetype" hx-vals='{"filepath": "%s"}' hx-trigger="change" hx-target="#metadata-save-status" hx-swap="innerHTML"`, filePath))
	}
	html.WriteString(`>`)

	filetypes := []files.Filetype{files.FileTypeFleeting, files.FileTypeLiterature, files.FileTypePermanent, files.FileTypeMOC, files.FileTypeTodo}
	for _, ft := range filetypes {
		selected := ""
		if metadata.FileType == ft {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, string(ft), selected, string(ft)))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// status field with auto-save
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-status">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "status")))
	html.WriteString(`<select id="meta-status" name="status" class="form-select"`)
	if filePath != "" {
		html.WriteString(fmt.Sprintf(` hx-post="/api/metadata/status" hx-vals='{"filepath": "%s"}' hx-trigger="change" hx-target="#metadata-save-status" hx-swap="innerHTML"`, filePath))
	}
	html.WriteString(`>`)

	statuses := []files.Status{files.StatusDraft, files.StatusPublished, files.StatusArchived}
	for _, s := range statuses {
		selected := ""
		if metadata.Status == s {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, string(s), selected, string(s)))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// priority field with auto-save
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-priority">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "priority")))
	html.WriteString(`<select id="meta-priority" name="priority" class="form-select"`)
	if filePath != "" {
		html.WriteString(fmt.Sprintf(` hx-post="/api/metadata/priority" hx-vals='{"filepath": "%s"}' hx-trigger="change" hx-target="#metadata-save-status" hx-swap="innerHTML"`, filePath))
	}
	html.WriteString(`>`)

	priorities := []files.Priority{files.PriorityLow, files.PriorityMedium, files.PriorityHigh}
	for _, p := range priorities {
		selected := ""
		if metadata.Priority == p {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, string(p), selected, string(p)))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// para section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<h4>%s</h4>`, translation.SprintfForRequest(configmanager.GetLanguage(), "PARA")))

	// projects
	projectsStr := strings.Join(metadata.PARA.Projects, ", ")
	html.WriteString(`<div class="form-subgroup">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-projects">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "projects")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-projects", "projects", projectsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add projects"),
			"/api/metadata/para/projects/all?format=options", filePath, "/api/metadata/para/projects"))
	} else {
		html.WriteString(generateTagChipsInput("meta-projects", "projects", projectsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add projects"),
			"/api/metadata/para/projects/all?format=options"))
	}
	html.WriteString(`</div>`)

	// areas
	areasStr := strings.Join(metadata.PARA.Areas, ", ")
	html.WriteString(`<div class="form-subgroup">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-areas">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "areas")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-areas", "areas", areasStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add areas"),
			"/api/metadata/para/areas/all?format=options", filePath, "/api/metadata/para/areas"))
	} else {
		html.WriteString(generateTagChipsInput("meta-areas", "areas", areasStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add areas"),
			"/api/metadata/para/areas/all?format=options"))
	}
	html.WriteString(`</div>`)

	// resources
	resourcesStr := strings.Join(metadata.PARA.Resources, ", ")
	html.WriteString(`<div class="form-subgroup">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-resources">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "resources")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-resources", "resources", resourcesStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add resources"),
			"/api/metadata/para/resources/all?format=options", filePath, "/api/metadata/para/resources"))
	} else {
		html.WriteString(generateTagChipsInput("meta-resources", "resources", resourcesStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add resources"),
			"/api/metadata/para/resources/all?format=options"))
	}
	html.WriteString(`</div>`)

	// archive
	archiveStr := strings.Join(metadata.PARA.Archive, ", ")
	html.WriteString(`<div class="form-subgroup">`)
	html.WriteString(fmt.Sprintf(`<label for="meta-archive">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "archive")))
	if filePath != "" {
		html.WriteString(generateTagChipsInputWithSave("meta-archive", "archive", archiveStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add archived items"),
			"/api/metadata/para/archive/all?format=options", filePath, "/api/metadata/para/archive"))
	} else {
		html.WriteString(generateTagChipsInput("meta-archive", "archive", archiveStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add archived items"),
			"/api/metadata/para/archive/all?format=options"))
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`) // close para form group

	html.WriteString(`</div>`) // close metadata form

	return html.String(), nil
}

// generateDatalistInput creates an input field with autocomplete datalist
func generateDatalistInput(id, name, value, placeholder, apiEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" autocomplete="off" list="%s" placeholder="%s"/>
<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
	<option value="">loading options...</option>
</datalist>`, id, name, value, datalistId, placeholder, datalistId, apiEndpoint)
}

// generateDatalistInputWithSave creates an input field with autocomplete and auto-save
func generateDatalistInputWithSave(id, name, value, placeholder, apiEndpoint, filePath, saveEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" autocomplete="off" list="%s" placeholder="%s"
	hx-post="%s" hx-vals='{"filepath": "%s"}' hx-trigger="change delay:500ms" hx-target="#metadata-save-status" hx-swap="innerHTML"/>
<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
	<option value="">loading options...</option>
</datalist>`, id, name, value, datalistId, placeholder, saveEndpoint, filePath, datalistId, apiEndpoint)
}

// generateTagChipsInput creates a tag chips input with autocomplete
func generateTagChipsInput(id, name, value, placeholder, apiEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	chipsId := fmt.Sprintf("%s-chips", id)
	inputId := fmt.Sprintf("%s-input", id)
	hiddenId := fmt.Sprintf("%s-hidden", id)

	var datalistHTML string
	if apiEndpoint != "" {
		datalistHTML = fmt.Sprintf(`<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
		<option value="">loading options...</option>
	</datalist>`, datalistId, apiEndpoint)
	} else {
		datalistHTML = fmt.Sprintf(`<datalist id="%s"></datalist>`, datalistId)
	}

	return fmt.Sprintf(`<div class="tag-chips-container" id="%s">
	<div class="tag-chips" id="%s-display"></div>
	<input type="text" id="%s" class="tag-chips-input" autocomplete="off" list="%s" placeholder="%s"/>
	<input type="hidden" id="%s" name="%s" value="%s"/>
	%s
</div>
<script>
(function() {
	const container = document.getElementById('%s');
	const display = document.getElementById('%s-display');
	const input = document.getElementById('%s');
	const hidden = document.getElementById('%s');
	
	let tags = [];
	
	// initialize with existing values
	if (hidden.value) {
		tags = hidden.value.split(',').map(t => t.trim()).filter(t => t);
		renderTags();
	}
	
	function renderTags() {
		display.innerHTML = '';
		tags.forEach((tag, index) => {
			const chip = document.createElement('span');
			chip.className = 'tag-chip';
			chip.innerHTML = tag + '<button type="button" class="tag-chip-remove">×</button>';
			
			const removeBtn = chip.querySelector('.tag-chip-remove');
			removeBtn.addEventListener('click', function() {
				removeTag(index);
			});
			
			display.appendChild(chip);
		});
		hidden.value = tags.join(', ');
	}
	
	function addTag(value) {
		const trimmed = value.trim();
		if (trimmed && !tags.includes(trimmed)) {
			tags.push(trimmed);
			renderTags();
			input.value = '';
		}
	}
	
	function removeTag(index) {
		tags.splice(index, 1);
		renderTags();
	}
	
	// handle input events
	input.addEventListener('keydown', function(e) {
		if (e.key === 'Enter' || e.key === ',' || e.key === 'Tab') {
			e.preventDefault();
			addTag(input.value);
		} else if (e.key === 'Backspace' && input.value === '' && tags.length > 0) {
			tags.pop();
			renderTags();
		}
	});
	
	// handle datalist selection
	input.addEventListener('change', function() {
		if (input.value) {
			addTag(input.value);
		}
	});
	
	// handle blur to catch paste events
	input.addEventListener('blur', function() {
		if (input.value) {
			addTag(input.value);
		}
	});
	
	// make container clickable to focus input
	container.addEventListener('click', function() {
		input.focus();
	});
})();
</script>`, chipsId, chipsId, inputId, datalistId, placeholder, hiddenId, name, value, datalistHTML, chipsId, chipsId, inputId, hiddenId)
}

// generateTagChipsInputWithSave creates a tag chips input with autocomplete and auto-save
func generateTagChipsInputWithSave(id, name, value, placeholder, apiEndpoint, filePath, saveEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	chipsId := fmt.Sprintf("%s-chips", id)
	inputId := fmt.Sprintf("%s-input", id)
	hiddenId := fmt.Sprintf("%s-hidden", id)

	var datalistHTML string
	if apiEndpoint != "" {
		datalistHTML = fmt.Sprintf(`<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
		<option value="">loading options...</option>
	</datalist>`, datalistId, apiEndpoint)
	} else {
		datalistHTML = fmt.Sprintf(`<datalist id="%s"></datalist>`, datalistId)
	}

	return fmt.Sprintf(`<div class="tag-chips-container" id="%s">
	<div class="tag-chips" id="%s-display"></div>
	<input type="text" id="%s" class="tag-chips-input" autocomplete="off" list="%s" placeholder="%s"/>
	<input type="hidden" id="%s" name="%s" value="%s"/>
	%s
</div>
<script>
(function() {
	const container = document.getElementById('%s');
	const display = document.getElementById('%s-display');
	const input = document.getElementById('%s');
	const hidden = document.getElementById('%s');
	
	let tags = [];
	let saveTimeout;
	
	// initialize with existing values
	if (hidden.value) {
		tags = hidden.value.split(',').map(t => t.trim()).filter(t => t);
		renderTags();
	}
	
	function renderTags() {
		display.innerHTML = '';
		tags.forEach((tag, index) => {
			const chip = document.createElement('span');
			chip.className = 'tag-chip';
			chip.innerHTML = tag + '<button type="button" class="tag-chip-remove">×</button>';
			
			const removeBtn = chip.querySelector('.tag-chip-remove');
			removeBtn.addEventListener('click', function() {
				removeTag(index);
			});
			
			display.appendChild(chip);
		});
		hidden.value = tags.join(', ');
		saveData();
	}
	
	function addTag(value) {
		const trimmed = value.trim();
		if (trimmed && !tags.includes(trimmed)) {
			tags.push(trimmed);
			renderTags();
			input.value = '';
		}
	}
	
	function removeTag(index) {
		tags.splice(index, 1);
		renderTags();
	}
	
	function saveData() {
		if ('%s' === '') return; // no filepath for new files
		
		clearTimeout(saveTimeout);
		saveTimeout = setTimeout(function() {
			htmx.ajax('POST', '%s', {
				values: {
					'filepath': '%s',
					'%s': hidden.value
				},
				target: '#metadata-save-status',
				swap: 'innerHTML'
			});
		}, 500);
	}
	
	// handle input events
	input.addEventListener('keydown', function(e) {
		if (e.key === 'Enter' || e.key === ',' || e.key === 'Tab') {
			e.preventDefault();
			addTag(input.value);
		} else if (e.key === 'Backspace' && input.value === '' && tags.length > 0) {
			tags.pop();
			renderTags();
		}
	});
	
	// handle datalist selection
	input.addEventListener('change', function() {
		if (input.value) {
			addTag(input.value);
		}
	});
	
	// handle blur to catch paste events
	input.addEventListener('blur', function() {
		if (input.value) {
			addTag(input.value);
		}
	});
	
	// make container clickable to focus input
	container.addEventListener('click', function() {
		input.focus();
	});
})();
</script>`, chipsId, chipsId, inputId, datalistId, placeholder, hiddenId, name, value, datalistHTML, chipsId, chipsId, inputId, hiddenId, filePath, saveEndpoint, filePath, name)
}
