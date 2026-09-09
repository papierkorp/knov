// Package server - Media upload API endpoints
package server

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

// @Summary Upload media file
// @Description Upload a media file with context path for directory mirroring
// @Tags media
// @Accept multipart/form-data
// @Param file formData file true "Media file to upload"
// @Param context_path formData string true "Current file being edited (for directory structure)"
// @Produce json,html
// @Success 200 {object} map[string]string "Upload success with file path"
// @Failure 400 {string} string "invalid request"
// @Failure 413 {string} string "file too large"
// @Failure 415 {string} string "unsupported file type"
// @Failure 500 {string} string "upload failed"
// @Router /api/media/upload [post]
func handleAPIMediaUpload(w http.ResponseWriter, r *http.Request) {
	// check if context path is provided (prevent uploads for unsaved files)
	contextPath := r.FormValue("context_path")
	if contextPath == "" {
		logging.LogWarning(logging.KeyApp, "media upload attempted without context path")
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "save document first to enable media uploads"))
		return
	}

	// prevent uploads to unsaved files (context_path like "new")
	if contextPath == "new" || strings.HasPrefix(contextPath, "new/") {
		logging.LogWarning(logging.KeyApp, "media upload attempted for unsaved file: %s", contextPath)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "save document first to enable media uploads"))
		return
	}

	// parse multipart form with size limit
	maxUploadSize := configmanager.GetMaxUploadSize()

	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to parse multipart form: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse upload form"))
		return
	}

	// get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get uploaded file: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no file uploaded"))
		return
	}
	defer file.Close()

	// use the files package to handle the upload
	result, err := files.UploadMedia(file, header, contextPath)
	if err != nil {
		var statusCode int
		switch err.Error() {
		case "file too large":
			statusCode = http.StatusRequestEntityTooLarge
		case "unsupported file type":
			statusCode = http.StatusUnsupportedMediaType
		default:
			statusCode = http.StatusInternalServerError
		}
		writeAPIError(w, r, statusCode, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		return
	}

	// return response
	responseData := map[string]string{
		"path":        result.Path,
		"filename":    result.Filename,
		"contentType": result.ContentType,
		"size":        result.Size,
	}

	writeResponse(w, r, responseData, fmt.Sprintf("media uploaded: %s", result.Path))
}

// @Summary Get all media files
// @Description Get list of all media files with metadata, optionally filtered
// @Tags media
// @Produce json,html
// @Param filter query string false "Filter: all, used, orphaned" default(all)
// @Param mode query string false "Mode: default, select" default(default)
// @Success 200 {object} map[string]interface{} "List of media files"
// @Failure 500 {string} string "internal error"
// @Router /api/media/list [get]
func handleAPIGetAllMedia(w http.ResponseWriter, r *http.Request) {
	// get filter parameter (all, used, orphaned)
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all" // default
	}

	// get mode parameter (default, select)
	mode := r.URL.Query().Get("mode")

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get media files: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load media files"))
		return
	}

	totalRawCount := len(mediaFiles)
	// apply hide-type settings (image, video, pdf, office, archives, etc.)
	mediaFiles = files.FilterByVisibility(mediaFiles, "")
	hiddenCount := totalRawCount - len(mediaFiles)

	// get orphaned media from cache
	orphanedMedia, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get orphaned media: %v", err)
		orphanedMedia = []string{} // fallback to empty
	}

	// filter media files based on filter parameter
	filteredMedia := files.FilterMediaFiles(mediaFiles, orphanedMedia, filter)

	// count orphaned only among the visible (hide-filtered) files
	visiblePaths := make(map[string]struct{}, len(mediaFiles))
	for _, f := range mediaFiles {
		visiblePaths[f.Path] = struct{}{}
	}
	visibleOrphanedCount := 0
	for _, o := range orphanedMedia {
		if _, ok := visiblePaths[o]; ok {
			visibleOrphanedCount++
		}
	}

	var html string
	switch mode {
	case "select":
		html = render.RenderMediaListSelect(filteredMedia)
	case "compact":
		html = render.RenderMediaListCompact(filteredMedia, "detail")
		if hiddenCount > 0 {
			w.Header().Set("X-Hidden-Message", translation.SprintfForRequest(configmanager.GetLanguage(), "%d files not shown (hidden in settings)", hiddenCount))
		}
	default:
		html = render.RenderMediaList(filteredMedia, filter, len(mediaFiles), visibleOrphanedCount, hiddenCount)
	}
	writeResponse(w, r, filteredMedia, html)
}

