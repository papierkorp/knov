package files

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// DefaultCollection is the Collection value for root-level files (no containing folder).
// Collection is always derived (see recomputeDerivedFields) — a file is never without one.
const DefaultCollection = "default"

// CollectionFromPath derives the collection name from a file path —
// the first path segment of the relative path, matching recomputeDerivedFields logic.
// Returns DefaultCollection for root-level files.
func CollectionFromPath(path string) string {
	relPath := pathutils.ToRelative(path)
	folderPath := pathutils.ToSlash(filepath.Dir(relPath))
	if folderPath == "." || folderPath == "" {
		return DefaultCollection
	}
	return strings.SplitN(folderPath, "/", 2)[0]
}

// FolderFromPath derives a file's containing folder path (all segments joined with "/"),
// matching the Folders metadata field computed by metaDataUpdate. Returns "" for root-level files.
func FolderFromPath(path string) string {
	relPath := pathutils.ToRelative(path)
	folderPath := pathutils.ToSlash(filepath.Dir(relPath))
	if folderPath == "." || folderPath == "" {
		return ""
	}
	return folderPath
}

// File represents a file in the system
type File struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Metadata   *Metadata `json:"metadata,omitempty"`
	FuzzyMatch bool      `json:"-"` // set by search when the result comes from fuzzy (trigram) matching rather than an exact match
}

type FileContent struct {
	HTML   string
	TOC    []parser.TOCItem
	Editor EditorType // resolved editor type, so view handlers can dispatch without re-sniffing
}

// pathsToFiles converts file paths to File structs
func pathsToFiles(paths []string, prefix string) []File {
	var files []File
	for _, path := range paths {
		fileName := filepath.Base(path)

		// add prefix to distinguish media files
		fullPath := path
		if prefix != "" {
			fullPath = pathutils.ToSlash(filepath.Join(prefix, path))
		}

		// get metadata if it exists
		metadata, _ := MetaDataGet(fullPath)

		file := File{
			Name:     fileName,
			Path:     fullPath,
			Metadata: metadata,
		}
		files = append(files, file)
	}
	return files
}

// ViewURL returns the correct browser URL for viewing this file
func (f File) ViewURL() string {
	return pathutils.ToFileURL(pathutils.ToRelative(f.Path))
}

// GetAllPhysicalFiles returns only files that exist on the filesystem
func GetAllPhysicalFiles() ([]File, error) {
	paths, err := contentStorage.ListFiles()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list files: %v", err)
		return nil, err
	}
	return pathsToFiles(paths, ""), nil
}

// GetAllFiles returns all files from the filesystem (docs only).
func GetAllFiles() ([]File, error) {
	return GetAllPhysicalFiles()
}

// GetAllMediaFiles returns list of all media files using contentStorage
func GetAllMediaFiles() ([]File, error) {
	paths, err := contentStorage.ListMediaFiles()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list media files: %v", err)
		return nil, err
	}

	files := pathsToFiles(paths, "media")
	logging.LogDebug(logging.KeyApp, "found %d media files", len(files))
	return files, nil
}

// GetFileContent converts file content to html based on detected type
func GetFileContent(filePath string) (*FileContent, error) {
	handler := parser.GetParserRegistry().GetHandler(filePath)
	if handler == nil {
		return nil, fmt.Errorf("no handler found for file: %s", filePath)
	}

	relativePath := pathutils.ToRelative(filePath)
	editor := ResolveEditor(pathutils.ToWithPrefix(relativePath))

	// a book is shown as its composed document (referenced bodies inlined), not its raw
	// entry list. the composed markdown has no source file, so it renders PathlessRender.
	// the file-view banner is added by the caller - see render.RenderBookViewPrefix.
	var content []byte
	renderPath := relativePath
	if editor == EditorTypeBook {
		composed, err := book.Compose(relativePath)
		if err != nil {
			return nil, err
		}
		content, renderPath = []byte(composed), parser.PathlessRender
	} else {
		var err error
		if content, err = contentStorage.ReadFile(filePath); err != nil {
			return nil, err
		}
	}

	parsed, err := handler.Parse(content)
	if err != nil {
		return nil, err
	}

	// editors with no inline section editing (a composed book renders pathless anyway,
	// but keep it in the set for clarity) get no per-heading/per-section edit buttons
	editableSections := !(editor == EditorTypeFilter || editor == EditorTypeTracker || editor == EditorTypeList ||
		editor == EditorTypeTodo || editor == EditorTypeIndex || editor == EditorTypeBook)

	html, err := handler.Render(parsed, renderPath, editableSections)
	if err != nil {
		return nil, err
	}
	processedContent := strings.ReplaceAll(string(html), "{{FILEPATH}}", relativePath)

	var toc []parser.TOCItem
	if handler.Name() == "markdown" {
		toc = parser.TOCFromMarkdown(strings.Split(string(content), "\n"))
	}

	return &FileContent{
		HTML:   processedContent,
		TOC:    toc,
		Editor: editor,
	}, nil
}

