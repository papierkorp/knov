package files

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/utils"
)

// MediaUploadResult contains the result of a media upload operation
type MediaUploadResult struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        string `json:"size"`
}

// UploadMedia handles the core media upload logic
func UploadMedia(file multipart.File, header *multipart.FileHeader, contextPath string) (*MediaUploadResult, error) {
	// get max upload size
	maxUploadSize := int64(configmanager.GetUserSettings().MediaSettings.MaxUploadSizeMB) * 1024 * 1024
	if maxUploadSize == 0 {
		maxUploadSize = 10 * 1024 * 1024 // 10MB default
	}

	// read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		logging.LogError("failed to read uploaded file: %v", err)
		return nil, fmt.Errorf("failed to read uploaded file")
	}

	// check file size after reading
	if int64(len(fileBytes)) > maxUploadSize {
		logging.LogWarning("uploaded file too large: %d bytes (max: %d)", len(fileBytes), maxUploadSize)
		return nil, fmt.Errorf("file too large")
	}

	// detect content type
	contentType := http.DetectContentType(fileBytes)

	// validate MIME type
	if !ValidateMediaMimeType(contentType) {
		logging.LogWarning("unsupported media type: %s", contentType)
		return nil, fmt.Errorf("unsupported file type")
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

	// resolve filename conflicts
	finalMediaPath := utils.ResolveFilenameConflicts(contentStorage.ToMediaPath(mediaPath), mediaPath)

	// get full file system path using contentStorage
	fullMediaPath := contentStorage.ToMediaPath(finalMediaPath)

	// create directory if it doesn't exist using contentStorage
	dir := filepath.Dir(fullMediaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create media directory %s: %v", dir, err)
		return nil, fmt.Errorf("failed to create directory")
	}

	// write file to disk using contentStorage path
	if err := os.WriteFile(fullMediaPath, fileBytes, 0644); err != nil {
		logging.LogError("failed to write media file %s: %v", fullMediaPath, err)
		return nil, fmt.Errorf("failed to save file")
	}

	// determine file type for metadata
	var fileType Filetype = FileTypeImage // default
	switch {
	case strings.HasPrefix(contentType, "image/"):
		fileType = FileTypeImage
	case strings.HasPrefix(contentType, "video/"):
		fileType = FileTypeVideo
	case strings.HasPrefix(contentType, "text/"):
		fileType = FileTypeText
	case contentType == "application/pdf":
		fileType = FileTypePDF
	}

	// create metadata for the media file with proper path prefix
	metadataPath := filepath.Join("media", finalMediaPath) // Add media/ prefix to distinguish from docs
	metadata := &Metadata{
		Path:     metadataPath,
		FileType: fileType,
	}

	if err := MetaDataSave(metadata); err != nil {
		logging.LogError("failed to save metadata for media file %s: %v", metadataPath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo("created metadata for media file: %s (filetype: %s)", metadataPath, fileType)
	}

	logging.LogInfo("uploaded media file: %s (%s, %d bytes)", fullMediaPath, contentType, len(fileBytes))

	// return result
	return &MediaUploadResult{
		Path:        finalMediaPath,
		Filename:    filepath.Base(finalMediaPath),
		ContentType: contentType,
		Size:        strconv.Itoa(len(fileBytes)),
	}, nil
}
