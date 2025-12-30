// Package utils provides path utilities
package utils

import (
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
)

// ToFullPath converts relative path to full filesystem path
// Input: "projects/file.md"
// Output: "/full/path/to/data/projects/file.md"
func ToFullPath(relativePath string) string {
	// if path is already absolute, return as-is
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(configmanager.GetAppConfig().DataPath, relativePath)
}

// ToRelativePath removes data directory prefix to get relative path
// Input: "/full/path/to/data/projects/file.md"
// Output: "projects/file.md"
func ToRelativePath(fullPath string) string {
	dataPath := configmanager.GetAppConfig().DataPath

	// if path is already relative, return as-is
	if !filepath.IsAbs(fullPath) {
		return fullPath
	}

	// clean both paths to normalize them (removes trailing slashes, resolves ./ and ../)
	fullPath = filepath.Clean(fullPath)
	dataPath = filepath.Clean(dataPath)

	// try to get relative path from data directory
	relPath, err := filepath.Rel(dataPath, fullPath)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		return relPath
	}

	// fallback: strip data path prefix if it exists
	dataPrefix := dataPath + string(filepath.Separator)
	if strings.HasPrefix(fullPath, dataPrefix) {
		return strings.TrimPrefix(fullPath, dataPrefix)
	}

	// last resort: if fullPath equals dataPath exactly, return empty
	if fullPath == dataPath {
		return ""
	}

	// if not under data path, return the full path as-is
	// this can happen if files are outside the data directory
	return fullPath
}
