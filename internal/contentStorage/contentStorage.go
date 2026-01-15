// Package contentStorage provides content storage functionality
package contentStorage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/utils"
)

// ContentStorage interface defines methods for content storage
type ContentStorage interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	DeleteFile(path string) error
	FileExists(path string) (bool, error)
	MkdirAll(path string, perm os.FileMode) error
	ListFiles() ([]string, error)
	GetDocsPath() string
	GetMediaPath() string
	GetGitPath() string
	GetBackendType() string
}

var storage ContentStorage

// Init initializes content storage with the specified provider
func Init() error {
	dataPath := configmanager.GetAppConfig().DataPath

	// for now, only filesystem provider is supported
	var err error
	storage, err = newFilesystemStorage(dataPath)
	if err != nil {
		return fmt.Errorf("failed to initialize content storage: %w", err)
	}

	logging.LogInfo("content storage initialized: filesystem")
	return nil
}

// ReadFile reads content from a file
func ReadFile(path string) ([]byte, error) {
	return storage.ReadFile(path)
}

// WriteFile writes content to a file
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return storage.WriteFile(path, data, perm)
}

// DeleteFile removes a file
func DeleteFile(path string) error {
	return storage.DeleteFile(path)
}

// FileExists checks if a file exists
func FileExists(path string) (bool, error) {
	return storage.FileExists(path)
}

// MkdirAll creates a directory path
func MkdirAll(path string, perm os.FileMode) error {
	return storage.MkdirAll(path, perm)
}

// ListFiles lists all files recursively
func ListFiles() ([]string, error) {
	return storage.ListFiles()
}

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}

// GetGitPath returns the full path to git directory
func GetGitPath() string {
	return storage.GetGitPath()
}

// GetDocsPath returns the full path to docs directory
func GetDocsPath() string {
	return storage.GetDocsPath()
}

// GetMediaPath returns the full path to media directory
func GetMediaPath() string {
	return storage.GetMediaPath()
}

// ToDocsPath converts relative path to full docs path
// Input: "projects/file.md" or "docs/projects/file.md"
// Output: "/full/path/to/data/docs/projects/file.md"
func ToDocsPath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}

	// normalize path and strip docs prefix if present
	normalizedPath := StripDocsPrefix(relativePath)

	return filepath.Join(GetDocsPath(), normalizedPath)
}

// ToMediaPath converts relative path to full media path
// Input: "images/photo.jpg" or "media/images/photo.jpg"
// Output: "/full/path/to/data/media/images/photo.jpg"
func ToMediaPath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}

	// normalize path and strip media prefix if present
	normalizedPath := StripMediaPrefix(relativePath)

	return filepath.Join(GetMediaPath(), normalizedPath)
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
		cleanPath := StripDocsPrefix(fullPath)
		if cleanPath != fullPath {
			return cleanPath
		}
		cleanPath = StripMediaPrefix(fullPath)
		if cleanPath != fullPath {
			return cleanPath
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
		cleanPath := utils.StripPathPrefix(relPath, "docs")
		if cleanPath != relPath {
			return cleanPath
		}
		cleanPath = utils.StripPathPrefix(relPath, "media")
		if cleanPath != relPath {
			return cleanPath
		}
		return relPath
	}

	// fallback: strip data path prefix if it exists
	dataPrefix := dataPath + string(filepath.Separator)
	if strings.HasPrefix(fullPath, dataPrefix) {
		stripped := strings.TrimPrefix(fullPath, dataPrefix)
		// also strip docs/ or media/ if present
		cleanPath := StripDocsPrefix(stripped)
		if cleanPath != stripped {
			return cleanPath
		}
		cleanPath = StripMediaPrefix(stripped)
		if cleanPath != stripped {
			return cleanPath
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

// StripDocsPrefix normalizes a docs path by stripping "docs/" prefix if present
func StripDocsPrefix(path string) string {
	return utils.StripPathPrefix(path, "docs")
}

// StripMediaPrefix normalizes a media path by stripping "media/" prefix if present
func StripMediaPrefix(path string) string {
	return utils.StripPathPrefix(path, "media")
}

// EnsureMetadataPrefix ensures the filepath has the correct prefix for metadata storage
// - paths with media/ prefix are left as-is
// - paths with docs/ prefix are left as-is
// - paths without either prefix get docs/ prefix added (assumes docs files)
func EnsureMetadataPrefix(gitPath string) string {
	// normalize path separators to forward slashes
	gitPath = filepath.ToSlash(gitPath)

	// if path already has media/ or docs/ prefix, return as-is
	if strings.HasPrefix(gitPath, "media/") || strings.HasPrefix(gitPath, "docs/") {
		return gitPath
	}

	// add docs/ prefix for all other paths
	return filepath.Join("docs", gitPath)
}
