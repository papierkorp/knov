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
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/utils"

	"github.com/go-chi/chi/v5"
)

// @Summary Upload media file
// @Description Upload a media file with context path for directory mirroring
// @Tags media
// @Accept multipart/form-data
// @Param file formData file true "Media file to upload"
// @Param context_path formData string true "url path of the page the file is uploaded from (/files/edit/<path> or /files/<path>) - the media file mirrors that doc's folder"
// @Produce json,html
// @Success 200 {object} map[string]string "Upload success with file path"
// @Failure 400 {string} string "invalid request"
// @Failure 413 {string} string "file too large"
// @Failure 415 {string} string "unsupported file type"
// @Failure 500 {string} string "upload failed"
// @Router /api/media/upload [post]
func handleAPIMediaUpload(w http.ResponseWriter, r *http.Request) {
	// the doc the upload comes from - none for an unsaved file (/files/new/...)
	contextPath := pathutils.FileFromURL(r.FormValue("context_path"))
	if contextPath == "" {
		logging.LogWarning(logging.KeyApp, "media upload attempted without a saved file: %s", r.FormValue("context_path"))
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
		"link":        result.Link,
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
// @Param link query string false "wiki or markdown - also return each media file as ready-to-insert link text"
// @Produce json,html
// @Success 200 {array} object "array of {value, label, detail, link}"
// @Router /api/media/autocomplete [get]
func handleAPIMediaAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	linkKind, withLink := linkKindParam(r)

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
		if withLink {
			results[i].Link = parser.Link{Kind: linkKind, Path: "media/" + rel}.Dest()
		}
	}

	writeResponse(w, r, results, render.RenderAutocompleteList(results))
}

// @Summary Delete media file
// @Description Deletes a media file and its metadata
// @Tags media
// @Param mediapath path string true "Media file path to delete"
// @Produce json,html
// @Success 200 {string} string "success message"
// @Failure 400 {string} string "missing media path"
// @Failure 404 {string} string "media file not found"
// @Failure 409 {string} string "media file still referenced"
// @Failure 500 {string} string "internal error"
// @Router /api/media/{mediapath} [delete]
func handleAPIDeleteMedia(w http.ResponseWriter, r *http.Request) {
	// read the raw request path instead of chi.URLParam(r, "*") - chi's wildcard
	// capture returns the still-percent-encoded RawPath segment when one is set,
	// so a caller that escapes "/" within a path (e.g. url.PathEscape on a nested
	// media path) would otherwise arrive here as a literal "%2F" instead of a
	// folder separator. r.URL.Path is always fully decoded. this assumes the
	// route stays mounted at exactly "/api/media/" (see server.go) - if that
	// prefix ever changes, update it here too.
	mediaPath := strings.TrimPrefix(r.URL.Path, "/api/media/")
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
// @Produce json,html
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
// @Success 200 {object} files.MediaStorageStats
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

// @Summary List orphaned media files
// @Description Lists media files that no document links to
// @Tags media
// @Produce json,html
// @Success 200 {array} string
// @Failure 500 {string} string "failed to get orphaned media"
// @Router /api/media/orphaned [get]
func handleAPIGetOrphanedMedia(w http.ResponseWriter, r *http.Request) {
	paths, err := files.ScanOrphanedMedia()
	if err != nil {
		logging.LogError(logging.KeyMediaCleanup, "failed to get orphaned media: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get orphaned media"))
		return
	}

	writeResponse(w, r, paths, render.RenderOrphanedMedia(paths))
}

// @Summary Cleanup orphaned media files
// @Description Deletes the selected orphaned media files (files not referenced by any documents)
// @Tags media
// @Accept application/x-www-form-urlencoded
// @Param path formData []string false "Orphaned media paths to delete (media/...), repeatable" collectionFormat(multi)
// @Produce json,html
// @Success 200 {object} job.MediaCleanupResult
// @Failure 400 {string} string "no media files selected"
// @Failure 409 {string} string "job already running"
// @Failure 500 {string} string "internal error or all selected files failed to delete"
// @Router /api/media/cleanup-orphaned [post]
func handleAPICleanupOrphanedMedia(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	paths := r.Form["path"]
	if len(paths) == 0 {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no media files selected"))
		return
	}

	result, err := job.RunMediaCleanup(paths)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}

	level := notify.LevelSuccess
	if result.Deleted == 0 {
		level = notify.LevelInfo
	}
	msg := translation.SprintfForRequest(configmanager.GetLanguage(), "%d orphaned media files deleted (%s)", result.Deleted, utils.FormatFileSize(result.Size))
	if result.Failed > 0 {
		msg = translation.SprintfForRequest(configmanager.GetLanguage(), "%d orphaned media files deleted (%s), %d failed", result.Deleted, utils.FormatFileSize(result.Size), result.Failed)
		if result.Deleted == 0 {
			writeAPIError(w, r, http.StatusInternalServerError, msg)
			return
		}
		level = notify.LevelWarning
	}
	notify.SetHeader(w, level, msg)

	// the cleanup itself succeeded, so a failed rescan only replaces the list
	var html string
	if paths, err := files.GetOrphanedMediaFromCache(); err != nil {
		logging.LogError(logging.KeyMediaCleanup, "failed to get orphaned media: %v", err)
		html = render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get orphaned media"))
	} else {
		html = render.RenderOrphanedMedia(paths)
	}
	writeResponse(w, r, result, html)
}

