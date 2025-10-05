// Package server ..
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"knov/internal/files"
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

	html := fmt.Sprintf(`<span class="collection">%s</span>`, metadata.Collection)
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

	html := fmt.Sprintf(`<span class="filetype">%s</span>`, metadata.FileType)
	writeResponse(w, r, string(metadata.FileType), html)
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
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := fmt.Sprintf(`<span class="status">%s</span>`, metadata.Status)
	writeResponse(w, r, string(metadata.Status), html)
}

// @Summary Get file priority
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/priority [get]
func handleAPIGetMetadataPriority(w http.ResponseWriter, r *http.Request) {
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

	html := fmt.Sprintf(`<span class="priority">%s</span>`, metadata.Priority)
	writeResponse(w, r, string(metadata.Priority), html)
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

// @Summary Get file folders
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/folders [get]
func handleAPIGetMetadataFolders(w http.ResponseWriter, r *http.Request) {
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
	html.WriteString(`<span class="folders">`)
	for i, folder := range metadata.Folders {
		if i > 0 {
			html.WriteString(", ")
		}
		html.WriteString(folder)
	}
	html.WriteString(`</span>`)

	writeResponse(w, r, metadata.Folders, html.String())
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- POST INDIVIDUAL ---------------------------------
// ----------------------------------------------------------------------------------------

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
// @Param filetype formData string true "File type (note, todo, journal)"
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

// @Summary Get all tags with counts
// @Tags metadata
// @Produce json,html
// @Success 200 {object} map[string]int
// @Router /api/metadata/tags [get]
func handleAPIGetAllTags(w http.ResponseWriter, r *http.Request) {
	tags, err := files.GetAllTags()
	if err != nil {
		http.Error(w, "failed to get tags", http.StatusInternalServerError)
		return
	}

	html := files.BuildBrowseHTML(tags, "/browse/tags")
	writeResponse(w, r, tags, html)
}

// @Summary Get all collections with counts
// @Tags metadata
// @Produce json,html
// @Success 200 {object} map[string]int
// @Router /api/metadata/collections [get]
func handleAPIGetAllCollections(w http.ResponseWriter, r *http.Request) {
	collections, err := files.GetAllCollections()
	if err != nil {
		http.Error(w, "failed to get collections", http.StatusInternalServerError)
		return
	}

	html := files.BuildBrowseHTML(collections, "/browse/collection")
	writeResponse(w, r, collections, html)
}

// @Summary Get all folders with counts
// @Tags metadata
// @Produce json,html
// @Success 200 {object} map[string]int
// @Router /api/metadata/folders [get]
func handleAPIGetAllFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := files.GetAllFolders()
	if err != nil {
		http.Error(w, "failed to get folders", http.StatusInternalServerError)
		return
	}

	html := files.BuildBrowseHTML(folders, "/browse/folders")
	writeResponse(w, r, folders, html)
}

// buildBrowseHTML creates HTML list for metadata browsing with counts
func buildBrowseHTML(items map[string]int, urlPrefix string) string {
	var html strings.Builder
	html.WriteString(`<ul class="search-results-simple-list">`)

	for item, count := range items {
		html.WriteString(fmt.Sprintf(`
			<li><a href="%s/%s">%s (%d)</a></li>`,
			urlPrefix, item, item, count))
	}

	html.WriteString(`</ul>`)
	return html.String()
}
