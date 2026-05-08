// Package server ..
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

// ----------------------------------------------------------------------------------------
// ----------------------------------- BULK OPERATIONS -----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get metadata for a single file
// @Description Get metadata for a file using filepath query parameter. Supports both media/ and docs/ paths.
// @Tags metadata
// @Produce json,html
// @Param filepath query string true "File path (with or without media/docs prefix)"
// @Success 200 {object} files.Metadata
// @Failure 400 {string} string "missing filepath parameter"
// @Failure 404 {string} string "metadata not found"
// @Failure 500 {string} string "failed to get metadata"
// @Router /api/metadata [get]
func handleAPIGetMetadata(w http.ResponseWriter, r *http.Request) {
	// get filepath from query parameter only
	filePath := r.URL.Query().Get("filepath")

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	// normalize path to ensure correct prefix for metadata lookup
	normalizedPath := pathutils.ToWithPrefix(filePath)
	metadata, err := files.MetaDataGet(normalizedPath)
	if err != nil {
		logging.LogError("failed to get metadata for %s: %v", normalizedPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"), http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
		return
	}

	// determine response format
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		// for media files, use media detail rendering
		if strings.HasPrefix(normalizedPath, "media/") {
			html := render.RenderMediaDetail(metadata)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}

		// for regular files, use simple metadata rendering
		html := render.RenderFileMetadataSimple(metadata)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	// return JSON response
	writeResponse(w, r, metadata, fmt.Sprintf("metadata for %s", normalizedPath))
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

// @Summary Rebuild metadata links for a single file
// @Description Rebuilds metadata links (ancestors, kids, usedLinks, linksToHere) for one file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath path string true "File path"
// @Success 200 {string} string "metadata links rebuilt"
// @Failure 400 {string} string "missing filepath"
// @Failure 500 {string} string "failed to rebuild metadata links"
// @Router /api/metadata/rebuild/{filepath} [post]
func handleAPIRebuildFileMetadata(w http.ResponseWriter, r *http.Request) {
	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"), http.StatusBadRequest)
		return
	}

	if err := files.MetaDataLinksRebuildForFile(filePath); err != nil {
		logging.LogError("failed to rebuild metadata links for %s: %v", filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rebuild metadata links"), http.StatusInternalServerError)
		return
	}

	data := map[string]string{"status": "metadata links rebuilt"}
	html := `<span class="status-ok">` + translation.SprintfForRequest(configmanager.GetLanguage(), "metadata links rebuilt") + `</span>`
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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

// @Summary Get file creation date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/createdat [get]
func handleAPIGetMetadataCreatedAt(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.FormValue("filepath")
	collection := r.FormValue("collection")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:       pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	filetype := r.FormValue("filetype")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:     pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	status := r.FormValue("status")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:   pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	priority := r.FormValue("priority")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:     pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	newpath := r.FormValue("newpath")

	if filePath == "" || newpath == "" {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or newpath parameter"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(html))
		return
	}

	// clean the new path
	newpath = filepath.Clean(newpath)

	// if paths are the same, no change needed
	if filePath == newpath {
		html := render.RenderStatusMessage(render.StatusOK, newpath)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	logging.LogInfo("changing file path via metadata: %s -> %s", filePath, newpath)

	// determine correct path functions based on file type
	var currentFullPath, newFullPath string
	if strings.HasPrefix(filePath, "media/") {
		currentNormalized := pathutils.ToRelative(filePath)
		currentFullPath = pathutils.ToMediaPath(currentNormalized)
		newNormalized := pathutils.ToRelative(newpath)
		newFullPath = pathutils.ToMediaPath(newNormalized)
	} else {
		currentNormalized := pathutils.ToRelative(filePath)
		currentFullPath = pathutils.ToDocsPath(currentNormalized)
		newNormalized := pathutils.ToRelative(newpath)
		newFullPath = pathutils.ToDocsPath(newNormalized)
	}

	// check if current file exists
	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "current file does not exist"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(html))
		return
	}

	// check if new path already exists
	if _, err := os.Stat(newFullPath); err == nil {
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "file with new path already exists"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(html))
		return
	}

	// create directory for new path if needed
	newDir := filepath.Dir(newFullPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", newDir, err)
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(html))
		return
	}

	// move the physical file
	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		logging.LogError("failed to move file %s -> %s: %v", filePath, newpath, err)
		html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move file"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(html))
		return
	}

	// update links in other files that reference this file (only for docs)
	if !strings.HasPrefix(filePath, "media/") {
		if err := files.UpdateLinksForMovedFile(filePath, newpath); err != nil {
			logging.LogWarning("failed to update links for moved file %s -> %s: %v", filePath, newpath, err)
			// don't fail the operation for this, just log a warning
		}
	}

	logging.LogInfo("successfully moved file via metadata: %s -> %s", filePath, newpath)

	// show success message with link to new location
	successMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "file moved successfully")
	linkText := translation.SprintfForRequest(configmanager.GetLanguage(), "view file")
	html := render.RenderStatusMessageWithLink(render.StatusOK, successMsg, "/files/"+newpath, linkText)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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
	filePath := r.FormValue("filepath")
	createdAtStr := r.FormValue("createdat")

	if filePath == "" || createdAtStr == "" {
		http.Error(w, "missing filepath or createdat parameter", http.StatusBadRequest)
		return
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:      pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	lastEditedStr := r.FormValue("lastedited")

	if filePath == "" || lastEditedStr == "" {
		http.Error(w, "missing filepath or lastedited parameter", http.StatusBadRequest)
		return
	}

	lastEdited, err := time.Parse("2006-01-02 15:04:05", lastEditedStr)
	if err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:       pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	foldersStr := r.FormValue("folders")

	if filePath == "" {
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
		Path:    pathutils.ToWithPrefix(filePath),
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
	filePath := r.URL.Query().Get("filepath")

	// if filepath is provided, return tags for that specific file
	if filePath != "" {
		handleAPIGetFileMetadataTags(w, r)
		return
	}

	// otherwise, return all tags
	format := r.URL.Query().Get("format")

	// for form options, use cached data
	if format == "options" {
		cachedTags, err := files.GetAllTagsFromSystemData()
		if err != nil {
			logging.LogError("failed to get cached tags, fallback to live data: %v", err)
			// fallback to live data
			tags, err := files.GetAllTags()
			if err != nil {
				http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get tags"), http.StatusInternalServerError)
				return
			}
			var tagList []string
			for tag := range tags {
				tagList = append(tagList, tag)
			}
			slices.Sort(tagList)
			cachedTags = tagList
		}

		var html strings.Builder
		for _, tag := range cachedTags {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, tag, tag))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	tags, err := files.GetAllTags()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get tags"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(tags, "/browse/tag")
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
	filePath := r.URL.Query().Get("filepath")

	// if filepath is provided, return collection for that specific file
	if filePath != "" {
		handleAPIGetFileMetadataCollection(w, r)
		return
	}

	// otherwise, return all collections
	format := r.URL.Query().Get("format")

	// for form options, use cached data
	if format == "options" {
		cachedCollections, err := files.GetAllCollectionsFromSystemData()
		if err != nil {
			logging.LogError("failed to get cached collections, fallback to live data: %v", err)
			// fallback to live data
			collections, err := files.GetAllCollections()
			if err != nil {
				http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get collections"), http.StatusInternalServerError)
				return
			}
			var collectionList []string
			for collection := range collections {
				collectionList = append(collectionList, collection)
			}
			slices.Sort(collectionList)
			cachedCollections = collectionList
		}

		var html strings.Builder
		for _, collection := range cachedCollections {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, collection, collection))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
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
	filePath := r.URL.Query().Get("filepath")

	// if filepath is provided, return folders for that specific file
	if filePath != "" {
		handleAPIGetFileMetadataFolders(w, r)
		return
	}

	// otherwise, return all folders
	format := r.URL.Query().Get("format")

	// for form options, use cached data
	if format == "options" {
		cachedFolders, err := files.GetAllFoldersFromSystemData()
		if err != nil {
			logging.LogError("failed to get cached folders, fallback to live data: %v", err)
			// fallback to live data
			folders, err := files.GetAllFolders()
			if err != nil {
				http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get folders"), http.StatusInternalServerError)
				return
			}
			var folderList []string
			for folder := range folders {
				folderList = append(folderList, folder)
			}
			slices.Sort(folderList)
			cachedFolders = folderList
		}

		var html strings.Builder
		for _, folder := range cachedFolders {
			html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, folder, folder))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	folders, err := files.GetAllFolders()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get folders"), http.StatusInternalServerError)
		return
	}

	html := render.RenderBrowseHTML(folders, "/browse/folder")
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.FormValue("filepath")
	tagsStr := r.FormValue("tags")

	if filePath == "" {
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
		Path: pathutils.ToWithPrefix(filePath),
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
	filePath := r.FormValue("filepath")
	parentsStr := r.FormValue("parents")

	if filePath == "" {
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
		Path:    pathutils.ToWithPrefix(filePath),
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil || metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	html := render.RenderMetadataLinkHTML(string(metadata.Status), "status")
	writeResponse(w, r, string(metadata.Status), html)
}

// @Summary Get file target date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/targetdate [get]
func handleAPIGetMetadataTargetDate(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
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
	filePath := r.FormValue("filepath")
	targetDateStr := r.FormValue("targetdate")

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path: pathutils.ToWithPrefix(filePath),
	}

	if targetDateStr != "" {
		targetDate, err := time.Parse("2006-01-02", targetDateStr)
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid date format"), http.StatusBadRequest)
			return
		}
		metadata.TargetDate = targetDate
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save target date metadata: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"), http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<span class="targetdate">%s</span>`, targetDateStr)
	writeResponse(w, r, "target date updated", html)
}

// @Summary Get references for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [get]
func handleAPIGetMetadataReferences(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil || metadata == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
		return
	}

	html := render.RenderReferencesHTML(metadata.References)
	if r.URL.Query().Get("sidebar") == "true" {
		html = render.RenderReferencesSidebarHTML(metadata.References)
	}
	writeResponse(w, r, metadata.References, html)
}

