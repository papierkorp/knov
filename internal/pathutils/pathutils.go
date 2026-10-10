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

	// a docs-relative path is never read here - DocsPath prefixes it, so a folder named docs, media
	// or files keeps its name
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")

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

// MetaPath is the metadata path of a docs or media file or folder: always "docs/..." or
// "media/...", what File.Path holds and the metadata, search and filter keys are. Build it with
// DocsPath, MediaPath or ParseMeta - never convert a string to it, no function guesses its kind.
type MetaPath string

// DocsRel is a path relative to the docs folder ("projects/a.md", no "docs/" prefix, a folder
// called docs, media or files keeps its name). Build it with NewDocsRel or MetaPath.DocsRel.
type DocsRel string

// MediaPath is the metadata path of the media file at the media-relative path rel.
func MediaPath(rel string) MetaPath {
	return MetaPath("media/" + strings.TrimPrefix(rel, "/"))
}

// NewDocsRel is the docs-relative path rel taken literally (a leading "/" dropped) - for input
// that is a docs-relative path by definition: a /files/<rel> url, a docs listing, a file picker.
func NewDocsRel(rel string) DocsRel {
	return DocsRel(strings.TrimPrefix(rel, "/"))
}

// ParseMeta reads s as a metadata path - it has to be one already ("docs/..." or "media/...", see
// IsMetaPath), false otherwise. For input that carries a metadata path, like a query param.
func ParseMeta(s string) (MetaPath, bool) {
	if !IsMetaPath(s) {
		return "", false
	}
	return MetaPath(s), true
}

func (m MetaPath) String() string { return string(m) }

// Strings is paths as plain strings - to join or show them.
func Strings(paths []MetaPath) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = p.String()
	}
	return out
}

// IsMedia reports whether m is in the media folder.
func (m MetaPath) IsMedia() bool { return strings.HasPrefix(string(m), "media/") }

// DocsRel is m relative to the docs folder, false for a media path.
func (m MetaPath) DocsRel() (DocsRel, bool) {
	rel, ok := strings.CutPrefix(string(m), "docs/")
	return DocsRel(rel), ok
}

// MediaRel is m relative to the media folder, false for a docs path.
func (m MetaPath) MediaRel() (string, bool) {
	return strings.CutPrefix(string(m), "media/")
}

// Rel is m relative to its root folder, docs or media ("docs/a/b.md" -> "a/b.md").
func (m MetaPath) Rel() string { return metaRel(string(m)) }

func (r DocsRel) String() string { return string(r) }

// MetaPath is the docs file r.
func (r DocsRel) MetaPath() MetaPath { return DocsPath(string(r)) }

// DocsPath is the docs file or folder at the docs-relative path rel (user input, a /files/<rel>
// url, a docs listing), taken literally: "media/x.md" is data/docs/media/x.md, not a media file.
// A docs-relative path goes through it before any other function here - they read a leading
// docs/, media/ or files/ as prefix.
func DocsPath(rel string) MetaPath {
	return MetaPath("docs/" + strings.TrimPrefix(rel, "/"))
}

// IsMetaPath reports whether p is the metadata path of an existing file or folder: "docs/..." or
// "media/..." (what File.Path holds), without a "." or ".." segment - a path naming an existing
// file is never docs-relative guessed, see DocsPath for what a user types.
func IsMetaPath(p string) bool {
	if !strings.HasPrefix(p, "docs/") && !strings.HasPrefix(p, "media/") {
		return false
	}
	return !slices.ContainsFunc(strings.Split(p, "/"), func(seg string) bool { return seg == ".." || seg == "." })
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
// the single place every MetaPath.FullPath call resolves
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
	if !IsRelativeLink(link) {
		return link
	}
	resolved := strings.TrimPrefix(path.Join("/", path.Dir(metaRel(docPath)), link), "/")
	if resolved == "" {
		return "/"
	}
	if strings.HasSuffix(link, "/") || link == "." || link == ".." {
		resolved += "/"
	}
	return resolved
}

// IsRelativeLink reports whether a decoded link path is written relative to its doc's folder:
// "./", "../", "." or "..".
func IsRelativeLink(link string) bool {
	return link == "." || link == ".." || strings.HasPrefix(link, "./") || strings.HasPrefix(link, "../")
}

// LinkClimbsAboveRoot reports whether a "./" or "../" link path written in the doc docPath climbs
// above the docs root - ResolveRelativeLink stops it there, like a url.
func LinkClimbsAboveRoot(docPath, link string) bool {
	if !IsRelativeLink(link) {
		return false
	}
	depth := 0
	if dir := path.Dir(metaRel(docPath)); dir != "." {
		depth = strings.Count(dir, "/") + 1
	}
	for _, seg := range strings.Split(link, "/") {
		switch seg {
		case "..":
			if depth--; depth < 0 {
				return true
			}
		case ".", "":
		default:
			depth++
		}
	}
	return false
}

