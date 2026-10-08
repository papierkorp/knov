// Package pathutils provides centralized filepath conversion and normalization operations
package pathutils

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
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

	// the prefixes stripped below must match configmanager.ReservedDocsFolders
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

// DocsRoot returns the full path to the docs directory - exported for callers outside this
// package (e.g. the backup package's docs/media storage adapters) that need the directory itself
// rather than a path resolved within it.
func DocsRoot() string { return getDocsPath() }

// MediaRoot returns the full path to the media directory - see DocsRoot.
func MediaRoot() string { return getMediaPath() }

// ResolveRelativeLink resolves a decoded "./" or "../" link path written in the doc docPath against
// that doc's folder to a docs-root path, the way bare links are read ("../b.md" in "docs/a/x/n.md"
// -> "a/b.md"), bare "." and ".." too, read as the folder "./" and "../" - one climbing above the docs root stops there, like a url,
// and one to the docs root itself is "/". Other link paths are returned unchanged.
func ResolveRelativeLink(docPath, link string) string {
	if link != "." && link != ".." && !strings.HasPrefix(link, "./") && !strings.HasPrefix(link, "../") {
		return link
	}
	resolved := strings.TrimPrefix(path.Join("/", path.Dir(ToRelative(docPath)), link), "/")
	if resolved == "" {
		return "/"
	}
	if strings.HasSuffix(link, "/") || link == "." || link == ".." {
		resolved += "/"
	}
	return resolved
}

// RelativeLink is the inverse of ResolveRelativeLink: the "./" or "../" link path from the folder of
// the doc docPath to target ("docs/a/b.md" from "docs/a/x/n.md" -> "../b.md"), a trailing "/" kept, "/" is the docs root. a
// media/ target gets the link to its path without the prefix, read as media like any relative link.
func RelativeLink(docPath, target string) string {
	if target == "/" {
		if n := strings.Count(ToRelative(docPath), "/"); n > 0 {
			return strings.Repeat("../", n)
		}
		return "./"
	}
	if strings.HasSuffix(target, "/") {
		return RelativeLink(docPath, strings.TrimSuffix(target, "/")) + "/"
	}
	from := strings.Split(path.Dir(ToRelative(docPath)), "/")
	if from[0] == "." {
		from = nil
	}
	to := strings.Split(ToRelative(target), "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	if i == len(from) {
		return "./" + strings.Join(to[i:], "/")
	}
	return strings.Repeat("../", len(from)-i) + strings.Join(to[i:], "/")
}

// ToSlash converts path separators to forward slashes, without any of the docs/media
// normalization the To* functions above do. Use this over filepath.ToSlash for any path
// that will be compared against or stored as a forward-slash path (git tree paths, cache
// keys, URLs) - on Windows filepath.Rel/Join/Dir/Clean etc. all return backslash paths.
// Only for real filesystem paths of the host OS (a "\" stays on linux, where it's a valid
// filename char) - for path text from content/settings/input use crosspath.ToSlash.
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

