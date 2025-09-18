// Package files handles file operations and metadata
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"knov/internal/configmanager"
	"knov/internal/content"
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
	dataDir := configmanager.GetAppConfig().DataPath
	var files []File
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
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

// GetFileContent converts markdown to html
func GetFileContent(filePath string) ([]byte, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filePath, err)
		return nil, err
	}

	processedContent := content.ProcessContent(string(fileContent))

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	html := markdown.ToHTML([]byte(processedContent), p, nil)

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

// FilterCriteria represents a single filter condition
type FilterCriteria struct {
	Metadata string `json:"metadata"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

// FilterFilesByMetadata filters files based on metadata criteria
func FilterFilesByMetadata(criteria []FilterCriteria, logic string) ([]File, error) {
	allFiles, err := GetAllFiles()

	if err != nil {
		return nil, err
	}

	if len(criteria) == 0 {
		return allFiles, nil
	}

	var filteredFiles []File

	for _, file := range allFiles {
		dataDir := configmanager.GetAppConfig().DataPath
		fullPath := filepath.Join(dataDir, file.Path)
		fileMetadata, err := MetaDataGet(fullPath)

		if err != nil {
			logging.LogWarning("failed to get metadata for %s: %v", fullPath, err)
			continue
		}

		if fileMetadata == nil {
			continue
		}

		if matchesFilter(fileMetadata, criteria, logic) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return filteredFiles, nil

}
func matchesFilter(metadata *Metadata, criteria []FilterCriteria, logic string) bool {
	if len(criteria) == 0 {
		return true
	}

	result := evaluateFilter(metadata, criteria[0])

	for i := 1; i < len(criteria); i++ {
		currentResult := evaluateFilter(metadata, criteria[i])
		if logic == "or" {
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

	switch filter.Metadata {
	case "collection":
		fieldValue = metadata.Collection
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
		logging.LogWarning("INTERNAL WARNING: selected metadata does not exist")
		return false
	}

	var matches bool
	switch filter.Operator {
	case "equals":
		if len(fieldArray) > 0 {
			matches = slices.Contains(fieldArray, filter.Value)
		} else {
			matches = fieldValue == filter.Value
		}
	case "contains":
		if len(fieldArray) > 0 {
			matches = slices.ContainsFunc(fieldArray, func(s string) bool {
				return strings.Contains(strings.ToLower(s), strings.ToLower(filter.Value))
			})
		} else {
			matches = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(filter.Value))
		}
	case "in":
		if len(fieldArray) > 0 {
			matches = slices.Contains(fieldArray, filter.Value)
		} else {
			matches = fieldValue == filter.Value
		}
	case "greater":
		if len(fieldArray) > 0 {
			arrayCount := len(fieldArray)
			if targetCount, err := strconv.Atoi(filter.Value); err == nil {
				matches = arrayCount > targetCount
			}
		} else {
			switch filter.Metadata {
			case "createdAt", "lastEdited":
				if targetDate, err := parseDate(filter.Value); err == nil {
					var compareDate time.Time
					if filter.Metadata == "createdAt" {
						compareDate = metadata.CreatedAt
					} else {
						compareDate = metadata.LastEdited
					}
					matches = compareDate.After(targetDate)
				}
			case "size":
				if targetSize, err := strconv.ParseInt(filter.Value, 10, 64); err == nil {
					matches = metadata.Size > targetSize
				}
			}
		}
	case "less":
		if len(fieldArray) > 0 {
			arrayCount := len(fieldArray)
			if targetCount, err := strconv.Atoi(filter.Value); err == nil {
				matches = arrayCount < targetCount
			}
		} else {
			switch filter.Metadata {
			case "createdAt", "lastEdited":
				if targetDate, err := parseDate(filter.Value); err == nil {
					var compareDate time.Time
					if filter.Metadata == "createdAt" {
						compareDate = metadata.CreatedAt
					} else {
						compareDate = metadata.LastEdited
					}
					matches = compareDate.Before(targetDate)
				}
			case "size":
				if targetSize, err := strconv.ParseInt(filter.Value, 10, 64); err == nil {
					matches = metadata.Size < targetSize
				}
			}
		}
	default:
		logging.LogWarning("INTERNAL WARNING: selected operator does not exist")
		matches = false
	}

	if filter.Action == "exclude" {
		return !matches
	}
	return matches
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02", // ISO format
		"02.01.2006", // DD.MM.YYYY
		"01/02/2006", // MM/DD/YYYY
		"2006/01/02", // YYYY/MM/DD
		"02-01-2006", // DD-MM-YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
