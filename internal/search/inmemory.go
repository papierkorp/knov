// Package search provides different search engine implementations
package search

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/files"
	"knov/internal/utils"
)

// InMemoryEngine ..
type InMemoryEngine struct {
	index map[string]FileIndex
	mutex sync.RWMutex
}

// FileIndex ..
type FileIndex struct {
	Path, Title, Content, Tags string
}

// Initialize ..
func (m *InMemoryEngine) Initialize() error {
	return nil
}

// IndexAllFiles ..
func (m *InMemoryEngine) IndexAllFiles() error {
	m.index = make(map[string]FileIndex)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	allFiles, err := files.GetAllFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		fullPath := utils.ToFullPath(file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		metadata, _ := files.MetaDataGet(file.Path)
		tags := ""
		if metadata != nil && len(metadata.Tags) > 0 {
			tags = strings.Join(metadata.Tags, " ")
		}

		m.index[file.Path] = FileIndex{
			Path:    file.Path,
			Title:   file.Name,
			Content: string(content),
			Tags:    tags,
		}
	}

	return nil
}

// SearchFiles ..
func (m *InMemoryEngine) SearchFiles(query string, limit int) ([]files.File, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var results []files.File
	queryLower := strings.ToLower(query)

	for _, fileIndex := range m.index {
		if len(results) >= limit {
			break
		}

		if strings.Contains(strings.ToLower(fileIndex.Title), queryLower) ||
			strings.Contains(strings.ToLower(fileIndex.Content), queryLower) ||
			strings.Contains(strings.ToLower(fileIndex.Tags), queryLower) {

			results = append(results, files.File{
				Name: filepath.Base(fileIndex.Path),
				Path: fileIndex.Path,
			})
		}
	}
	return results, nil
}
