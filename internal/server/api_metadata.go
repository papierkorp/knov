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
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
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
	filePath := r.URL.Query().Get("filepath")

	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	metadata, err := files.MetaDataGet(normalizedPath)
	if err != nil {
		logging.LogError("failed to get metadata for %s: %v", normalizedPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"), http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		if strings.HasPrefix(normalizedPath, "media/") {
			metadata = &files.Metadata{Path: normalizedPath}
		} else {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"), http.StatusNotFound)
			return
		}
	}

	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		if strings.HasPrefix(normalizedPath, "media/") {
			html := render.RenderMediaDetail(metadata)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}
		html := render.RenderFileMetadataSimple(metadata)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

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

	if err := files.MetaDataSave(&metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata saved"))
	writeResponse(w, r, "metadata saved", "")
}

// @Summary Initialize/Rebuild metadata for all files
// @Description Creates metadata for all files that don't have metadata yet
// @Tags metadata
// @Produce json,html
// @Success 200 {string} string "metadata initialized"
// @Failure 500 {string} string "failed to initialize metadata"
// @Router /api/metadata/rebuild [post]
func handleAPIRebuildMetadata(w http.ResponseWriter, r *http.Request) {
	files.StartMetaGetCounter()
	defer files.StopMetaGetCounter()

	if err := files.MetaDataInitializeAll(); err != nil {
		http.Error(w, "failed to initialize metadata", http.StatusInternalServerError)
		return
	}

	stalePurged, err := files.MetaDataPurgeStale()
	if err != nil {
		logging.LogError("failed to purge stale metadata: %v", err)
	}

	dupPurged, err := files.MetaDataPurgeDuplicates()
	if err != nil {
		logging.LogError("failed to purge duplicate metadata: %v", err)
	}

	if err = files.MetaDataLinksRebuild(); err != nil {
		http.Error(w, "failed to rebuild metadata links", http.StatusInternalServerError)
		return
	}

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		logging.LogWarning("failed to update orphaned media cache after rebuild: %v", err)
	}

	logging.LogInfo("purged %d stale metadata entries", stalePurged)
	logging.LogInfo("purged %d duplicate metadata entries", dupPurged)

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata rebuilt successfully"))
	writeResponse(w, r, map[string]string{"status": "metadata initialized"}, "")
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

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata links rebuilt"))
	writeResponse(w, r, map[string]string{"status": "metadata links rebuilt"}, "")
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

// @Summary Get editor type for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/editor [get]
func handleAPIGetMetadataEditor(w http.ResponseWriter, r *http.Request) {
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

	html := render.RenderMetadataLinkHTML(string(metadata.Editor), "editor")
	writeResponse(w, r, string(metadata.Editor), html)
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

// ----------------------------------------------------------------------------------------
// ---------------------------------- SET INDIVIDUAL ----------------------------------
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

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "collection updated"))
	writeResponse(w, r, "collection updated", "")
}

