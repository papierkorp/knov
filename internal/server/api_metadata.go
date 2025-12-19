// Package server ..
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// ----------------------------------------------------------------------------------------
// ----------------------------------- BULK OPERATIONS -----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get metadata for a single file
// @Description Get metadata for a file by providing filepath as query parameter
// @Tags metadata
// @Produce json,html
// @Param filepath query string true "File path"
// @Success 200 {object} files.Metadata
// @Failure 400 {string} string "missing filepath parameter"
// @Failure 404 {string} string "metadata not found"
// @Failure 500 {string} string "failed to get metadata"
// @Router /api/metadata [get]
func handleAPIGetMetadata(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	var html strings.Builder
	html.WriteString("<div class='component-metadata'>")
	html.WriteString(fmt.Sprintf("<p>Path: %s</p>", metadata.Path))
	html.WriteString(fmt.Sprintf("<p>Collection: %s</p>", metadata.Collection))
	html.WriteString(fmt.Sprintf("<p>Type: %s</p>", metadata.FileType))
	html.WriteString(fmt.Sprintf("<p>Status: %s</p>", metadata.Status))
	html.WriteString(fmt.Sprintf("<p>Priority: %s</p>", metadata.Priority))
	if len(metadata.Tags) > 0 {
		html.WriteString(fmt.Sprintf("<p>Tags: %s</p>", strings.Join(metadata.Tags, ", ")))
	}
	html.WriteString("</div>")

	writeResponse(w, r, metadata, html.String())
}

// @Summary Set metadata for a single file
// @Description Set metadata for a file using JSON payload
// @Tags metadata
// @Accept json
// @Produce json,html
// @Param metadata body files.Metadata true "Metadata object"
// @Success 200 {string} string "metadata saved"
// @Failure 400 {string} string "invalid json or missing path"
// @Failure 500 {string} string "failed to save metadata"
// @Router /api/metadata [post]
func handleAPISetMetadata(w http.ResponseWriter, r *http.Request) {
	var metadata files.Metadata

	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if metadata.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	err := files.MetaDataSave(&metadata)
	if err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	data := "metadata saved"
	html := `<span class="status-ok">metadata saved successfully</span>`
	writeResponse(w, r, data, html)
}

// @Summary Initialize/Rebuild metadata for all files
// @Description Creates metadata for all files that don't have metadata yet
// @Tags metadata
// @Produce json,html
// @Success 200 {string} string "metadata initialized"
// @Failure 500 {string} string "failed to initialize metadata"
// @Router /api/metadata/rebuild [post]
func handleAPIRebuildMetadata(w http.ResponseWriter, r *http.Request) {
	if err := files.MetaDataInitializeAll(); err != nil {
		http.Error(w, "failed to initialize metadata", http.StatusInternalServerError)
		return
	}

	err := files.MetaDataLinksRebuild()
	if err != nil {
		http.Error(w, "failed to rebuild metadata links", http.StatusInternalServerError)
		return
	}

	data := map[string]string{"status": "metadata initialized"}
	html := `<span class="status-ok">metadata initialized and rebuilt successfully</span>`
	writeResponse(w, r, data, html)
}

