// Package server - Media upload API endpoints
package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/translation"
	"knov/internal/utils"
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

	// read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		logging.LogError("failed to read uploaded file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read uploaded file"), http.StatusInternalServerError)
		return
	}

	// check file size after reading
	if int64(len(fileBytes)) > maxUploadSize {
		logging.LogWarning("uploaded file too large: %d bytes (max: %d)", len(fileBytes), maxUploadSize)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "file too large"), http.StatusRequestEntityTooLarge)
		return
	}

	// detect content type
	contentType := http.DetectContentType(fileBytes)

	// validate MIME type (placeholder for now)
	if !files.ValidateMediaMimeType(contentType) {
		logging.LogWarning("unsupported media type: %s", contentType)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusUnsupportedMediaType)
		return
	}

	// extract directory from context path
	contextDir := filepath.Dir(contextPath)
	if contextDir == "." {
		contextDir = ""
	}

	// sanitize filename
	sanitizedName := utils.SanitizeMediaFilename(header.Filename)

	// create media path mirroring docs structure
	var mediaPath string
	if contextDir != "" {
		mediaPath = filepath.Join(contextDir, sanitizedName)
	} else {
		mediaPath = sanitizedName
	}

	// resolve filename conflicts by appending -1, -2, etc.
	finalMediaPath := resolveMediaFilenameConflicts(mediaPath)

	// get full file system path
	fullMediaPath := contentStorage.ToMediaPath(finalMediaPath)

	// create directory if it doesn't exist
	dir := filepath.Dir(fullMediaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create media directory %s: %v", dir, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"), http.StatusInternalServerError)
		return
	}

	// write file to disk
	if err := os.WriteFile(fullMediaPath, fileBytes, 0644); err != nil {
		logging.LogError("failed to write media file %s: %v", fullMediaPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	// determine file type for metadata
	var fileType files.Filetype = files.FileTypeImage // default
	switch {
	case strings.HasPrefix(contentType, "image/"):
		fileType = files.FileTypeImage
	case strings.HasPrefix(contentType, "video/"):
		fileType = files.FileTypeVideo
	case contentType == "application/pdf":
		fileType = files.FileTypePDF
	}

	// create metadata for the media file
	metadata := &files.Metadata{
		Path:     finalMediaPath,
		FileType: fileType,
	}

	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for media file %s: %v", finalMediaPath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("created metadata for media file: %s (filetype: %s)", finalMediaPath, fileType)
	}

	logging.LogInfo("uploaded media file: %s (%s, %d bytes)", finalMediaPath, contentType, len(fileBytes))

	// return markdown-ready path (relative to docs root)
	responseData := map[string]string{
		"path":        finalMediaPath,
		"filename":    filepath.Base(finalMediaPath),
		"contentType": contentType,
		"size":        strconv.Itoa(len(fileBytes)),
	}

	writeResponse(w, r, responseData, fmt.Sprintf("media uploaded: %s", finalMediaPath))
}

// resolveMediaFilenameConflicts checks for existing files and appends -1, -2, etc. to avoid conflicts
func resolveMediaFilenameConflicts(basePath string) string {
	fullPath := contentStorage.ToMediaPath(basePath)

	// if file doesn't exist, return original path
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return basePath
	}

	// extract filename parts
	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// try appending numbers until we find a non-existing filename
	for i := 1; i < 1000; i++ { // reasonable limit
		newFilename := fmt.Sprintf("%s-%d%s", nameWithoutExt, i, ext)
		var newPath string
		if dir != "" && dir != "." {
			newPath = filepath.Join(dir, newFilename)
		} else {
			newPath = newFilename
		}

		fullNewPath := contentStorage.ToMediaPath(newPath)
		if _, err := os.Stat(fullNewPath); os.IsNotExist(err) {
			return newPath
		}
	}

	// fallback: use timestamp if we couldn't resolve conflicts
	timestamp := utils.SanitizeFilename("", 20) // this generates timestamp
	newFilename := fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext)
	if dir != "" && dir != "." {
		return filepath.Join(dir, newFilename)
	}
	return newFilename
}