// RelativeLink is the inverse of ResolveRelativeLink: the "./" or "../" link path from the folder of
// the doc docPath to target ("docs/a/b.md" from "docs/a/x/n.md" -> "../b.md"), a trailing "/" kept, "/" and "docs/" are the docs root. a
// media/ target gets the link to its path without the prefix, read as media like any relative link.
func RelativeLink(docPath, target string) string {
	if target == "/" || target == "docs/" {
		if n := strings.Count(metaRel(docPath), "/"); n > 0 {
			return strings.Repeat("../", n)
		}
		return "./"
	}
	if strings.HasSuffix(target, "/") {
		return RelativeLink(docPath, strings.TrimSuffix(target, "/")) + "/"
	}
	from := strings.Split(path.Dir(metaRel(docPath)), "/")
	if from[0] == "." {
		from = nil
	}
	to := strings.Split(metaRel(target), "/")
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

// ErrInvalidName is returned when a new file or folder name breaks the filename policy (see
// CheckTarget).
var ErrInvalidName = errors.New("name contains # ? | [ ] \\ or a leading/trailing space")

// CheckTarget checks creating a file or folder at the host path newFull, or moving oldFull there
// (oldFull "" for a new one):
//   - ErrInvalidName for a name holding # ? | [ ] \ or starting/ending with a space - these break or
//     need encoding in links, so the app never creates them
//   - existing paths (git sync, manual copy) are left alone, the link codec still reads them
//   - a move keeping its name ("a#b.md" into another folder) only checks the new folders, any
//     rename has to fix the name
//   - the docs and media roots are never checked
func CheckTarget(oldFull, newFull string) error {
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

// CheckNewDocsPath is CheckTarget for creating the docs file p.
func CheckNewDocsPath(p MetaPath) error { return CheckTarget("", p.FullPath()) }

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

// escapeRelPath path-escapes each segment of a relative path (metadata path text, a "\" is a file name char) and joins them with "/",
// so spaces, Unicode, and special characters survive as a URL path.
func escapeRelPath(rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// docsURLRel is the part of a docs metadata path ("docs/a/b.md") after the docs/ prefix, which
// is how the /files/ routes name a docs file - taken literally, so "docs/media/x.md" is
// /files/media/x.md.
func docsURLRel(docsPath string) string {
	return strings.TrimPrefix(strings.TrimPrefix(docsPath, "/"), "docs/")
}

// ToFileURL returns a browser-safe URL for viewing a docs file (docsPath is its docs/ path).
func ToFileURL(docsPath MetaPath) string {
	return "/files/" + escapeRelPath(docsURLRel(docsPath.String()))
}

// ToFileEditURL returns a browser-safe URL for editing a docs file.
func ToFileEditURL(docsPath MetaPath) string {
	return "/files/edit/" + escapeRelPath(docsURLRel(docsPath.String()))
}

// ToMediaURL returns a browser-safe URL for viewing a media file.
func ToMediaURL(rel string) string { return "/media/" + escapeRelPath(rel) }

// ToFileEditTableURL returns a browser-safe URL for editing a file's table.
func ToFileEditTableURL(docsPath MetaPath) string {
	return "/files/edittable/" + escapeRelPath(docsURLRel(docsPath.String()))
}

// ToFileHistoryURL returns a browser-safe URL for viewing a file's history.
func ToFileHistoryURL(docsPath MetaPath) string {
	return "/files/history/" + escapeRelPath(docsURLRel(docsPath.String()))
}

// ToRouteURL returns a browser-safe URL for a route that takes a path after its prefix
// (route ends with "/"), e.g. ToRouteURL("/api/files/delete/", rel) - for api routes, the
// /files/ and /media/ page urls have their own To*URL.
func ToRouteURL(route, rel string) string { return route + escapeRelPath(rel) }

// FileFromURL returns the docs/ path of the file a page URL shows (the reverse of
// ToFileURL / ToFileEditURL / ToFileEditTableURL / ToFileHistoryURL), or "" for any other page
// or a path with a ".." segment.
func FileFromURL(rawURL string) MetaPath {
	u, err := url.Parse(rawURL)
	if err != nil || strings.HasPrefix(u.Path, "/files/new/") {
		return ""
	}
	// ToFileURL("") is a prefix of the other routes, so it has to be checked last
	for _, prefix := range []string{ToFileEditURL(""), ToFileEditTableURL(""), ToFileHistoryURL(""), ToFileURL("")} {
		if rel, ok := strings.CutPrefix(u.Path, prefix); ok {
			// a ".." segment would point outside the docs folder
			if rel == "" || slices.Contains(strings.Split(rel, "/"), "..") {
				return ""
			}
			return DocsPath(rel)
		}
	}
	return ""
}

// metaRel is the metadata path p without its docs/ or media/ prefix, taken literally - a "\" stays,
// as it is a valid file name char on linux and the path is no filesystem path of the host.
func metaRel(p string) string {
	p = strings.TrimPrefix(p, "/")
	for _, prefix := range []string{"docs/", "media/"} {
		if rel, ok := strings.CutPrefix(p, prefix); ok {
			return rel
		}
	}
	return p
}

// FromFullPath is the metadata path of a file or folder at the full filesystem path of the host
// (a walk result, a git path): the one place a host path becomes a MetaPath, separators converted.
// A path outside the docs and media folder is read as a docs path, like GuessMeta. Always
// forward-slash, even from a Windows full path ("C:\data\docs\a.md" -> "docs/a.md") - safe to
// compare against git tree paths, which are always forward-slash.
func FromFullPath(full string) MetaPath {
	return MetaPath(parsePath(full).WithPrefix)
}

// FullPath is the full filesystem path of m, exactly: no prefix is guessed.
func (m MetaPath) FullPath() string {
	root, rel := getDocsPath(), string(m)
	if r, ok := m.MediaRel(); ok {
		root, rel = getMediaPath(), r
	} else if r, ok := m.DocsRel(); ok {
		rel = string(r)
	}
	return containPath(root, filepath.Join(root, rel))
}

// GuessMeta reads s as a metadata path where its kind is not known: "docs/..." and "media/..." as
// they are, a data path prefix dropped, any other path as a docs path. Only for input that is
// entered by a user or another system (a form value, a typed link, a settings value) - a value that
// is known to be a docs-relative path goes through DocsPath, one that is known to be a metadata
// path through ParseMeta.
func GuessMeta(s string) MetaPath {
	return MetaPath(parsePath(s).WithPrefix)
}