// @Summary Export all metadata
// @Description Export all metadata as JSON or CSV file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce application/json,text/csv
// @Param format formData string false "Export format (json or csv)" default(json)
// @Success 200 {file} file "exported metadata file"
// @Failure 500 {string} string "failed to export metadata"
// @Router /api/metadata/export [post]
func handleAPIExportMetadata(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	format := r.FormValue("format")
	if format == "" {
		format = "json"
	}

	allMetadata, err := files.MetaDataExportAll()
	if err != nil {
		http.Error(w, "failed to export metadata", http.StatusInternalServerError)
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=metadata_export.csv")

		csvData := render.RenderMetadataCSV(allMetadata)
		w.Write([]byte(csvData))

	case "json":
		fallthrough
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=metadata_export.json")

		if err := json.NewEncoder(w).Encode(allMetadata); err != nil {
			http.Error(w, "failed to encode json", http.StatusInternalServerError)
			return
		}
	}
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- GET INDIVIDUAL ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get file collection
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/collection [get]
func handleAPIGetMetadataCollection(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(metadata.Collection, "collections")
	writeResponse(w, r, metadata.Collection, html)
}

// @Summary Get file type
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/filetype [get]
func handleAPIGetMetadataFileType(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(string(metadata.FileType), "type")
	writeResponse(w, r, string(metadata.FileType), html)
}

// @Summary Get file path
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/path [get]
func handleAPIGetMetadataPath(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := fmt.Sprintf(`<span class="path">%s</span>`, metadata.Path)
	writeResponse(w, r, metadata.Path, html)
}

// @Summary Get file name
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/name [get]
func handleAPIGetMetadataName(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := fmt.Sprintf(`<span class="name">%s</span>`, metadata.Name)
	writeResponse(w, r, metadata.Name, html)
}

// @Summary Get file creation date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/createdat [get]
func handleAPIGetMetadataCreatedAt(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	createdAt := metadata.CreatedAt.Format("2006-01-02 15:04:05")
	html := fmt.Sprintf(`<span class="createdat">%s</span>`, createdAt)
	writeResponse(w, r, createdAt, html)
}

// @Summary Get file last edited date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/lastedited [get]
func handleAPIGetMetadataLastEdited(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	lastEdited := metadata.LastEdited.Format("2006-01-02 15:04:05")
	html := fmt.Sprintf(`<span class="lastedited">%s</span>`, lastEdited)
	writeResponse(w, r, lastEdited, html)
}

// @Summary Set file collection
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param collection formData string true "Collection name"
// @Success 200 {string} string
// @Router /api/metadata/collection [post]
func handleAPISetMetadataCollection(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	collection := r.FormValue("collection")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:       filepath,
		Collection: collection,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="collection">%s</span>`, collection)
	writeResponse(w, r, "collection updated", html)
}

// @Summary Set file type
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param filetype formData string true "File type (fleeting, literature, permanent, moc, todo)"
// @Success 200 {string} string
// @Router /api/metadata/filetype [post]
func handleAPISetMetadataFileType(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	filetype := r.FormValue("filetype")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:     filepath,
		FileType: files.Filetype(filetype),
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="filetype">%s</span>`, filetype)
	writeResponse(w, r, "filetype updated", html)
}

// @Summary Set file status
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param status formData string true "Status (draft, published, archived)"
// @Success 200 {string} string
// @Router /api/metadata/status [post]
func handleAPISetMetadataStatus(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	status := r.FormValue("status")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:   filepath,
		Status: files.Status(status),
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="status">%s</span>`, status)
	writeResponse(w, r, "status updated", html)
}

// @Summary Set file priority
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param priority formData string true "Priority (low, medium, high)"
// @Success 200 {string} string
// @Router /api/metadata/priority [post]
func handleAPISetMetadataPriority(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	priority := r.FormValue("priority")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:     filepath,
		Priority: files.Priority(priority),
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="priority">%s</span>`, priority)
	writeResponse(w, r, "priority updated", html)
}

