package files

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/pathutils"
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
	// get max upload size from settings
	maxUploadSize := configmanager.GetMaxUploadSize()

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

	// strip docs/ prefix from context dir to avoid media/docs/... paths
	// media should mirror docs structure without the docs/ prefix
	if contextDir == "docs" {
		contextDir = ""
	} else {
		contextDir = pathutils.ToRelative(contextDir)
	}

	// sanitize filename
	sanitizedName := utils.SanitizeFilename(header.Filename, 255, true, false)

	// create media path mirroring docs structure
	var mediaPath string
	if contextDir != "" {
		mediaPath = filepath.Join(contextDir, sanitizedName)
	} else {
		mediaPath = sanitizedName
	}

	// resolve filename conflicts
	finalMediaPath := utils.ResolveFilenameConflicts(pathutils.ToMediaPath(mediaPath), mediaPath)

	// get full file system path using contentStorage
	fullMediaPath := pathutils.ToMediaPath(finalMediaPath)

	// write file to disk using contentStorage
	if err := contentStorage.WriteFile(fullMediaPath, fileBytes, 0644); err != nil {
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

	// return result with relative path for markdown links
	return &MediaUploadResult{
		Path:        finalMediaPath, // Return just the relative path without media/ prefix
		Filename:    filepath.Base(finalMediaPath),
		ContentType: contentType,
		Size:        strconv.Itoa(len(fileBytes)),
	}, nil
}

// IsImageFile checks if file extension represents an image
func IsImageFile(ext string) bool {
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico"}
	for _, imageExt := range imageExts {
		if ext == imageExt {
			return true
		}
	}
	return false
}

// IsVideoFile checks if file extension represents a video
func IsVideoFile(ext string) bool {
	videoExts := []string{".mp4", ".webm", ".ogg", ".avi", ".mov", ".wmv", ".flv", ".mkv"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// IsAudioFile checks if file extension represents audio
func IsAudioFile(ext string) bool {
	audioExts := []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".wma"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

// GetFileTypeIcon returns appropriate Font Awesome icon for file type
func GetFileTypeIcon(ext string) string {
	switch {
	case IsImageFile(ext):
		return "fa-image"
	case IsVideoFile(ext):
		return "fa-video"
	case IsAudioFile(ext):
		return "fa-music"
	case ext == ".pdf":
		return "fa-file-pdf"
	case ext == ".doc" || ext == ".docx":
		return "fa-file-word"
	case ext == ".xls" || ext == ".xlsx":
		return "fa-file-excel"
	case ext == ".ppt" || ext == ".pptx":
		return "fa-file-powerpoint"
	case ext == ".txt":
		return "fa-file-alt"
	case ext == ".zip" || ext == ".rar" || ext == ".7z":
		return "fa-file-archive"
	default:
		return "fa-file"
	}
}