// BaseWithoutExt returns the filename component of path with its extension stripped
// (e.g. "docs/notes.md" -> "notes"). Handles both "/" and "\" separators regardless
// of host OS, unlike filepath.Base/Ext which only split on the OS's own separator.
func BaseWithoutExt(path string) string {
	base := path
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		base = path[i+1:]
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// FolderContains reports whether dirPath is folderPath itself or a subfolder of it
// (recursive folder-path matching) — shared by kanban board scoping and
// auto-create-tag folder scoping.
func FolderContains(dirPath, folderPath string) bool {
	return dirPath == folderPath || strings.HasPrefix(dirPath, folderPath+"/")
}

// ErrReservedPath is returned when a new docs file or folder would land in a reserved top-level
// folder (see CheckTarget).
var ErrReservedPath = errors.New("target is in a reserved top-level folder")

// ErrInvalidName is returned when a new file or folder name breaks the filename policy (see
// CheckTarget).
var ErrInvalidName = errors.New("name contains # ? | [ ] \\ or a leading/trailing space")

// CheckTarget checks creating a file or folder at the host path newFull, or moving oldFull there
// (oldFull "" for a new one):
//   - ErrReservedPath for a docs path in a top-level folder parsePath reads as a prefix (see
//     configmanager.ReservedDocsFolders) - such a file can't be resolved back to itself
//   - ErrInvalidName for a name holding # ? | [ ] \ or starting/ending with a space - these break or
//     need encoding in links, so the app never creates them
//   - existing paths (git sync, manual copy) are left alone, the link codec still reads them
//   - a move keeping its name ("a#b.md" into another folder) only checks the new folders, any
//     rename has to fix the name
//   - the docs and media roots are never checked
func CheckTarget(oldFull, newFull string) error {
	_, statErr := os.Stat(newFull)
	if docsRoot := getDocsPath(); statErr != nil && PathContains(docsRoot, newFull) {
		rel, _ := filepath.Rel(docsRoot, newFull)
		if first, _, _ := strings.Cut(filepath.ToSlash(rel), "/"); slices.Contains(configmanager.ReservedDocsFolders(), first) {
			return ErrReservedPath
		}
	}
	if oldFull != "" && filepath.Base(oldFull) == filepath.Base(newFull) {
		newFull = filepath.Dir(newFull)
	}
	for p := filepath.Clean(newFull); ; p = filepath.Dir(p) {
		if _, err := os.Stat(p); err == nil || p == filepath.Dir(p) || p == getDocsPath() || p == getMediaPath() {
			return nil
		}
		if invalidName(filepath.Base(p)) {
			return ErrInvalidName
		}
	}
}

// CheckNewDocsPath is CheckTarget for creating the docs path p ("a/b.md" or "docs/a/b.md").
func CheckNewDocsPath(p string) error { return CheckTarget("", ToDocsPath(p)) }

// CleanName replaces the chars of name that break the filename policy (see CheckTarget) with "_"
// and trims its spaces ("a#b" -> "a_b").
func CleanName(name string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`#?|[]\`, r) {
			return '_'
		}
		return r
	}, name))
}

// invalidName reports whether a file or folder name breaks the filename policy, see CheckTarget.
func invalidName(name string) bool {
	return CleanName(name) != name
}

// PathContains reports whether candidate is root itself or strictly beneath it on the
// filesystem, checking real OS paths (filepath.Clean + filepath.Separator) rather than
// forward-slash logical paths - use this over FolderContains for actual filesystem
// directories so it works correctly on both Linux and Windows.
func PathContains(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	return candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator))
}

// escapeRelPath path-escapes each segment of a relative path and joins them with "/",
// so spaces, Unicode, and special characters survive as a URL path.
func escapeRelPath(rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// ToFileURL returns a browser-safe URL for viewing a file.
func ToFileURL(rel string) string { return "/files/" + escapeRelPath(rel) }

// ToFileEditURL returns a browser-safe URL for editing a file.
func ToFileEditURL(rel string) string { return "/files/edit/" + escapeRelPath(rel) }

// ToMediaURL returns a browser-safe URL for viewing a media file.
func ToMediaURL(rel string) string { return "/media/" + escapeRelPath(rel) }

// ToFileEditTableURL returns a browser-safe URL for editing a file's table.
func ToFileEditTableURL(rel string) string { return "/files/edittable/" + escapeRelPath(rel) }

// ToFileHistoryURL returns a browser-safe URL for viewing a file's history.
func ToFileHistoryURL(rel string) string { return "/files/history/" + escapeRelPath(rel) }

// ToRouteURL returns a browser-safe URL for a route that takes a path after its prefix
// (route ends with "/"), e.g. ToRouteURL("/api/files/delete/", rel) - for api routes, the
// /files/ and /media/ page urls have their own To*URL.
func ToRouteURL(route, rel string) string { return route + escapeRelPath(rel) }

// FileFromURL returns the docs-relative path of the file a page URL shows (the reverse of
// ToFileURL / ToFileEditURL / ToFileEditTableURL / ToFileHistoryURL), or "" for any other page
// or a path with a ".." segment.
func FileFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || strings.HasPrefix(u.Path, "/files/new/") {
		return ""
	}
	// ToFileURL("") is a prefix of the other routes, so it has to be checked last
	for _, prefix := range []string{ToFileEditURL(""), ToFileEditTableURL(""), ToFileHistoryURL(""), ToFileURL("")} {
		if rel, ok := strings.CutPrefix(u.Path, prefix); ok {
			// a ".." segment would point outside the docs folder
			if slices.Contains(strings.Split(rel, "/"), "..") {
				return ""
			}
			return rel
		}
	}
	return ""
}