// IsHidden reports whether file is hidden by hide - built for one scope via
// configmanager.NewHideMatcher. This is the single place every hide setting (type, HidePaths,
// HideFilesByTag) is applied; a new hide-by-x setting only needs to be added here.
func IsHidden(file File, hide *configmanager.HideMatcher) bool {
	return isHiddenByType(file) ||
		isInHiddenFolder(file, hide) ||
		(file.Metadata != nil && slices.ContainsFunc(file.Metadata.Tags, hide.TagHidden))
}

// FilterByVisibility returns only files not hidden for scope - pass "" for feature areas without a
// per-scope override, or one of configmanager.HideScope* (see IsHidden).
func FilterByVisibility(files []File, scope string) []File {
	hide := configmanager.NewHideMatcher(scope)
	var filtered []File
	for _, file := range files {
		if !IsHidden(file, hide) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// isHiddenByType returns true if the file should be excluded from listings based on its type.
// For media paths the mime type (derived from extension) is used.
// For docs paths the metadata Editor field is used.
func isHiddenByType(file File) bool {
	ext := strings.ToLower(filepath.Ext(file.Path))
	mime := configmanager.MimeTypeByExtension(ext)

	// check by mime (image, video, pdf — reliable on all platforms)
	if configmanager.IsHiddenByMime(mime) {
		return true
	}

	// check by extension (office, archives, executables, scripts — mime unreliable on Linux)
	if configmanager.IsHiddenByExt(ext) {
		return true
	}

	// text-based files: use metadata editor type
	if file.Metadata != nil && file.Metadata.Editor != "" {
		return configmanager.IsFileTypeHidden(string(file.Metadata.Editor))
	}

	return false
}

// isInHiddenFolder returns true if the file's containing folder path matches a configured
// hide-path pattern.
func isInHiddenFolder(file File, hide *configmanager.HideMatcher) bool {
	rel := pathutils.ToRelative(file.Path)
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return false
	}
	return hide.PathHidden(strings.Join(parts[:len(parts)-1], "/"))
}

// TreeNode represents a node in the file tree (either a directory or a file)
type TreeNode struct {
	Name     string
	Path     string // relative path, only set for file nodes
	IsDir    bool
	Metadata *Metadata // only set for file nodes, carried over from the source File
	Children []*TreeNode
}

// BuildFileTree constructs a sorted directory tree from a flat file list
func BuildFileTree(allFiles []File) *TreeNode {
	root := &TreeNode{IsDir: true}
	for _, file := range allFiles {
		rel := pathutils.ToRelative(file.Path)
		parts := strings.Split(rel, "/")
		insertTreeNode(root, parts, rel, file.Metadata)
	}
	sortTreeNode(root)
	return root
}

func insertTreeNode(parent *TreeNode, parts []string, filePath string, metadata *Metadata) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		parent.Children = append(parent.Children, &TreeNode{Name: parts[0], Path: filePath, Metadata: metadata})
		return
	}
	for _, child := range parent.Children {
		if child.IsDir && child.Name == parts[0] {
			insertTreeNode(child, parts[1:], filePath, metadata)
			return
		}
	}
	dir := &TreeNode{Name: parts[0], IsDir: true}
	parent.Children = append(parent.Children, dir)
	insertTreeNode(dir, parts[1:], filePath, metadata)
}

func sortTreeNode(node *TreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		sortTreeNode(child)
	}
}
