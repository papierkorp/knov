package files

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
)

var parserRegistry *parser.Registry

func init() {
	parserRegistry = parser.NewRegistry()
	parserRegistry.Register(parser.NewMarkdownHandler())
	parserRegistry.Register(parser.NewDokuwikiHandler())
	parserRegistry.Register(parser.NewPlaintextHandler())
}

// GetParserRegistry returns the global file type registry
func GetParserRegistry() *parser.Registry {
	return parserRegistry
}

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

type FileContent struct {
	HTML string
	TOC  []TOCItem
}

// pathsToFiles converts file paths to File structs
func pathsToFiles(paths []string, prefix string) []File {
	var files []File
	for _, path := range paths {
		fileName := filepath.Base(path)

		// add prefix to distinguish media files
		fullPath := path
		if prefix != "" {
			fullPath = filepath.Join(prefix, path)
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

// GetAllFiles returns list of all files using contentStorage
func GetAllFiles() ([]File, error) {
	paths, err := contentStorage.ListFiles()
	if err != nil {
		logging.LogError("failed to list files: %v", err)
		return nil, err
	}
	return pathsToFiles(paths, ""), nil
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
	handler := parserRegistry.GetHandler(filePath)
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

	html, err := handler.Render(parsed, contentStorage.ToRelativePath(filePath))
	if err != nil {
		return nil, err
	}

	relativePath := contentStorage.ToRelativePath(filePath)
	processedContent := strings.ReplaceAll(string(html), "{{FILEPATH}}", relativePath)

	processedContent = InjectHeaderIDs(processedContent)
	toc := GenerateTOC(processedContent)

	return &FileContent{
		HTML: processedContent,
		TOC:  toc,
	}, nil
}
