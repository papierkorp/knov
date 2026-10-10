package files

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/types"
	"knov/internal/utils"
)

// MediaUploadResult contains the result of a media upload operation
type MediaUploadResult struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        string `json:"size"`
	Link        string `json:"link"` // ready-to-insert markdown link (image link for images)
}

// UploadMedia handles the core media upload logic - contextPath is the doc the file is uploaded
// from, the media path mirrors its folder
func UploadMedia(file multipart.File, header *multipart.FileHeader, contextPath pathutils.MetaPath) (*MediaUploadResult, error) {
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

	// sanitize filename first, so the stored name is the one validated
	sanitizedName := utils.SanitizeFilename(header.Filename, 255, true, false)

	// validate MIME type
	if !ValidateMediaType(sanitizedName, contentType) {
		logging.LogWarning(logging.KeyApp, "unsupported media type: %s", contentType)
		return nil, fmt.Errorf("unsupported file type")
	}

	// the media path mirrors the folder of the doc the file is uploaded from
	mediaPath := sanitizedName
	if dir := path.Dir(contextPath.Rel()); dir != "." {
		mediaPath = dir + "/" + sanitizedName
	}

	// resolve filename conflicts
	finalMediaPath := pathutils.ToSlash(utils.ResolveFilenameConflicts(pathutils.MediaPath(mediaPath).FullPath(), mediaPath))

	// get full file system path using contentStorage
	fullMediaPath := pathutils.MediaPath(finalMediaPath).FullPath()

	// write file to disk using contentStorage
	if err := contentStorage.WriteFile(fullMediaPath, fileBytes, 0644); err != nil {
		logging.LogError(logging.KeyApp, "failed to write media file %s: %v", fullMediaPath, err)
		return nil, fmt.Errorf("failed to save file")
	}

	// create metadata for the media file with proper path prefix
	metadataPath := pathutils.MediaPath(pathutils.ToSlash(finalMediaPath)) // Add media/ prefix to distinguish from docs

	// Editor is intentionally left unset for media files — recomputeDerivedFields skips
	// the editor fallback for media/ paths, so it stays empty. Filtering uses the path
	// prefix + mime type via isHiddenByType instead.
	if err := MetaDataSync(metadataPath); err != nil {
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
		Link: parser.Link{
			Kind:  parser.LinkMarkdown,
			Image: strings.HasPrefix(contentType, "image/"),
			Text:  filepath.Base(finalMediaPath),
			Path:  "media/" + finalMediaPath,
		}.String(),
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
	switch strings.ToLower(ext) {
	case ".pdf":
		return "fa-file-pdf"
	case ".doc", ".docx", ".odt", ".rtf":
		return "fa-file-word"
	case ".xls", ".xlsx", ".ods":
		return "fa-file-excel"
	case ".ppt", ".pptx", ".odp":
		return "fa-file-powerpoint"
	}
	switch types.MediaCategory(ext) {
	case types.MediaCategoryImage:
		return "fa-image"
	case types.MediaCategoryVideo:
		return "fa-video"
	case types.MediaCategoryAudio:
		return "fa-music"
	case types.MediaCategoryText:
		return "fa-file-alt"
	case types.MediaCategoryArchive:
		return "fa-file-archive"
	}
	return "fa-file"
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
			if orphaned == media.Path.String() {
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

// MediaStorageStats contains statistics about media file storage, the embedded
// MediaCategoryStats holds the totals over all categories (Category is empty)
type MediaStorageStats struct {
	MediaCategoryStats
	Categories []MediaCategoryStats `json:"categories"` // per MediaCategory, sorted by category
}

// MediaCategoryStats holds the file counts and sizes of one MediaCategory
type MediaCategoryStats struct {
	Category      string `json:"category,omitempty"`
	TotalFiles    int    `json:"totalFiles"`
	TotalSize     int64  `json:"totalSize"`
	UsedFiles     int    `json:"usedFiles"`
	UsedSize      int64  `json:"usedSize"`
	OrphanedFiles int    `json:"orphanedFiles"`
	OrphanedSize  int64  `json:"orphanedSize"`
}

// add counts one file of size as used or orphaned
func (s *MediaCategoryStats) add(size int64, orphaned bool) {
	s.TotalFiles++
	s.TotalSize += size
	if orphaned {
		s.OrphanedFiles++
		s.OrphanedSize += size
	} else {
		s.UsedFiles++
		s.UsedSize += size
	}
}

// GetMediaStorageStats returns statistics about media file storage
func GetMediaStorageStats() (*MediaStorageStats, error) {
	stats := &MediaStorageStats{Categories: []MediaCategoryStats{}}

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

	orphaned := make(map[string]bool, len(orphanedMedia))
	for _, p := range orphanedMedia {
		orphaned[p] = true
	}

	// calculate stats
	byCategory := map[string]*MediaCategoryStats{}
	for _, file := range mediaFiles {
		// size stays 0 if the file info can't be read
		var fileSize int64
		fullPath := file.Path.FullPath()
		if fileInfo, err := contentStorage.GetFileInfo(fullPath); err == nil && fileInfo != nil {
			fileSize = fileInfo.Size()
		}

		c := types.MediaCategory(file.Path.String())
		if byCategory[c] == nil {
			byCategory[c] = &MediaCategoryStats{Category: c}
		}
		byCategory[c].add(fileSize, orphaned[file.Path.String()])
		stats.add(fileSize, orphaned[file.Path.String()])
	}

	for _, c := range byCategory {
		stats.Categories = append(stats.Categories, *c)
	}
	slices.SortFunc(stats.Categories, func(a, b MediaCategoryStats) int { return strings.Compare(a.Category, b.Category) })

	return stats, nil
}
