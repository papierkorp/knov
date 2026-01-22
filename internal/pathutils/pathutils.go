// Package pathutils provides centralized filepath conversion and normalization operations
package pathutils

import (
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
)

// PathType represents the type of content path
type PathType int

const (
	// TypeDocs represents documentation files
	TypeDocs PathType = iota
	// TypeMedia represents media files
	TypeMedia
)

// PathInfo contains comprehensive information about a file path
type PathInfo struct {
	Type       PathType // Whether this is docs or media
	Relative   string   // Path without any prefix (e.g. "projects/file.md")
	WithPrefix string   // Path with docs/media prefix (e.g. "docs/projects/file.md")
	FullPath   string   // Full filesystem path
	IsAbsolute bool     // Whether the original path was absolute
}

// parsePath analyzes any path and returns comprehensive path information
func parsePath(inputPath string) *PathInfo {
	if inputPath == "" {
		return &PathInfo{}
	}

	// check if path is absolute
	isAbsolute := filepath.IsAbs(inputPath)

	// normalize separators to forward slashes for consistent processing
	normalizedPath := filepath.ToSlash(inputPath)

	var pathType PathType
	var relativePath string
	var withPrefix string

	// determine type based on prefix
	if strings.HasPrefix(normalizedPath, "media/") {
		pathType = TypeMedia
		relativePath = strings.TrimPrefix(normalizedPath, "media/")
		withPrefix = normalizedPath
	} else if strings.HasPrefix(normalizedPath, "docs/") {
		pathType = TypeDocs
		relativePath = strings.TrimPrefix(normalizedPath, "docs/")
		withPrefix = normalizedPath
	} else {
		// no prefix - default to docs type
		pathType = TypeDocs
		relativePath = stripDataPathPrefix(normalizedPath)
		withPrefix = "docs/" + relativePath
	}

	// clean up relative path
	relativePath = strings.Trim(relativePath, "/")

	// calculate full filesystem path
	var fullPath string
	if isAbsolute {
		fullPath = inputPath
	} else {
		if pathType == TypeMedia {
			fullPath = filepath.Join(getMediaPath(), relativePath)
		} else {
			fullPath = filepath.Join(getDocsPath(), relativePath)
		}
	}

	return &PathInfo{
		Type:       pathType,
		Relative:   relativePath,
		WithPrefix: withPrefix,
		FullPath:   fullPath,
		IsAbsolute: isAbsolute,
	}
}

// ToRelative strips any prefix and data path to return clean relative path
func ToRelative(path string) string {
	return parsePath(path).Relative
}

// ToFullPath returns the full filesystem path
func ToFullPath(path string) string {
	return parsePath(path).FullPath
}

// ToWithPrefix ensures the path has the correct docs/media prefix for metadata storage
func ToWithPrefix(path string) string {
	return parsePath(path).WithPrefix
}

// ToDocsPath converts any path to a full docs filesystem path
func ToDocsPath(path string) string {
	info := parsePath(path)
	return filepath.Join(getDocsPath(), info.Relative)
}

// ToMediaPath converts any path to a full media filesystem path
func ToMediaPath(path string) string {
	info := parsePath(path)
	return filepath.Join(getMediaPath(), info.Relative)
}

// IsMedia returns true if the path represents a media file
func IsMedia(path string) bool {
	return parsePath(path).Type == TypeMedia
}

// IsDocs returns true if the path represents a docs file
func IsDocs(path string) bool {
	return parsePath(path).Type == TypeDocs
}

// convertType converts a path from one type to another while preserving structure
func convertType(path string, targetType PathType) string {
	info := parsePath(path)

	if targetType == TypeMedia {
		return "media/" + info.Relative
	}
	return "docs/" + info.Relative
}

// normalizePath standardizes path separators and cleans the path
func normalizePath(path string) string {
	if path == "" {
		return path
	}

	// convert to forward slashes and clean
	normalized := filepath.ToSlash(filepath.Clean(path))

	// remove leading slash if present (keep paths relative)
	return strings.TrimPrefix(normalized, "/")
}

// stripDataPathPrefix removes data directory prefix if present
func stripDataPathPrefix(path string) string {
	dataPath := configmanager.GetAppConfig().DataPath
	dataPathName := filepath.Base(filepath.Clean(dataPath))

	// handle relative paths with data prefix
	if strings.HasPrefix(path, dataPathName+"/") {
		return strings.TrimPrefix(path, dataPathName+"/")
	}
	if strings.HasPrefix(path, dataPathName+"\\") {
		return strings.TrimPrefix(path, dataPathName+"\\")
	}

	return path
}

// getDocsPath returns the full path to docs directory
func getDocsPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "docs")
}

// getMediaPath returns the full path to media directory
func getMediaPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "media")
}
