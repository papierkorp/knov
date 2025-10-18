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
	return filepath.Join(configmanager.GetAppConfig().DataPath, relativePath)
}

// ToRelativePath removes data directory prefix to get relative path
// Input: "projects/file.md"
// Output: "/full/path/to/data/projects/file.md"
func ToRelativePath(fullPath string) string {
	dataPrefix := configmanager.GetAppConfig().DataPath + "/"
	return strings.TrimPrefix(fullPath, dataPrefix)
}
