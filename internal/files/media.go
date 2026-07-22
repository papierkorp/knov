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
		logging.LogError(logging.KeyApp, "failed to read uploaded file: %v", err)
		return nil, fmt.Errorf("failed to read uploaded file")
	}

	// check file size after reading
	if int64(len(fileBytes)) > maxUploadSize {
		logging.LogWarning(logging.KeyApp, "uploaded file too large: %d bytes (max: %d)", len(fileBytes), maxUploadSize)
		return nil, fmt.Errorf("file too large")
	}

	// detect content type
	contentType := http.DetectContentType(fileBytes)

	// validate MIME type
	if !ValidateMediaMimeType(contentType) {
		logging.LogWarning(logging.KeyApp, "unsupported media type: %s", contentType)
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
		mediaPath = filepath.ToSlash(filepath.Join(contextDir, sanitizedName))
	} else {
		mediaPath = sanitizedName
	}

	// resolve filename conflicts
	finalMediaPath := utils.ResolveFilenameConflicts(pathutils.ToMediaPath(mediaPath), mediaPath)

	// get full file system path using contentStorage
	fullMediaPath := pathutils.ToMediaPath(finalMediaPath)

	// write file to disk using contentStorage
	if err := contentStorage.WriteFile(fullMediaPath, fileBytes, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write media file %s: %v", fullMediaPath, err)
		return nil, fmt.Errorf("failed to save file")
	}

	// create metadata for the media file with proper path prefix
	metadataPath := "media/" + filepath.ToSlash(finalMediaPath) // Add media/ prefix to distinguish from docs

	metadata := &Metadata{
		Path: metadataPath,
		// Editor is intentionally not set for media files — metaDataUpdate skips
		// the editor fallback for media/ paths, so it stays empty.
		// Filtering uses the path prefix + mime type via isHiddenByType instead.
	}

	if err := MetaDataSave(metadata); err != nil {
		logging.LogError(logging.KeyApp, "failed to save metadata for media file %s: %v", metadataPath, err)
		// don't fail the whole request, just log the error
	} else {
		logging.LogInfo(logging.KeyApp, "created metadata for media file: %s", metadataPath)

		// update links for this media file (scan all files to find references)
		if err := UpdateLinksForSingleFile(metadataPath); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to update links for media file %s: %v", metadataPath, err)
			// don't fail the request, just log the error
		}
	}

	logging.LogInfo(logging.KeyApp, "uploaded media file: %s (%s, %d bytes)", fullMediaPath, contentType, len(fileBytes))

	// return result with relative path for markdown links
	return &MediaUploadResult{
		Path:        finalMediaPath, // Return just the relative path without media/ prefix
		Filename:    filepath.Base(finalMediaPath),
		ContentType: contentType,
		Size:        strconv.Itoa(len(fileBytes)),
	}, nil
}

// IsImageFile checks if file extension represents an image
// IsImageFile checks if file extension represents an image
func IsImageFile(ext string) bool {
	return configmanager.IsImageExtension(ext)
}

// IsVideoFile checks if file extension represents a video
func IsVideoFile(ext string) bool {
	return configmanager.IsVideoExtension(ext)
}

// IsAudioFile checks if file extension represents audio
func IsAudioFile(ext string) bool {
	return configmanager.IsAudioExtension(ext)
}

// GetFileTypeIcon returns appropriate Font Awesome icon for file type
func GetFileTypeIcon(ext string) string {
	mimeType := configmanager.MimeTypeByExtension(ext)
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "fa-image"
	case strings.HasPrefix(mimeType, "video/"):
		return "fa-video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "fa-music"
	case mimeType == "application/pdf":
		return "fa-file-pdf"
	case mimeType == "application/msword" || mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "fa-file-word"
	case mimeType == "application/vnd.ms-excel" || mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "fa-file-excel"
	case mimeType == "application/vnd.ms-powerpoint" || mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "fa-file-powerpoint"
	case strings.HasPrefix(mimeType, "text/"):
		return "fa-file-alt"
	case mimeType == "application/zip" || mimeType == "application/x-rar-compressed" || mimeType == "application/x-7z-compressed":
		return "fa-file-archive"
	default:
		return "fa-file"
	}
}

// FilterMediaFiles filters media files based on orphaned status
func FilterMediaFiles(mediaFiles []File, orphanedMedia []string, filter string) []File {
	if filter == "all" {
		return mediaFiles
	}

	var filtered []File
	for _, media := range mediaFiles {
		isOrphaned := false
		for _, orphaned := range orphanedMedia {
			if orphaned == media.Path {
				isOrphaned = true
				break
			}
		}

		if filter == "orphaned" && isOrphaned {
			filtered = append(filtered, media)
		} else if filter == "used" && !isOrphaned {
			filtered = append(filtered, media)
		}
	}

	return filtered
}

// MediaStorageStats contains statistics about media file storage
type MediaStorageStats struct {
	TotalFiles    int
	TotalSize     int64
	UsedFiles     int
	UsedSize      int64
	OrphanedFiles int
	OrphanedSize  int64
}

// GetMediaStorageStats returns statistics about media file storage
func GetMediaStorageStats() (*MediaStorageStats, error) {
	stats := &MediaStorageStats{}

	// get all media files
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		return nil, err
	}

	// get orphaned media from cache
	orphanedMedia, err := GetOrphanedMediaFromCache()
	if err != nil {
		orphanedMedia = []string{}
	}

	// calculate stats
	for _, file := range mediaFiles {
		stats.TotalFiles++

		// get file size from filesystem
		fullPath := pathutils.ToMediaPath(strings.TrimPrefix(file.Path, "media/"))
		fileInfo, err := contentStorage.GetFileInfo(fullPath)
		if err == nil && fileInfo != nil {
			fileSize := fileInfo.Size()
			stats.TotalSize += fileSize

			isOrphaned := false
			for _, orphaned := range orphanedMedia {
				if orphaned == file.Path {
					isOrphaned = true
					break
				}
			}

			if isOrphaned {
				stats.OrphanedFiles++
				stats.OrphanedSize += fileSize
			} else {
				stats.UsedFiles++
				stats.UsedSize += fileSize
			}
		}
	}

	return stats, nil
}
