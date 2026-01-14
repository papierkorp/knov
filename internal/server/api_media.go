// Package server - Media upload API endpoints
package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/translation"
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
	maxUploadSize := int64(configmanager.GetUserSettings().MediaSettings.MaxUploadSizeMB) * 1024 * 1024
	if maxUploadSize == 0 {
		maxUploadSize = 10 * 1024 * 1024 // 10MB default
	}

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