// @Summary Set file path
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "Current file path"
// @Param newpath formData string true "New file path"
// @Success 200 {string} string
// @Router /api/metadata/path [post]
func handleAPISetMetadataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	newpath := r.FormValue("newpath")

	if filepath == "" || newpath == "" {
		http.Error(w, "missing filepath or newpath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path: newpath,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="path">%s</span>`, newpath)
	writeResponse(w, r, "path updated", html)
}

// @Summary Set file name
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param name formData string true "New file name"
// @Success 200 {string} string
// @Router /api/metadata/name [post]
func handleAPISetMetadataName(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	name := r.FormValue("name")

	if filepath == "" || name == "" {
		http.Error(w, "missing filepath or name parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path: filepath,
		Name: name,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="name">%s</span>`, name)
	writeResponse(w, r, "name updated", html)
}

// @Summary Set file creation date
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param createdat formData string true "Creation date (YYYY-MM-DD HH:MM:SS)"
// @Success 200 {string} string
// @Router /api/metadata/createdat [post]
func handleAPISetMetadataCreatedAt(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	createdAtStr := r.FormValue("createdat")

	if filepath == "" || createdAtStr == "" {
		http.Error(w, "missing filepath or createdat parameter", http.StatusBadRequest)
		return
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:      filepath,
		CreatedAt: createdAt,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="createdat">%s</span>`, createdAtStr)
	writeResponse(w, r, "createdat updated", html)
}

// @Summary Set file last edited date
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param lastedited formData string true "Last edited date (YYYY-MM-DD HH:MM:SS)"
// @Success 200 {string} string
// @Router /api/metadata/lastedited [post]
func handleAPISetMetadataLastEdited(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	lastEditedStr := r.FormValue("lastedited")

	if filepath == "" || lastEditedStr == "" {
		http.Error(w, "missing filepath or lastedited parameter", http.StatusBadRequest)
		return
	}

	lastEdited, err := time.Parse("2006-01-02 15:04:05", lastEditedStr)
	if err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:       filepath,
		LastEdited: lastEdited,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="lastedited">%s</span>`, lastEditedStr)
	writeResponse(w, r, "lastedited updated", html)
}

// @Summary Set file folders
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param folders formData string true "Comma-separated folder list"
// @Success 200 {string} string
// @Router /api/metadata/folders [post]
func handleAPISetMetadataFolders(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	foldersStr := r.FormValue("folders")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	var folders []string
	if foldersStr != "" {
		folders = strings.Split(foldersStr, ",")
		for i := range folders {
			folders[i] = strings.TrimSpace(folders[i])
		}
	}

	metadata := &files.Metadata{
		Path:    filepath,
		Folders: folders,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	html.WriteString(`<span class="folders">`)
	for i, folder := range folders {
		if i > 0 {
			html.WriteString(", ")
		}
		html.WriteString(folder)
	}
	html.WriteString(`</span>`)

	writeResponse(w, r, "folders updated", html.String())
}

// @Summary Get all tags or tags for a specific file
// @Description Get all tags with counts, or tags for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns tags for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.TagCount
// @Router /api/metadata/tags [get]
func handleAPIGetAllTags(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return tags for that specific file
	if filepath != "" {
		handleAPIGetFileMetadataTags(w, r)
		return
	}

	// otherwise, return all tags
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict tags
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	tags, err := files.GetAllTags()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get tags"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(tags, "/browse/tags")
	writeResponse(w, r, tags, html)
}

// @Summary Get all collections or collection for a specific file
// @Description Get all collections with counts, or collection for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns collection for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.CollectionCount
// @Router /api/metadata/collections [get]
func handleAPIGetAllCollections(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return collection for that specific file
	if filepath != "" {
		handleAPIGetFileMetadataCollection(w, r)
		return
	}

	// otherwise, return all collections
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict collections
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	collections, err := files.GetAllCollections()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get collections"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(collections, "/browse/collection")
	writeResponse(w, r, collections, html)
}

// @Summary Get all folders or folders for a specific file
// @Description Get all folders with counts, or folders for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns folders for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.FolderCount
// @Router /api/metadata/folders [get]
func handleAPIGetAllFolders(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return folders for that specific file
	if filepath != "" {
		handleAPIGetFileMetadataFolders(w, r)
		return
	}

	// otherwise, return all folders
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict folders
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	folders, err := files.GetAllFolders()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get folders"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(folders, "/browse/folders")
	writeResponse(w, r, folders, html)
}

// @Summary Get all available priorities
// @Tags metadata
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.PriorityCount
// @Router /api/metadata/priorities [get]
func handleAPIGetAllPriorities(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		var html strings.Builder
		for _, p := range files.AllPriorities() {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, p, p))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	priorities, err := files.GetAllPriorities()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get priorities"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(priorities, "/browse/priority")
	writeResponse(w, r, priorities, html)
}

// @Summary Get all available statuses
// @Tags metadata
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.StatusCount
// @Router /api/metadata/statuses [get]
func handleAPIGetAllStatuses(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		var html strings.Builder
		for _, s := range files.AllStatuses() {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, s, s))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	statuses, err := files.GetAllStatuses()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get statuses"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(statuses, "/browse/status")
	writeResponse(w, r, statuses, html)
}

// @Summary Get all available filetypes
// @Tags metadata
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.FiletypeCount
// @Router /api/metadata/filetypes [get]
func handleAPIGetAllFiletypes(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		var html strings.Builder
		for _, ft := range files.AllFiletypes() {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, ft, ft))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	filetypes, err := files.GetAllFiletypes()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get filetypes"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(filetypes, "/browse/type")
	writeResponse(w, r, filetypes, html)
}

// @Summary Get tags for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/file/tags [get]
func handleAPIGetFileMetadataTags(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinksHTML(metadata.Tags, "tags")
	writeResponse(w, r, metadata.Tags, html)
}

// @Summary Get folders for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/file/folders [get]
func handleAPIGetFileMetadataFolders(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinksHTML(metadata.Folders, "folders")
	writeResponse(w, r, metadata.Folders, html)
}

// @Summary Get collection for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/file/collection [get]
func handleAPIGetFileMetadataCollection(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(metadata.Collection, "collection")
	writeResponse(w, r, metadata.Collection, html)
}

// @Summary Set PARA projects
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param projects formData string true "Comma-separated project list"
// @Success 200 {string} string
// @Router /api/metadata/para/projects [post]
func handleAPISetMetadataPARAProjects(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	projectsStr := r.FormValue("projects")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var projects []string
	if projectsStr != "" {
		projects = strings.Split(projectsStr, ",")
		for i := range projects {
			projects[i] = strings.TrimSpace(projects[i])
		}
	} else {
		projects = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path: filepath,
		PARA: files.PARA{
			Projects: projects,
		},
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="para-projects">%s</span>`, strings.Join(projects, ", "))
	writeResponse(w, r, translation.SprintfForRequest(configmanager.GetLanguage(), "projects updated"), html)
}

// @Summary Set PARA areas
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param areas formData string true "Comma-separated area list"
// @Success 200 {string} string
// @Router /api/metadata/para/areas [post]
func handleAPISetMetadataPARAreas(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	areasStr := r.FormValue("areas")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var areas []string
	if areasStr != "" {
		areas = strings.Split(areasStr, ",")
		for i := range areas {
			areas[i] = strings.TrimSpace(areas[i])
		}
	} else {
		areas = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path: filepath,
		PARA: files.PARA{
			Areas: areas,
		},
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="para-areas">%s</span>`, strings.Join(areas, ", "))
	writeResponse(w, r, translation.SprintfForRequest(configmanager.GetLanguage(), "areas updated"), html)
}

// @Summary Set PARA resources
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param resources formData string true "Comma-separated resource list"
// @Success 200 {string} string
// @Router /api/metadata/para/resources [post]
func handleAPISetMetadataPARAResources(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	resourcesStr := r.FormValue("resources")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var resources []string
	if resourcesStr != "" {
		resources = strings.Split(resourcesStr, ",")
		for i := range resources {
			resources[i] = strings.TrimSpace(resources[i])
		}
	} else {
		resources = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path: filepath,
		PARA: files.PARA{
			Resources: resources,
		},
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="para-resources">%s</span>`, strings.Join(resources, ", "))
	writeResponse(w, r, translation.SprintfForRequest(configmanager.GetLanguage(), "resources updated"), html)
}

// @Summary Set PARA archive
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param archive formData string true "Comma-separated archive list"
// @Success 200 {string} string
// @Router /api/metadata/para/archive [post]
func handleAPISetMetadataPARAArchive(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	archiveStr := r.FormValue("archive")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var archive []string
	if archiveStr != "" {
		archive = strings.Split(archiveStr, ",")
		for i := range archive {
			archive[i] = strings.TrimSpace(archive[i])
		}
	} else {
		archive = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path: filepath,
		PARA: files.PARA{
			Archive: archive,
		},
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="para-archive">%s</span>`, strings.Join(archive, ", "))
	writeResponse(w, r, translation.SprintfForRequest(configmanager.GetLanguage(), "archive updated"), html)
}

// @Summary Set file tags
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param tags formData string true "Comma-separated tag list"
// @Success 200 {string} string
// @Router /api/metadata/tags [post]
func handleAPISetMetadataTags(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	tagsStr := r.FormValue("tags")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		// remove empty strings that might result from trimming
		var filteredTags []string
		for _, tag := range tags {
			if tag != "" {
				filteredTags = append(filteredTags, tag)
			}
		}
		tags = filteredTags
	} else {
		tags = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path: filepath,
		Tags: tags,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="tags-updated">%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "tags updated"))
	writeResponse(w, r, "tags updated", html)
}

// @Summary Set file parents
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param parents formData string true "Comma-separated parent file paths"
// @Success 200 {string} string
// @Router /api/metadata/parents [post]
func handleAPISetMetadataParents(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	parentsStr := r.FormValue("parents")

	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	// always create a slice, even if empty
	var parents []string
	if parentsStr != "" {
		parents = strings.Split(parentsStr, ",")
		for i := range parents {
			parents[i] = strings.TrimSpace(parents[i])
		}
	} else {
		parents = []string{} // explicit empty slice, not nil
	}

	metadata := &files.Metadata{
		Path:    filepath,
		Parents: parents,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="parents-updated">%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "parents updated"))
	writeResponse(w, r, "parents updated", html)
}

// @Summary Get file priority
// @Tags metadata
// @Param filepath query string true "File path"
// @Success 200 {string} string
// @Router /api/metadata/priority [get]
func handleAPIGetMetadataPriority(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(string(metadata.Priority), "priority")
	writeResponse(w, r, string(metadata.Priority), html)
}

// @Summary Get file status
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/status [get]
func handleAPIGetMetadataStatus(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(string(metadata.Status), "status")
	writeResponse(w, r, string(metadata.Status), html)
}

// @Summary Get PARA projects for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Success 200 {string} string
// @Router /api/metadata/para/projects [get]
func handleAPIGetMetadataPARAProjects(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	if len(metadata.PARA.Projects) == 0 {
		html := `<span class="para-projects">no projects</span>`
		writeResponse(w, r, []string{}, html)
		return
	}

	var links []string
	for _, project := range metadata.PARA.Projects {
		links = append(links, fmt.Sprintf(`<a href="/browse/projects/%s" class="meta-link">%s</a>`,
			url.QueryEscape(project), project))
	}

	html := fmt.Sprintf(`<span class="para-projects">%s</span>`, strings.Join(links, ", "))
	writeResponse(w, r, metadata.PARA.Projects, html)
}

// @Summary Get PARA areas for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Success 200 {string} string
// @Router /api/metadata/para/areas [get]
func handleAPIGetMetadataPARAreas(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	if len(metadata.PARA.Areas) == 0 {
		html := `<span class="para-areas">no areas</span>`
		writeResponse(w, r, []string{}, html)
		return
	}

	var links []string
	for _, area := range metadata.PARA.Areas {
		links = append(links, fmt.Sprintf(`<a href="/browse/areas/%s" class="meta-link">%s</a>`,
			url.QueryEscape(area), area))
	}

	html := fmt.Sprintf(`<span class="para-areas">%s</span>`, strings.Join(links, ", "))
	writeResponse(w, r, metadata.PARA.Areas, html)
}

// @Summary Get PARA resources for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Success 200 {string} string
// @Router /api/metadata/para/resources [get]
func handleAPIGetMetadataPARAResources(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	if len(metadata.PARA.Resources) == 0 {
		html := `<span class="para-resources">no resources</span>`
		writeResponse(w, r, []string{}, html)
		return
	}

	var links []string
	for _, resource := range metadata.PARA.Resources {
		links = append(links, fmt.Sprintf(`<a href="/browse/resources/%s" class="meta-link">%s</a>`,
			url.QueryEscape(resource), resource))
	}

	html := fmt.Sprintf(`<span class="para-resources">%s</span>`, strings.Join(links, ", "))
	writeResponse(w, r, metadata.PARA.Resources, html)
}

// @Summary Get PARA archive for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Success 200 {string} string
// @Router /api/metadata/para/archive [get]
func handleAPIGetMetadataPARAArchive(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	if len(metadata.PARA.Archive) == 0 {
		html := `<span class="para-archive">no archive</span>`
		writeResponse(w, r, []string{}, html)
		return
	}

	var links []string
	for _, archive := range metadata.PARA.Archive {
		links = append(links, fmt.Sprintf(`<a href="/browse/archive/%s" class="meta-link">%s</a>`,
			url.QueryEscape(archive), archive))
	}

	html := fmt.Sprintf(`<span class="para-archive">%s</span>`, strings.Join(links, ", "))
	writeResponse(w, r, metadata.PARA.Archive, html)
}

// @Summary Get all PARA projects or projects for a specific file
// @Description Get all PARA projects with counts, or projects for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns projects for that specific file)"
// @Param format query string false "Response format (options for datalist)" Enums(options)
// @Produce json,html
// @Success 200 {object} files.PARAProjectCount
// @Router /api/metadata/para/projects [get]
func handleAPIGetAllPARAProjects(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return projects for that specific file
	if filepath != "" {
		handleAPIGetMetadataPARAProjects(w, r)
		return
	}

	// otherwise, return all projects
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict projects
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	projectCount, err := files.GetAllPARAProjects()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get projects"), http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	// default: return div elements for display
	if len(projectCount) == 0 {
		html.WriteString(`<div class="no-items">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no projects found") + `</div>`)
	} else {
		for project, count := range projectCount {
			html.WriteString(fmt.Sprintf(`<div class="meta-item"><a href="/browse/projects/%s" class="meta-link">%s</a> <span class="meta-count">(%d)</span></div>`,
				url.QueryEscape(project), project, count))
		}
	}

	writeResponse(w, r, projectCount, html.String())
}

// @Summary Get all PARA areas or areas for a specific file
// @Description Get all PARA areas with counts, or areas for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns areas for that specific file)"
// @Param format query string false "Response format (options for datalist)" Enums(options)
// @Produce json,html
// @Success 200 {object} files.PARAAreaCount
// @Router /api/metadata/para/areas [get]
func handleAPIGetAllPARAreas(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return areas for that specific file
	if filepath != "" {
		handleAPIGetMetadataPARAreas(w, r)
		return
	}

	// otherwise, return all areas
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict areas
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	areaCount, err := files.GetAllPARAreas()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get areas"), http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	// default: return div elements for display
	if len(areaCount) == 0 {
		html.WriteString(`<div class="no-items">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no areas found") + `</div>`)
	} else {
		for area, count := range areaCount {
			html.WriteString(fmt.Sprintf(`<div class="meta-item"><a href="/browse/areas/%s" class="meta-link">%s</a> <span class="meta-count">(%d)</span></div>`,
				url.QueryEscape(area), area, count))
		}
	}

	writeResponse(w, r, areaCount, html.String())
}

// @Summary Get all PARA resources or resources for a specific file
// @Description Get all PARA resources with counts, or resources for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns resources for that specific file)"
// @Param format query string false "Response format (options for datalist)" Enums(options)
// @Produce json,html
// @Success 200 {object} files.PARAResourceCount
// @Router /api/metadata/para/resources [get]
func handleAPIGetAllPARAResources(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return resources for that specific file
	if filepath != "" {
		handleAPIGetMetadataPARAResources(w, r)
		return
	}

	// otherwise, return all resources
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict resources
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	resourceCount, err := files.GetAllPARAResources()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get resources"), http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	// default: return div elements for display
	if len(resourceCount) == 0 {
		html.WriteString(`<div class="no-items">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no resources found") + `</div>`)
	} else {
		for resource, count := range resourceCount {
			html.WriteString(fmt.Sprintf(`<div class="meta-item"><a href="/browse/resources/%s" class="meta-link">%s</a> <span class="meta-count">(%d)</span></div>`,
				url.QueryEscape(resource), resource, count))
		}
	}

	writeResponse(w, r, resourceCount, html.String())
}

// @Summary Get all PARA archive or archive for a specific file
// @Description Get all PARA archive with counts, or archive for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns archive for that specific file)"
// @Param format query string false "Response format (options for datalist)" Enums(options)
// @Produce json,html
// @Success 200 {object} files.PARAArchiveCount
// @Router /api/metadata/para/archive [get]
func handleAPIGetAllPARAArchive(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if filepath is provided, return archive for that specific file
	if filepath != "" {
		handleAPIGetMetadataPARAArchive(w, r)
		return
	}

	// otherwise, return all archive
	format := r.URL.Query().Get("format")

	// for form options, return empty options since we can't predict archive items
	if format == "options" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("")) // empty options for now
		return
	}

	archiveCount, err := files.GetAllPARAArchive()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get archive"), http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	// default: return div elements for display
	if len(archiveCount) == 0 {
		html.WriteString(`<div class="no-items">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no archive found") + `</div>`)
	} else {
		for archive, count := range archiveCount {
			html.WriteString(fmt.Sprintf(`<div class="meta-item"><a href="/browse/archive/%s" class="meta-link">%s</a> <span class="meta-count">(%d)</span></div>`,
				url.QueryEscape(archive), archive, count))
		}
	}

	writeResponse(w, r, archiveCount, html.String())
}

// @Summary Get file target date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/targetdate [get]
func handleAPIGetMetadataTargetDate(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
		return
	}

	if metadata.TargetDate.IsZero() {
		html := `<span class="targetdate">-</span>`
		writeResponse(w, r, "-", html)
		return
	}

	targetDateStr := metadata.TargetDate.Format("2006-01-02")
	html := fmt.Sprintf(`<span class="targetdate">%s</span>`, targetDateStr)
	writeResponse(w, r, targetDateStr, html)
}

// @Summary Set file target date
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param targetdate formData string false "Target date (YYYY-MM-DD, empty to clear)"
// @Success 200 {string} string
// @Router /api/metadata/targetdate [post]
func handleAPISetMetadataTargetDate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filepath := r.FormValue("filepath")
	targetDateStr := r.FormValue("targetdate")

	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path: filepath,
	}

	if targetDateStr != "" {
		targetDate, err := time.Parse("2006-01-02", targetDateStr)
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid date format"), http.StatusBadRequest)
			return
		}
		metadata.TargetDate = targetDate
	} else {
		// explicitly set zero time to clear the target date
		metadata.TargetDate = time.Time{}
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save target date metadata: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"), http.StatusInternalServerError)
		return
	}

	var html string
	if targetDateStr != "" {
		html = fmt.Sprintf(`<span class="targetdate">%s</span>`, targetDateStr)
	} else {
		html = `<span class="targetdate">-</span>`
	}
	writeResponse(w, r, "target date updated", html)
}