// @Summary Set editor type for a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param editor formData string true "Editor type"
// @Success 200 {string} string
// @Router /api/metadata/editor [post]
func handleAPISetMetadataEditor(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	editor := r.FormValue("editor")

	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata := &files.Metadata{
		Path:   pathutils.ToWithPrefix(filePath),
		Editor: files.EditorType(editor),
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor updated"))
	writeResponse(w, r, "editor updated", "")
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
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or newpath parameter"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or newpath parameter"), http.StatusBadRequest)
		return
	}

	newpath = filepath.Clean(newpath)

	if filePath == newpath {
		writeResponse(w, r, newpath, "")
		return
	}

	logging.LogInfo("changing file path via metadata: %s -> %s", filePath, newpath)

	var currentFullPath, newFullPath string
	if strings.HasPrefix(filePath, "media/") {
		currentFullPath = pathutils.ToMediaPath(pathutils.ToRelative(filePath))
		newFullPath = pathutils.ToMediaPath(pathutils.ToRelative(newpath))
	} else {
		currentFullPath = pathutils.ToDocsPath(pathutils.ToRelative(filePath))
		newFullPath = pathutils.ToDocsPath(pathutils.ToRelative(newpath))
	}

	if _, err := os.Stat(currentFullPath); os.IsNotExist(err) {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "current file does not exist"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "current file does not exist"), http.StatusNotFound)
		return
	}

	if _, err := os.Stat(newFullPath); err == nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "file with new path already exists"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "file with new path already exists"), http.StatusConflict)
		return
	}

	newDir := filepath.Dir(newFullPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", newDir, err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
		return
	}

	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		logging.LogError("failed to move file %s -> %s: %v", filePath, newpath, err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move file"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move file"), http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(filePath, "media/") {
		if err := files.UpdateLinksForMovedFile(filePath, newpath); err != nil {
			logging.LogWarning("failed to update links for moved file %s -> %s: %v", filePath, newpath, err)
		}
	}

	logging.LogInfo("successfully moved file via metadata: %s -> %s", filePath, newpath)
	newRelPath := pathutils.ToRelative(newpath)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file moved successfully"))
	w.Header().Set("HX-Redirect", pathutils.ToFileURL(newRelPath))
	w.WriteHeader(http.StatusOK)
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

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "created at updated"))
	writeResponse(w, r, "createdat updated", "")
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

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "last edited updated"))
	writeResponse(w, r, "lastedited updated", "")
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

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "folders updated"))
	writeResponse(w, r, "folders updated", "")
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

	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		var filteredTags []string
		for _, tag := range tags {
			if tag != "" {
				filteredTags = append(filteredTags, tag)
			}
		}
		tags = filteredTags
	} else {
		tags = []string{}
	}

	sanitized, err := files.SanitizeKanbanTags(tags)
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()), http.StatusBadRequest)
		return
	}

	// read existing tags to detect kanban changes for notify messages
	oldMeta, _ := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	var oldKbTag string
	if oldMeta != nil {
		oldKbTag = kanban.TagFromList(oldMeta.Tags)
	}
	newKbTag := kanban.TagFromList(sanitized)

	metadata := &files.Metadata{
		Path: pathutils.ToWithPrefix(filePath),
		Tags: sanitized,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	if msg := kanban.TagNotifyMsg(oldKbTag, newKbTag); msg != "" {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), msg))
	} else {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "tags updated"))
	}
	writeResponse(w, r, "tags updated", "")
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

	var parents []string
	if parentsStr != "" {
		parents = strings.Split(parentsStr, ",")
		for i := range parents {
			parents[i] = strings.TrimSpace(parents[i])
		}
		for _, parent := range parents {
			if parent == "" {
				continue
			}
			fullParentPath := pathutils.ToFullPath(parent)
			if _, err := os.Stat(fullParentPath); os.IsNotExist(err) {
				html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "parent file does not exist: %s", parent))
				notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "parent file does not exist: %s", parent))
				writeResponse(w, r, nil, html)
				return
			}
		}
	} else {
		parents = []string{}
	}

	metadata := &files.Metadata{
		Path:    pathutils.ToWithPrefix(filePath),
		Parents: parents,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "parents updated"))
	writeResponse(w, r, "parents updated", "")
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- GET ALL ----------------------------------
// ----------------------------------------------------------------------------------------

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
	if filePath != "" {
		handleAPIGetFileMetadataTags(w, r)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "options" {
		cachedTags, err := files.GetAllTagsFromSystemData()
		if err != nil || len(cachedTags) == 0 {
			logging.LogError("failed to get cached tags, fallback to live data: %v", err)
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
	html := render.RenderBrowseHTML(tags, "/browse/tag", r.URL.Query().Get("actions") == "true", "tag")
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
	if filePath != "" {
		handleAPIGetFileMetadataCollection(w, r)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "options" {
		cachedCollections, err := files.GetAllCollectionsFromSystemData()
		if err != nil || len(cachedCollections) == 0 {
			logging.LogError("failed to get cached collections, fallback to live data: %v", err)
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
	html := render.RenderBrowseHTML(collections, "/browse/collection", r.URL.Query().Get("actions") == "true", "collection")
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
	if filePath != "" {
		handleAPIGetFileMetadataFolders(w, r)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "options" {
		cachedFolders, err := files.GetAllFoldersFromSystemData()
		if err != nil || len(cachedFolders) == 0 {
			logging.LogError("failed to get cached folders, fallback to live data: %v", err)
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
	html := render.RenderBrowseHTML(folders, "/browse/folder", r.URL.Query().Get("actions") == "true", "folder")
	writeResponse(w, r, folders, html)
}

// @Summary Get all file titles
// @Description Returns all non-empty titles extracted from file content, as options for datalist
// @Tags metadata
// @Param format query string false "Response format (options for HTML datalist options)"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/titles [get]
func handleAPIGetAllTitles(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		cachedTitles, err := files.GetAllTitlesFromSystemData()
		if err != nil || len(cachedTitles) == 0 {
			logging.LogError("failed to get cached titles, fallback to live data: %v", err)
			cachedTitles, err = files.GetAllTitles()
			if err != nil {
				http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get titles"), http.StatusInternalServerError)
				return
			}
		}
		var html strings.Builder
		for _, title := range cachedTitles {
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, title, title)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	titles, err := files.GetAllTitlesFromSystemData()
	if err != nil || len(titles) == 0 {
		titles, err = files.GetAllTitles()
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get titles"), http.StatusInternalServerError)
			return
		}
	}
	writeResponse(w, r, titles, "")
}

// @Summary Get all available editor types
// @Tags metadata
// @Param format query string false "Response format: options for HTML select options"
// @Param context query string false "Context: chat excludes filter-editor from suggestions"
// @Produce json,html
// @Success 200 {object} files.EditorTypeCount
// @Router /api/metadata/editors [get]
func handleAPIGetAllEditors(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		context := r.URL.Query().Get("context")
		var html strings.Builder
		for _, ft := range files.AllEditorTypes() {
			if context == "chat" && ft == files.EditorTypeFilter {
				continue
			}
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, ft, ft)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	filetypes, err := files.GetAllEditors()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get editor types"), http.StatusInternalServerError)
		return
	}
	html := render.RenderBrowseHTML(filetypes, "/browse/editor", false, "")
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

// ----------------------------------------------------------------------------------------
// ---------------------------------- REFERENCES ----------------------------------
// ----------------------------------------------------------------------------------------

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

// ----------------------------------------------------------------------------------------
// ---------------------------------- HELPERS ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get inline display for a sidebar metadata field
// @Tags metadata
// @Param field query string true "Field name (tags, parents, editor, path)"
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/metadata/inline-display [get]
func handleAPIMetadataInlineDisplay(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	filePath := r.URL.Query().Get("filepath")
	if field == "" || filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing parameters"), http.StatusBadRequest)
		return
	}
	metadata, _ := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	html := render.RenderSidebarFieldDisplay(field, filePath, metadata)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// @Summary Get inline editor for a sidebar metadata field
// @Tags metadata
// @Param field query string true "Field name (tags, parents, editor, path)"
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/metadata/inline-edit [get]
func handleAPIMetadataInlineEdit(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	filePath := r.URL.Query().Get("filepath")
	if field == "" || filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing parameters"), http.StatusBadRequest)
		return
	}
	metadata, _ := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	html := render.RenderSidebarFieldEdit(field, filePath, metadata)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}
