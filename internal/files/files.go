// Package files handles file operations and metadata
package files

import (
	"os"
	"path/filepath"
	"slices"
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

// FilterCriteria represents a single filter condition
type FilterCriteria struct {
	Metadata string `json:"metadata"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Logic    string `json:"logic"`
	Action   string `json:"action"`
}

// GetAllFiles returns list of all files
func GetAllFiles() ([]File, error) {
	dataDir := configmanager.DataPath
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

// ------------------------- FILTER -------------------------

// FilterFilesByMetadata filters files based on metadata criteria
func FilterFilesByMetadata(criteria []FilterCriteria) ([]File, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	if len(criteria) == 0 {
		return allFiles, nil
	}

	var filteredFiles []File
	for _, file := range allFiles {
		fileMetadata, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogWarning("failed to get metadata for %s: %v", file.Path, err)
			continue
		}
		if fileMetadata == nil {
			continue
		}

		if matchesFilter(fileMetadata, criteria) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return filteredFiles, nil
}

func matchesFilter(metadata *Metadata, criteria []FilterCriteria) bool {
	if len(criteria) == 0 {
		return true
	}

	result := evaluateFilter(metadata, criteria[0])

	for i := 1; i < len(criteria); i++ {
		currentResult := evaluateFilter(metadata, criteria[i])

		if criteria[i-1].Logic == "or" {
			result = result || currentResult
		} else {
			result = result && currentResult
		}
	}

	return result
}

func evaluateFilter(metadata *Metadata, filter FilterCriteria) bool {
	var fieldValue string
	var fieldArray []string

	filterValue := strings.TrimSpace(filter.Value)

	switch filter.Metadata {
	case "project":
		fieldValue = metadata.Project
	case "type":
		fieldValue = string(metadata.FileType)
	case "status":
		fieldValue = string(metadata.Status)
	case "priority":
		fieldValue = string(metadata.Priority)
	case "tags":
		fieldArray = metadata.Tags
	case "folders":
		fieldArray = metadata.Folders
	case "boards":
		fieldArray = metadata.Boards
	case "createdAt":
		fieldValue = metadata.CreatedAt.Format("2006-01-02")
	case "lastEdited":
		fieldValue = metadata.LastEdited.Format("2006-01-02")
	default:
		return false
	}

	var matches bool
	switch filter.Operator {
	case "equals":
		if len(fieldArray) > 0 {
			matches = slices.Contains(fieldArray, filterValue)
		} else {
			matches = fieldValue == filterValue
		}
	case "contains":
		if len(fieldArray) > 0 {
			matches = slices.ContainsFunc(fieldArray, func(s string) bool {
				return strings.Contains(strings.ToLower(s), strings.ToLower(filterValue))
			})
		} else {
			matches = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(filterValue))
		}
	case "in":
		if len(fieldArray) > 0 {
			matches = slices.Contains(fieldArray, filterValue)
		} else {
			matches = fieldValue == filterValue
		}
	default:
		matches = false
	}

	if filter.Action == "exclude" {
		return !matches
	}
	return matches
}