// @Summary Autocomplete media file paths
// @Description Returns media files matching a query string for use in image/media link autocomplete
// @Tags media
// @Param q query string false "search query"
// @Produce json,html
// @Success 200 {array} object "array of {value, label, detail}"
// @Router /api/media/autocomplete [get]
func handleAPIMediaAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	paths := make([]string, len(mediaFiles))
	for i, f := range mediaFiles {
		paths[i] = strings.TrimPrefix(f.Path, "media/")
	}

	matches := files.RankAutocompleteMatches(paths, q, 20)
	results := make([]render.AutocompleteItem, len(matches))
	for i, rel := range matches {
		results[i] = render.AutocompleteItem{Value: rel, Label: filepath.Base(rel), Detail: rel}
	}

	writeResponse(w, r, results, render.RenderAutocompleteList(results))
}

// @Summary Delete media file
// @Description Deletes a media file and its metadata
// @Tags media
// @Param mediapath path string true "Media file path to delete"
// @Produce html
// @Success 200 {string} string "success message"
// @Failure 400 {string} string "missing media path"
// @Failure 404 {string} string "media file not found"
// @Failure 409 {string} string "media file still referenced"
// @Failure 500 {string} string "internal error"
// @Router /api/media/{mediapath} [delete]
func handleAPIDeleteMedia(w http.ResponseWriter, r *http.Request) {
	mediaPath := chi.URLParam(r, "*")
	if mediaPath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing media path"))
		return
	}

	// add media prefix if not present
	fullMediaPath := mediaPath
	if !strings.HasPrefix(mediaPath, "media/") {
		fullMediaPath = "media/" + mediaPath
	}

	logging.LogInfo(logging.KeyApp, "deleting media file: %s", fullMediaPath)

	// check if file exists
	fullPath := pathutils.ToMediaPath(strings.TrimPrefix(fullMediaPath, "media/"))
	exists, err := contentStorage.FileExists(fullPath)
	if err != nil || !exists {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "media file not found"))
		return
	}

	// check if file is still referenced
	metadata, err := files.MetaDataGet(fullMediaPath)
	if err == nil && metadata != nil && len(metadata.LinksToHere) > 0 {
		logging.LogWarning(logging.KeyApp, "cannot delete media file %s: still referenced by %d files", fullMediaPath, len(metadata.LinksToHere))

		refs := metadata.LinksToHere
		if len(refs) > 5 {
			refs = append(refs[:5:5], translation.SprintfForRequest(configmanager.GetLanguage(), "and %d more", len(metadata.LinksToHere)-5))
		}
		// route the error into the list's own error slot so the grid survives the failed swap
		w.Header().Set("HX-Retarget", "#component-media-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		writeAPIError(w, r, http.StatusConflict, translation.SprintfForRequest(configmanager.GetLanguage(),
			"cannot delete media file: still referenced by %s", strings.Join(refs, ", ")))
		return
	}

	// delete file from filesystem
	if err := contentStorage.DeleteFile(fullPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete media file %s: %v", fullPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete file"))
		return
	}

	// delete metadata
	if err := files.MetaDataDelete(fullMediaPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to delete metadata for media file %s: %v", fullMediaPath, err)
		// don't fail the whole operation, just log warning
	}

	logging.LogInfo(logging.KeyApp, "successfully deleted media file: %s", fullMediaPath)

	// return updated media list with current filter preserved
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get media files after deletion: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to refresh media list"))
		return
	}

	// get orphaned media from cache
	orphanedMedia, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get orphaned media: %v", err)
		orphanedMedia = []string{}
	}

	// filter media files
	filteredMedia := files.FilterMediaFiles(mediaFiles, orphanedMedia, filter)

	// render updated media list
	html := render.RenderMediaList(filteredMedia, filter, len(mediaFiles), len(orphanedMedia), 0)
	writeResponse(w, r, filteredMedia, html)
}

// @Summary Get media preview HTML
// @Description Returns HTML for a media preview image
// @Tags media
// @Param path query string true "media file path"
// @Param size query int false "preview size in pixels (default from settings)"
// @Produce html
// @Success 200 {string} string "HTML preview element"
// @Failure 400 {string} string "invalid request"
// @Failure 404 {string} string "media file not found"
// @Router /api/media/preview [get]
func handleAPIMediaPreview(w http.ResponseWriter, r *http.Request) {
	if !configmanager.GetPreviewsEnabled() {
		writeAPIError(w, r, http.StatusNotImplemented, translation.SprintfForRequest(configmanager.GetLanguage(), "previews are disabled"))
		return
	}

	mediaPath := r.URL.Query().Get("path")
	if mediaPath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing path parameter"))
		return
	}

	// parse size parameter (optional)
	size := configmanager.GetDefaultPreviewSize()
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if parsedSize, err := strconv.Atoi(sizeStr); err == nil && parsedSize > 0 {
			size = parsedSize
		}
	}

	// render preview HTML using simple CSS approach
	html := render.RenderMediaPreviewWithSize(mediaPath, size)
	writeResponse(w, r, map[string]any{"path": mediaPath, "size": size}, html)
}

