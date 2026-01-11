// Package contentStorage provides content storage directory management and path utilities
package contentStorage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
)

// Init initializes content storage directories
func Init() error {
	dataPath := configmanager.GetAppConfig().DataPath

	// create main data directory
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// create docs subdirectory
	docsPath := filepath.Join(dataPath, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		return fmt.Errorf("failed to create docs directory: %w", err)
	}

	// create media subdirectory
	mediaPath := filepath.Join(dataPath, "media")
	if err := os.MkdirAll(mediaPath, 0755); err != nil {
		return fmt.Errorf("failed to create media directory: %w", err)
	}

	logging.LogInfo("content storage initialized")
	return nil
}

// GetDocsPath returns the full path to docs directory
func GetDocsPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "docs")
}

// GetMediaPath returns the full path to media directory
func GetMediaPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "media")
}

// ToDocsPath converts relative path to full docs path
// Input: "projects/file.md"
// Output: "/full/path/to/data/docs/projects/file.md"
func ToDocsPath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(GetDocsPath(), relativePath)
}

// ToMediaPath converts relative path to full media path
// Input: "images/photo.jpg"
// Output: "/full/path/to/data/media/images/photo.jpg"
func ToMediaPath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(GetMediaPath(), relativePath)
}

// ToRelativePath removes data directory prefix to get relative path
// Input: "/full/path/to/data/docs/projects/file.md"
// Output: "projects/file.md"
func ToRelativePath(fullPath string) string {
	dataPath := configmanager.GetAppConfig().DataPath
	dataPathName := filepath.Base(filepath.Clean(dataPath))

	// if path is already relative, check if it starts with data path and strip it
	if !filepath.IsAbs(fullPath) {
		// normalize path separators
		fullPath = filepath.ToSlash(fullPath)

		// strip leading "data/" or "data\" if present
		if strings.HasPrefix(fullPath, dataPathName+"/") {
			return strings.TrimPrefix(fullPath, dataPathName+"/")
		}
		if strings.HasPrefix(fullPath, dataPathName+"\\") {
			return strings.TrimPrefix(fullPath, dataPathName+"\\")
		}

		// strip leading "docs/" or "media/" if present (for subdirectories)
		if strings.HasPrefix(fullPath, "docs/") {
			return strings.TrimPrefix(fullPath, "docs/")
		}
		if strings.HasPrefix(fullPath, "media/") {
			return strings.TrimPrefix(fullPath, "media/")
		}

		return fullPath
	}

	// clean both paths to normalize them (removes trailing slashes, resolves ./ and ../)
	fullPath = filepath.Clean(fullPath)
	dataPath = filepath.Clean(dataPath)

	// try stripping docs path first
	docsPath := GetDocsPath()
	docsPrefix := docsPath + string(filepath.Separator)
	if strings.HasPrefix(fullPath, docsPrefix) {
		return strings.TrimPrefix(fullPath, docsPrefix)
	}

	// try stripping media path
	mediaPath := GetMediaPath()
	mediaPrefix := mediaPath + string(filepath.Separator)
	if strings.HasPrefix(fullPath, mediaPrefix) {
		return strings.TrimPrefix(fullPath, mediaPrefix)
	}

	// try to get relative path from data directory
	relPath, err := filepath.Rel(dataPath, fullPath)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		// if it's in docs/ or media/ subdirectory, strip that prefix
		if strings.HasPrefix(relPath, "docs"+string(filepath.Separator)) {
			return strings.TrimPrefix(relPath, "docs"+string(filepath.Separator))
		}
		if strings.HasPrefix(relPath, "media"+string(filepath.Separator)) {
			return strings.TrimPrefix(relPath, "media"+string(filepath.Separator))
		}
		return relPath
	}

	// fallback: strip data path prefix if it exists
	dataPrefix := dataPath + string(filepath.Separator)
	if strings.HasPrefix(fullPath, dataPrefix) {
		stripped := strings.TrimPrefix(fullPath, dataPrefix)
		// also strip docs/ or media/ if present
		if strings.HasPrefix(stripped, "docs"+string(filepath.Separator)) {
			return strings.TrimPrefix(stripped, "docs"+string(filepath.Separator))
		}
		if strings.HasPrefix(stripped, "media"+string(filepath.Separator)) {
			return strings.TrimPrefix(stripped, "media"+string(filepath.Separator))
		}
		return stripped
	}

	// last resort: if fullPath equals dataPath exactly, return empty
	if fullPath == dataPath {
		return ""
	}

	// if not under data path, return the full path as-is
	// this can happen if files are outside the data directory
	return fullPath
}
