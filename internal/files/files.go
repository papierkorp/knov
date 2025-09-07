// Package files handles file operations and metadata
package files

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"knov/internal/configmanager"
	"knov/internal/logging"
)

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// GetAllFiles returns list of all files
func GetAllFiles() ([]File, error) {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	var files []File
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			file := File{
				Name: info.Name(),
				Path: path,
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

// GetFileContent converts markdown to html
func GetFileContent(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filePath, err)
		return nil, err
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	html := markdown.ToHTML(content, p, nil)

	return html, nil
}

// GetAllFilesWithMetadata returns files with metadata
func GetAllFilesWithMetadata() ([]File, error) {
	files, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	// TODO: populate metadata
	for i := range files {
		files[i].Metadata = nil
	}

	return files, nil
}