// @Summary Add a reference to a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param url formData string true "Reference URL"
// @Param description formData string false "Why this link was added"
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [post]
func handleAPIAddMetadataReference(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	refURL := r.FormValue("url")
	description := r.FormValue("description")

	if filePath == "" || refURL == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath and url are required"), http.StatusBadRequest)
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	metadata, err := files.MetaDataGet(normalizedPath)
	if err != nil || metadata == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
		return
	}

	metadata.References = append(metadata.References, files.Reference{
		URL:         refURL,
		Description: description,
	})

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save references for %s: %v", normalizedPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"), http.StatusInternalServerError)
		return
	}

	html := render.RenderReferencesHTML(metadata.References)
	writeResponse(w, r, metadata.References, html)
}

// @Summary Delete a reference from a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param url formData string true "Reference URL to remove"
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [delete]
func handleAPIDeleteMetadataReference(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	refURL := r.FormValue("url")

	if filePath == "" || refURL == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath and url are required"), http.StatusBadRequest)
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	metadata, err := files.MetaDataGet(normalizedPath)
	if err != nil || metadata == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
		return
	}

	filtered := metadata.References[:0]
	for _, ref := range metadata.References {
		if ref.URL != refURL {
			filtered = append(filtered, ref)
		}
	}
	metadata.References = filtered

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save references for %s: %v", normalizedPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"), http.StatusInternalServerError)
		return
	}

	html := render.RenderReferencesHTML(metadata.References)
	writeResponse(w, r, metadata.References, html)
}
