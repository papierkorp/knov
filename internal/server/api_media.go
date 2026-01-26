// Package server - Media upload API endpoints
package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
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
		logging.LogWarning("media upload attempted without context path")
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "save document first to enable media uploads"), http.StatusBadRequest)
		return
	}

	// prevent uploads to unsaved files (context_path like "new")
	if contextPath == "new" || strings.HasPrefix(contextPath, "new/") {
		logging.LogWarning("media upload attempted for unsaved file: %s", contextPath)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "save document first to enable media uploads"), http.StatusBadRequest)
		return
	}

	// parse multipart form with size limit
	maxUploadSize := configmanager.GetMaxUploadSize()

	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		logging.LogError("failed to parse multipart form: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse upload form"), http.StatusBadRequest)
		return
	}

	// get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		logging.LogError("failed to get uploaded file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "no file uploaded"), http.StatusBadRequest)
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()), statusCode)
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
// @Success 200 {object} map[string]interface{} "List of media files"
// @Failure 500 {string} string "internal error"
// @Router /api/media/list [get]
func handleAPIGetAllMedia(w http.ResponseWriter, r *http.Request) {
	// get filter parameter (all, used, orphaned)
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all" // default
	}

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		logging.LogError("failed to get media files: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load media files"), http.StatusInternalServerError)
		return
	}

	// get orphaned media from cache
	orphanedMedia, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning("failed to get orphaned media: %v", err)
		orphanedMedia = []string{} // fallback to empty
	}

	// filter media files based on filter parameter
	filteredMedia := files.FilterMediaFiles(mediaFiles, orphanedMedia, filter)

	// determine response format
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		// render HTML response with filter
		html := render.RenderMediaList(filteredMedia, filter, len(mediaFiles), len(orphanedMedia))
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	// return JSON response
	writeResponse(w, r, filteredMedia, fmt.Sprintf("found %d media files", len(filteredMedia)))
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing media path"), http.StatusBadRequest)
		return
	}

	// add media prefix if not present
	fullMediaPath := mediaPath
	if !strings.HasPrefix(mediaPath, "media/") {
		fullMediaPath = "media/" + mediaPath
	}

	logging.LogInfo("deleting media file: %s", fullMediaPath)

	// check if file exists
	fullPath := pathutils.ToMediaPath(strings.TrimPrefix(fullMediaPath, "media/"))
	exists, err := contentStorage.FileExists(fullPath)
	if err != nil || !exists {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "media file not found"), http.StatusNotFound)
		return
	}

	// check if file is still referenced
	metadata, err := files.MetaDataGet(fullMediaPath)
	if err == nil && metadata != nil && len(metadata.LinksToHere) > 0 {
		logging.LogWarning("cannot delete media file %s: still referenced by %d files", fullMediaPath, len(metadata.LinksToHere))

		// get current filter
		filter := r.URL.Query().Get("filter")
		if filter == "" {
			filter = "all"
		}

		// get all media files for re-rendering
		mediaFiles, err := files.GetAllMediaFiles()
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load media files"), http.StatusInternalServerError)
			return
		}

		// get orphaned media from cache
		orphanedMedia, err := files.GetOrphanedMediaFromCache()
		if err != nil {
			orphanedMedia = []string{}
		}

		// filter media files
		filteredMedia := files.FilterMediaFiles(mediaFiles, orphanedMedia, filter)

		// build error message with referencing files
		var referencingFiles []string
		maxShow := 5
		for i, ref := range metadata.LinksToHere {
			if i >= maxShow {
				referencingFiles = append(referencingFiles, fmt.Sprintf("... %s %d %s",
					translation.SprintfForRequest(configmanager.GetLanguage(), "and"),
					len(metadata.LinksToHere)-maxShow,
					translation.SprintfForRequest(configmanager.GetLanguage(), "more")))
				break
			}
			referencingFiles = append(referencingFiles, fmt.Sprintf(`<a href="/files/%s" target="_blank">%s</a>`, ref, ref))
		}

		errorMsg := fmt.Sprintf(`<div class="status-error" style="margin: 1rem; padding: 1rem; border-radius: 4px;">
			<strong>%s</strong><br><br>
			%s:<br>%s
		</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "cannot delete media file"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "still referenced by"),
			strings.Join(referencingFiles, "<br>"))

		// render media list with error message at the top
		mediaListHTML := render.RenderMediaList(filteredMedia, filter, len(mediaFiles), len(orphanedMedia))

		// inject error message at the beginning of the media content
		finalHTML := strings.Replace(mediaListHTML, `<div id="component-media-content">`,
			`<div id="component-media-content">`+errorMsg, 1)

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK) // use 200 so htmx swaps the content
		w.Write([]byte(finalHTML))
		return
	}

	// delete file from filesystem
	if err := contentStorage.DeleteFile(fullPath); err != nil {
		logging.LogError("failed to delete media file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete file"), http.StatusInternalServerError)
		return
	}

	// delete metadata
	if err := files.MetaDataDelete(fullMediaPath); err != nil {
		logging.LogWarning("failed to delete metadata for media file %s: %v", fullMediaPath, err)
		// don't fail the whole operation, just log warning
	}

	logging.LogInfo("successfully deleted media file: %s", fullMediaPath)

	// return updated media list with current filter preserved
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		logging.LogError("failed to get media files after deletion: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to refresh media list"), http.StatusInternalServerError)
		return
	}

	// get orphaned media from cache
	orphanedMedia, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning("failed to get orphaned media: %v", err)
		orphanedMedia = []string{}
	}

	// filter media files
	filteredMedia := files.FilterMediaFiles(mediaFiles, orphanedMedia, filter)

	// render updated media list
	html := render.RenderMediaList(filteredMedia, filter, len(mediaFiles), len(orphanedMedia))
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "previews are disabled"), http.StatusNotImplemented)
		return
	}

	mediaPath := r.URL.Query().Get("path")
	if mediaPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing path parameter"), http.StatusBadRequest)
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

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