// @Summary Get media storage statistics
// @Description Returns statistics about media file storage (total, used, orphaned)
// @Tags media
// @Produce json,html
// @Success 200 {object} map[string]interface{} "storage statistics"
// @Failure 500 {string} string "internal error"
// @Router /api/media/stats [get]
func handleAPIMediaStats(w http.ResponseWriter, r *http.Request) {
	stats, err := files.GetMediaStorageStats()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get media storage stats: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get storage stats"))
		return
	}

	writeResponse(w, r, stats, render.RenderMediaStorageStats(stats))
}

// @Summary Cleanup orphaned media files
// @Description Deletes all orphaned media files (files not referenced by any documents)
// @Tags media
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {object} map[string]interface{} "cleanup result"
// @Failure 500 {string} string "internal error"
// @Router /api/media/cleanup-orphaned [post]
func handleAPICleanupOrphanedMedia(w http.ResponseWriter, r *http.Request) {
	result, err := job.RunMediaCleanup()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}

	if result.Deleted == 0 && result.Failed == 0 {
		msg := translation.SprintfForRequest(configmanager.GetLanguage(), "no orphaned media files to clean up")
		html := render.RenderStatusMessage(render.StatusInfo, msg)
		writeResponse(w, r, map[string]interface{}{"deleted": 0, "message": msg}, html)
		return
	}

	sizeStr := fmt.Sprintf("%.2f MB", float64(result.Size)/(1024*1024))
	msg := fmt.Sprintf("%s %d %s (%s)",
		translation.SprintfForRequest(configmanager.GetLanguage(), "deleted"),
		result.Deleted,
		translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned media files"),
		sizeStr)
	if result.Failed > 0 {
		msg += fmt.Sprintf(". %s: %d", translation.SprintfForRequest(configmanager.GetLanguage(), "failed"), result.Failed)
	}

	html := render.RenderStatusMessage(render.StatusOK, msg)
	writeResponse(w, r, map[string]interface{}{
		"deleted": result.Deleted,
		"size":    result.Size,
		"failed":  result.Failed,
		"message": msg,
	}, html)
}

// @Summary Rename a media file
// @Description Renames a media file and updates all document links pointing to it
// @Tags media
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "Current media file path (without media/ prefix)"
// @Param newpath formData string true "New media file path (without media/ prefix)"
// @Produce html
// @Success 200 {string} string "success"
// @Failure 400 {string} string "invalid request"
// @Failure 404 {string} string "file not found"
// @Failure 409 {string} string "target path already exists"
// @Failure 500 {string} string "rename failed"
// @Router /api/media/rename/{filepath} [post]
func handleAPIMediaRename(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form data"))
		return
	}

	currentRel := chi.URLParam(r, "*")
	if currentRel == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
		return
	}

	newRel := strings.TrimSpace(r.FormValue("newpath"))
	if newRel == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "new path is required"))
		return
	}

	newRel = strings.TrimPrefix(newRel, "media/")
	newRel = filepath.Clean(newRel)

	if currentRel == newRel {
		writeResponse(w, r, nil, render.RenderMediaPathDisplay(newRel))
		return
	}

	err := files.MoveMediaFileNoRefresh(currentRel, newRel)
	msgs := moveErrorMessages{
		sourceMissing: translation.SprintfForRequest(configmanager.GetLanguage(), "file does not exist"),
		targetExists:  translation.SprintfForRequest(configmanager.GetLanguage(), "file with new path already exists"),
		moveFailed:    translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rename file"),
	}
	if handleMoveError(err, "media rename", currentRel, newRel, msgs, func(status int, message string) {
		writeAPIError(w, r, status, message)
	}) {
		return
	}
	files.RefreshCaches()

	logging.LogInfo(logging.KeyApp, "media renamed: media/%s -> media/%s", currentRel, newRel)

	// redirect to the new media detail page
	w.Header().Set("HX-Redirect", "/media/"+newRel+"?mode=detail")
	writeResponse(w, r, nil, render.RenderMediaPathDisplay(newRel))
}

// @Summary Get media rename form
// @Tags media
// @Param filepath path string true "Media file path (without media/ prefix)"
// @Produce html
// @Router /api/media/rename-form/{filepath} [get]
func handleAPIMediaRenameForm(w http.ResponseWriter, r *http.Request) {
	relativePath := chi.URLParam(r, "*")
	writeResponse(w, r, nil, render.RenderMediaRenameForm(relativePath))
}

// @Summary Get media path display
// @Tags media
// @Param filepath path string true "Media file path (without media/ prefix)"
// @Produce html
// @Router /api/media/path-display/{filepath} [get]
func handleAPIMediaPathDisplay(w http.ResponseWriter, r *http.Request) {
	relativePath := chi.URLParam(r, "*")
	writeResponse(w, r, nil, render.RenderMediaPathDisplay(relativePath))
}
