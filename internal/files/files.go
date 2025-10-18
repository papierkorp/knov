package files

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/filetype"
	"knov/internal/logging"
	"knov/internal/utils"
)

var fileTypeRegistry *filetype.Registry

func init() {
	fileTypeRegistry = filetype.NewRegistry()
	fileTypeRegistry.Register(filetype.NewMarkdownHandler())
	fileTypeRegistry.Register(filetype.NewDokuwikiHandler())
	fileTypeRegistry.Register(filetype.NewPlaintextHandler())
}

// GetFileTypeRegistry returns the global file type registry
func GetFileTypeRegistry() *filetype.Registry {
	return fileTypeRegistry
}

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

type FileContent struct {
	HTML template.HTML
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
	handler := fileTypeRegistry.GetHandler(filePath)
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
		HTML: template.HTML(processedContent),
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
		fileMetadata, err := MetaDataGet(file.Path)

		if err != nil {
			logging.LogWarning("failed to get metadata for %s: %v", file.Path, err)
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

	if logic == "AND" {
		for _, criterion := range criteria {
			if !matchesCriterion(metadata, criterion) {
				return false
			}
		}
		return true
	}

	for _, criterion := range criteria {
		if matchesCriterion(metadata, criterion) {
			return true
		}
	}
	return false
}

func matchesCriterion(metadata *Metadata, criterion FilterCriteria) bool {
	var fieldValue string

	switch criterion.Metadata {
	case "name":
		fieldValue = metadata.Name
	case "collection":
		fieldValue = metadata.Collection
	case "tags":
		return matchesTagCriterion(metadata.Tags, criterion)
	case "folders":
		return matchesArrayCriterion(metadata.Folders, criterion)
	case "boards":
		return matchesArrayCriterion(metadata.Boards, criterion)
	case "parents":
		return matchesArrayCriterion(metadata.Parents, criterion)
	case "kids":
		return matchesArrayCriterion(metadata.Kids, criterion)
	case "usedlinks":
		return matchesArrayCriterion(metadata.UsedLinks, criterion)
	case "linkstohere":
		return matchesArrayCriterion(metadata.LinksToHere, criterion)
	case "priority":
		fieldValue = string(metadata.Priority)
	case "status":
		fieldValue = string(metadata.Status)
	case "type":
		fieldValue = string(metadata.FileType)
	case "createdAt":
		if !metadata.CreatedAt.IsZero() {
			fieldValue = metadata.CreatedAt.Format(time.RFC3339)
		}
	case "lastEdited":
		if !metadata.LastEdited.IsZero() {
			fieldValue = metadata.LastEdited.Format(time.RFC3339)
		}
	default:
		return false
	}

	return matchesOperator(fieldValue, criterion.Operator, criterion.Value)
}

func matchesTagCriterion(tags []string, criterion FilterCriteria) bool {
	switch criterion.Operator {
	case "contains":
		return slices.Contains(tags, criterion.Value)
	case "not_contains":
		return !slices.Contains(tags, criterion.Value)
	case "empty":
		return len(tags) == 0
	case "not_empty":
		return len(tags) > 0
	default:
		return false
	}
}

func matchesArrayCriterion(array []string, criterion FilterCriteria) bool {
	switch criterion.Operator {
	case "contains":
		return slices.Contains(array, criterion.Value)
	case "not_contains":
		return !slices.Contains(array, criterion.Value)
	case "empty":
		return len(array) == 0
	case "not_empty":
		return len(array) > 0
	default:
		return false
	}
}

func matchesOperator(fieldValue, operator, criterionValue string) bool {
	switch operator {
	case "equals":
		return fieldValue == criterionValue
	case "not_equals":
		return fieldValue != criterionValue
	case "contains":
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(criterionValue))
	case "not_contains":
		return !strings.Contains(strings.ToLower(fieldValue), strings.ToLower(criterionValue))
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(fieldValue), strings.ToLower(criterionValue))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(fieldValue), strings.ToLower(criterionValue))
	case "empty":
		return fieldValue == ""
	case "not_empty":
		return fieldValue != ""
	case "greater_than":
		return fieldValue > criterionValue
	case "less_than":
		return fieldValue < criterionValue
	default:
		return false
	}
}

// GetRawContent returns raw file content as string
func GetRawContent(filePath string) (string, error) {
	handler := fileTypeRegistry.GetHandler(filePath)
	if handler == nil {
		return "", fmt.Errorf("no handler found for file: %s", filePath)
	}

	content, err := handler.GetContent(filePath)
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
