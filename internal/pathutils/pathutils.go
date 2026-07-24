// Package pathutils provides centralized filepath conversion and normalization operations
package pathutils

import (
	"net/url"
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

// parsePath analyzes any path and returns comprehensive path information.
// Returns empty PathInfo for external URLs (http:// / https://).
func parsePath(inputPath string) *PathInfo {
	if inputPath == "" {
		return &PathInfo{}
	}

	// external URLs are not managed paths
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		return &PathInfo{}
	}

	// check if path is absolute
	isAbsolute := filepath.IsAbs(inputPath)

	// normalize separators to forward slashes for consistent processing
	normalizedPath := filepath.ToSlash(inputPath)

	var pathType PathType
	var relativePath string
	var withPrefix string

	// strip leading slash and "files/" prefix used in stored metadata links
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	normalizedPath = strings.TrimPrefix(normalizedPath, "files/")

	// determine type based on prefix
	// strip absolute/data-path prefix first, then detect docs/media
	normalizedPath = stripDataPathPrefix(normalizedPath)

	prefix := "docs/"
	if strings.HasPrefix(normalizedPath, "media/") {
		pathType = TypeMedia
		prefix = "media/"
		relativePath = strings.TrimPrefix(normalizedPath, "media/")
	} else if strings.HasPrefix(normalizedPath, "docs/") {
		pathType = TypeDocs
		relativePath = strings.TrimPrefix(normalizedPath, "docs/")
	} else {
		pathType = TypeDocs
		relativePath = normalizedPath
	}
	withPrefix = prefix + relativePath

	// clean up relative path
	relativePath = strings.Trim(relativePath, "/")

	// calculate full filesystem path
	var fullPath string
	if isAbsolute {
		fullPath = inputPath
	} else {
		if pathType == TypeMedia {
			fullPath = containPath(getMediaPath(), filepath.Join(getMediaPath(), relativePath))
		} else {
			fullPath = containPath(getDocsPath(), filepath.Join(getDocsPath(), relativePath))
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

// ToWithPrefix ensures the path has the correct docs/media prefix for metadata storage.
// Always forward-slash, even from a Windows full path (e.g. "C:\data\docs\a.md" ->
// "docs/a.md") - safe to compare against git tree paths, which are always forward-slash.
func ToWithPrefix(path string) string {
	return parsePath(path).WithPrefix
}

// ToDocsPath converts any path to a full docs filesystem path
func ToDocsPath(path string) string {
	info := parsePath(path)
	docsRoot := getDocsPath()
	return containPath(docsRoot, filepath.Join(docsRoot, info.Relative))
}

// ToMediaPath converts any path to a full media filesystem path
func ToMediaPath(path string) string {
	info := parsePath(path)
	mediaRoot := getMediaPath()
	return containPath(mediaRoot, filepath.Join(mediaRoot, info.Relative))
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
	dataPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(configmanager.GetAppConfig().DataPath)), "/")
	normalizedPath := strings.TrimPrefix(filepath.ToSlash(path), "/")

	// strip full absolute data path (e.g. home/user/project/data/docs/file.md -> docs/file.md)
	if p, ok := strings.CutPrefix(normalizedPath, dataPath+"/"); ok {
		return p
	}

	// fallback: strip just the basename of data path (e.g. data/docs/file.md -> docs/file.md)
	dataPathName := filepath.Base(dataPath)
	if p, ok := strings.CutPrefix(normalizedPath, dataPathName+"/"); ok {
		return p
	}

	return path
}

// containPath returns candidate if it is root itself or strictly beneath it,
// otherwise root. A relativePath with enough "../" segments (e.g. from an
// unsanitized "?filepath=../../../etc/passwd" query param) can walk
// filepath.Join's result outside root entirely; clamping back to root here —
// the single place every ToDocsPath/ToMediaPath/ToFullPath call resolves
// through — means every caller is sandboxed without needing its own check.
func containPath(root, candidate string) string {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return candidate
	}
	return root
}

// getDocsPath returns the full path to docs directory
func getDocsPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "docs")
}

// getMediaPath returns the full path to media directory
func getMediaPath() string {
	return filepath.Join(configmanager.GetAppConfig().DataPath, "media")
}

// ToSlash converts path separators to forward slashes, without any of the docs/media
// normalization the To* functions above do. Use this over filepath.ToSlash for any path
// that will be compared against or stored as a forward-slash path (git tree paths, cache
// keys, URLs) - on Windows filepath.Rel/Join/Dir/Clean etc. all return backslash paths.
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

// FolderContains reports whether dirPath is folderPath itself or a subfolder of it
// (recursive folder-path matching) — shared by kanban board scoping and
// auto-create-tag folder scoping.
func FolderContains(dirPath, folderPath string) bool {
	return dirPath == folderPath || strings.HasPrefix(dirPath, folderPath+"/")
}

// ToFileURL returns a browser-safe URL for viewing a file.
// Segments are path-escaped so spaces, Unicode, and special characters work correctly.
func ToFileURL(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/files/" + strings.Join(parts, "/")
}

// ToMediaURL returns a browser-safe URL for viewing a media file.
func ToMediaURL(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/media/" + strings.Join(parts, "/")
}
