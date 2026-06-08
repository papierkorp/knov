package files

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// CollectionFromPath derives the collection name from a file path —
// the first path segment of the relative path, matching metaDataUpdate logic.
// Returns "" for root-level files.
func CollectionFromPath(path string) string {
	relPath := pathutils.ToRelative(path)
	folderPath := filepath.ToSlash(filepath.Dir(relPath))
	if folderPath == "." || folderPath == "" {
		return ""
	}
	return strings.SplitN(folderPath, "/", 2)[0]
}

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

type FileContent struct {
	HTML string
	TOC  []parser.TOCItem
}

// pathsToFiles converts file paths to File structs
func pathsToFiles(paths []string, prefix string) []File {
	var files []File
	for _, path := range paths {
		fileName := filepath.Base(path)

		// add prefix to distinguish media files
		fullPath := path
		if prefix != "" {
			fullPath = filepath.ToSlash(filepath.Join(prefix, path))
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
		logging.LogError("failed to list files: %v", err)
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
		logging.LogError("failed to list media files: %v", err)
		return nil, err
	}

	files := pathsToFiles(paths, "media")
	logging.LogDebug("found %d media files", len(files))
	return files, nil
}

// GetFileContent converts file content to html based on detected type
func GetFileContent(filePath string) (*FileContent, error) {
	handler := parser.GetParserRegistry().GetHandler(filePath)
	if handler == nil {
		return nil, fmt.Errorf("no handler found for file: %s", filePath)
	}

	// read file content directly using contentStorage
	content, err := contentStorage.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parsed, err := handler.Parse(content)
	if err != nil {
		return nil, err
	}

	html, err := handler.Render(parsed, pathutils.ToRelative(filePath))
	if err != nil {
		return nil, err
	}

	relativePath := pathutils.ToRelative(filePath)

	// strip section edit buttons for non-markdown editors
	if meta, err := MetaDataGet(pathutils.ToWithPrefix(relativePath)); err == nil && meta != nil {
		if meta.Editor != EditorTypeMarkdown && meta.Editor != EditorTypeTextarea && meta.Editor != "" {
			html = regexp.MustCompile(`<a href="/files/edit/[^"]*\?section=[^"]*" class="header-edit-btn"[^>]*>.*?</a>`).ReplaceAll(html, nil)
		}
	}
	processedContent := strings.ReplaceAll(string(html), "{{FILEPATH}}", relativePath)

	toc := parser.GenerateTOC(processedContent)

	return &FileContent{
		HTML: processedContent,
		TOC:  toc,
	}, nil
}

// FilterByVisibility returns only files that should be visible based on the current hide settings.
// Checks mime type, extension, and editor type in that order.
func FilterByVisibility(files []File) []File {
	var filtered []File
	for _, file := range files {
		if !isHiddenByType(file) {
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

// TreeNode represents a node in the file tree (either a directory or a file)
type TreeNode struct {
	Name     string
	Path     string // relative path, only set for file nodes
	IsDir    bool
	Children []*TreeNode
}

// BuildFileTree constructs a sorted directory tree from a flat file list
func BuildFileTree(allFiles []File) *TreeNode {
	root := &TreeNode{IsDir: true}
	for _, file := range allFiles {
		rel := pathutils.ToRelative(file.Path)
		parts := strings.Split(rel, "/")
		insertTreeNode(root, parts, rel)
	}
	sortTreeNode(root)
	return root
}

func insertTreeNode(parent *TreeNode, parts []string, filePath string) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		parent.Children = append(parent.Children, &TreeNode{Name: parts[0], Path: filePath})
		return
	}
	for _, child := range parent.Children {
		if child.IsDir && child.Name == parts[0] {
			insertTreeNode(child, parts[1:], filePath)
			return
		}
	}
	dir := &TreeNode{Name: parts[0], IsDir: true}
	parent.Children = append(parent.Children, dir)
	insertTreeNode(dir, parts[1:], filePath)
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