// @Summary Scan for misplaced media files
// @Description Lists non-text files in the docs folder (e.g. images copied in from another wiki) with their planned media destination. Files not matching the allowed mime types have no destination and are only reported.
// @Tags media
// @Produce json,html
// @Success 200 {array} files.MisplacedMedia
// @Failure 500 {string} string "failed to scan for misplaced media"
// @Router /api/media/misplaced [get]
func handleAPIGetMisplacedMedia(w http.ResponseWriter, r *http.Request) {
	items, err := files.ScanMisplacedMedia()
	if err != nil {
		logging.LogError(logging.KeyMediaRelocate, "failed to scan for misplaced media: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to scan for misplaced media"))
		return
	}

	writeResponse(w, r, items, render.RenderMisplacedMedia(items))
}

// @Summary Relocate misplaced media files
// @Description Moves the selected allowed-type media files from the docs folder into the media folder (mirroring their folder) and rewrites every link pointing to them
// @Tags media
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param path formData []string true "docs-relative paths of the misplaced media files to move" collectionFormat(multi)
// @Success 200 {object} files.MediaRelocateResult
// @Failure 400 {string} string "no media files selected"
// @Failure 409 {string} string "job already running"
// @Failure 500 {string} string "internal error"
// @Router /api/media/misplaced/relocate [post]
func handleAPIRelocateMisplacedMedia(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	paths := r.Form["path"]
	if len(paths) == 0 {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no media files selected"))
		return
	}

	result, err := job.RunMediaRelocate(paths)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}

	level := notify.LevelSuccess
	if result.Failed > 0 {
		level = notify.LevelError
	}
	notify.SetHeader(w, level, translation.SprintfForRequest(configmanager.GetLanguage(), "%d media files moved, %d failed, %d files updated", result.Moved, result.Failed, result.FilesUpdated))

	// the relocate itself succeeded, so a failed rescan only replaces the list
	html := ""
	if items, err := files.ScanMisplacedMedia(); err != nil {
		logging.LogError(logging.KeyMediaRelocate, "failed to rescan for misplaced media: %v", err)
		html = render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to scan for misplaced media"))
	} else {
		html = render.RenderMisplacedMedia(items)
	}
	writeResponse(w, r, result, html)
}

// @Summary Rename a media file
// @Description Renames a media file and updates all document links pointing to it
// @Tags media
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "Current media file path (without media/ prefix)"
// @Param newpath formData string true "New media file path (without media/ prefix)"
// @Produce json,html
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
	w.Header().Set("HX-Redirect", pathutils.ToMediaURL(newRel)+"?mode=detail")
	writeResponse(w, r, nil, render.RenderMediaPathDisplay(newRel))
}

// @Summary Get media rename form
// @Tags media
// @Param filepath path string true "Media file path (without media/ prefix)"
// @Produce json,html
// @Router /api/media/rename-form/{filepath} [get]
func handleAPIMediaRenameForm(w http.ResponseWriter, r *http.Request) {
	relativePath := chi.URLParam(r, "*")
	writeResponse(w, r, nil, render.RenderMediaRenameForm(relativePath))
}

// @Summary Get media path display
// @Tags media
// @Param filepath path string true "Media file path (without media/ prefix)"
// @Produce json,html
// @Router /api/media/path-display/{filepath} [get]
func handleAPIMediaPathDisplay(w http.ResponseWriter, r *http.Request) {
	relativePath := chi.URLParam(r, "*")
	writeResponse(w, r, nil, render.RenderMediaPathDisplay(relativePath))
}
