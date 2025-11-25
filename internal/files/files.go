package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/utils"
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

// GetAllFiles returns list of all files
func GetAllFiles() ([]File, error) {
	dataDir := configmanager.GetAppConfig().DataPath
	var files []File
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			relativePath, err := filepath.Rel(dataDir, path)
			if err != nil {
				return err
			}
			file := File{
				Name: info.Name(),
				Path: relativePath,
			}
			files = append(files, file)
		}
		return nil
	})

	if err != nil {
		logging.LogError("failed to walk directory: %v", err)
		return nil, err
	}
	return files, nil
}

// GetFileContent converts file content to html based on detected type
func GetFileContent(filePath string) (*FileContent, error) {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return nil, fmt.Errorf("no handler found for file: %s", filePath)
	}

	content, err := handler.GetContent(filePath)
	if err != nil {
		return nil, err
	}

	parsed, err := handler.Parse(content)
	if err != nil {
		return nil, err
	}

	html, err := handler.Render(parsed)
	if err != nil {
		return nil, err
	}

	relativePath := utils.ToRelativePath(filePath)
	processedContent := strings.ReplaceAll(string(html), "{{FILEPATH}}", relativePath)

	processedContent = InjectHeaderIDs(processedContent)
	toc := GenerateTOC(processedContent)

	return &FileContent{
		HTML: processedContent,
		TOC:  toc,
	}, nil
}

// GetAllFilesWithMetadata returns files with metadata
func GetAllFilesWithMetadata() ([]File, error) {
	files, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	for i := range files {
		files[i].Metadata = nil
	}

	return files, nil
}

// GetRawContent returns raw file content as string
func GetRawContent(filePath string) (string, error) {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return "", fmt.Errorf("no handler found for file: %s", filePath)
	}

	fullPath := utils.ToFullPath(filePath)
	content, err := handler.GetContent(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SaveRawContent saves raw content to file (creates if not exists)
func SaveRawContent(filePath string, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		return err
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		logging.LogError("failed to write file %s: %v", filePath, err)
		return err
	}

	logging.LogInfo("saved file: %s", filePath)
	return nil
}
