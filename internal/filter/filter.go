// Package filter handles filtering operations across the application
package filter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/logging"
	"knov/internal/types"
	"knov/internal/utils"
)

// MetadataGetter is an interface for getting file metadata
type MetadataGetter interface {
	GetMetadata(path string) (*types.Metadata, error)
}

// MetadataSaver is an interface for saving file metadata
type MetadataSaver interface {
	SaveMetadata(metadata *types.Metadata) error
}

// type alias for compatibility
type Criteria = types.Criteria

// Config represents filter configuration
type Config struct {
	Criteria []Criteria `json:"criteria"`
	Logic    string     `json:"logic"`
	Display  string     `json:"display"` // list, cards, dropdown, content
	Limit    int        `json:"limit"`
}

// Result represents filter result with metadata
type Result struct {
	Files       []types.File `json:"files"`
	Total       int          `json:"total"`
	FilterCount int          `json:"filter_count"`
	Logic       string       `json:"logic"`
}

// FilterFiles filters provided files based on criteria
func FilterFiles(allFiles []types.File, getter MetadataGetter, criteria []Criteria, logic string) ([]types.File, error) {
	if len(criteria) == 0 {
		return allFiles, nil
	}

	var filteredFiles []types.File

	for _, file := range allFiles {
		fileMetadata, err := getter.GetMetadata(file.Path)
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

// FilterFilesWithConfig filters files using config and returns result
func FilterFilesWithConfig(allFiles []types.File, getter MetadataGetter, config *Config) (*Result, error) {
	if config == nil {
		return nil, fmt.Errorf("filter config is required")
	}

	// validate configuration
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid filter configuration: %v", err)
	}

	filteredFiles, err := FilterFiles(allFiles, getter, config.Criteria, config.Logic)
	if err != nil {
		return nil, err
	}

	total := len(filteredFiles)

	// Apply limit
	if config.Limit > 0 && len(filteredFiles) > config.Limit {
		filteredFiles = filteredFiles[:config.Limit]
	}

	return &Result{
		Files:       filteredFiles,
		Total:       total,
		FilterCount: len(config.Criteria),
		Logic:       config.Logic,
	}, nil
}

func matchesFilter(metadata *types.Metadata, criteria []Criteria, logic string) bool {
	if len(criteria) == 0 {
		return true
	}

	var includeCriteria []Criteria
	var excludeCriteria []Criteria

	for _, criterion := range criteria {
		if criterion.Action == "exclude" {
			excludeCriteria = append(excludeCriteria, criterion)
		} else {
			includeCriteria = append(includeCriteria, criterion)
		}
	}

	// check exclude criteria first
	for _, criterion := range excludeCriteria {
		if matchesCriteria(metadata, criterion) {
			return false
		}
	}

	// if no include criteria, and passed exclude, include it
	if len(includeCriteria) == 0 {
		return true
	}

	// check include criteria
	if logic == "or" {
		for _, criterion := range includeCriteria {
			if matchesCriteria(metadata, criterion) {
				return true
			}
		}
		return false
	} else { // AND logic
		for _, criterion := range includeCriteria {
			if !matchesCriteria(metadata, criterion) {
				return false
			}
		}
		return true
	}
}

func matchesCriteria(metadata *types.Metadata, criterion Criteria) bool {
	// get field descriptor
	field, ok := types.GetFieldDescriptor(criterion.Metadata)
	if !ok {
		logging.LogWarning("unknown metadata field: %s", criterion.Metadata)
		return false
	}

	// get field value from metadata
	fieldValue := getMetadataFieldValue(metadata, field)
	if fieldValue == nil {
		return false
	}

	// get operator function
	opType := types.OperatorType(criterion.Operator)
	operatorFunc, err := GetOperator(opType)
	if err != nil {
		logging.LogWarning("unknown operator: %s", criterion.Operator)
		return false
	}

	// apply operator
	matches, err := operatorFunc(fieldValue, criterion.Value, field.Type)
	if err != nil {
		logging.LogWarning("error applying operator %s to field %s: %v", criterion.Operator, criterion.Metadata, err)
		return false
	}

	return matches
}

// getMetadataFieldValue extracts the field value from metadata based on field descriptor
func getMetadataFieldValue(metadata *types.Metadata, field *types.FieldDescriptor) interface{} {
	switch field.Name {
	case types.MetadataFields.Name.Name:
		return metadata.Name
	case types.MetadataFields.Path.Name:
		return metadata.Path
	case types.MetadataFields.Collection.Name:
		return metadata.Collection
	case types.MetadataFields.Tags.Name:
		return metadata.Tags
	case types.MetadataFields.Folders.Name:
		return metadata.Folders
	case types.MetadataFields.Boards.Name:
		return metadata.Boards
	case types.MetadataFields.CreatedAt.Name:
		return metadata.CreatedAt
	case types.MetadataFields.LastEdited.Name:
		return metadata.LastEdited
	case types.MetadataFields.TargetDate.Name:
		return metadata.TargetDate
	case types.MetadataFields.FileType.Name:
		return string(metadata.FileType)
	case types.MetadataFields.Status.Name:
		return string(metadata.Status)
	case types.MetadataFields.Priority.Name:
		return string(metadata.Priority)
	case types.MetadataFields.PARAProjects.Name:
		return metadata.PARA.Projects
	case types.MetadataFields.PARAreas.Name:
		return metadata.PARA.Areas
	case types.MetadataFields.PARAResources.Name:
		return metadata.PARA.Resources
	case types.MetadataFields.PARAArchive.Name:
		return metadata.PARA.Archive
	default:
		return nil
	}
}

// GetMetadataFields returns available metadata fields for filtering
func GetMetadataFields() []string {
	return types.AllFieldNames()
}

// GetOperators returns available filter operators
func GetOperators() []string {
	return []string{"equals", "contains", "regex", "greater", "less", "in"}
}

// GetActions returns available filter actions
func GetActions() []string {
	return []string{"include", "exclude"}
}

// SaveFilterConfig validates and saves a filter configuration to file
func SaveFilterConfig(saver MetadataSaver, config *Config, filePath string) error {
	// validate the configuration first
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid filter config: %w", err)
	}

	// ensure filepath has .filter extension
	if !strings.HasSuffix(filePath, ".filter") {
		filePath = filePath + ".filter"
	}

	// convert config to JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal filter config: %w", err)
	}

	fullPath := utils.ToFullPath(filePath)

	// create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// save JSON to file
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		logging.LogError("failed to save filter file %s: %v", fullPath, err)
		return fmt.Errorf("failed to save filter file: %w", err)
	}

	// create metadata for the filter file
	metadata := &types.Metadata{
		Path:     filePath,
		FileType: types.FileTypeFilter,
	}
	if err := saver.SaveMetadata(metadata); err != nil {
		logging.LogError("failed to save filter metadata: %v", err)
		// don't fail the request, just log the error since file was saved successfully
	}

	logging.LogInfo("saved filter file: %s", filePath)
	return nil
}

func ParseFilterConfigFromForm(r *http.Request) *Config {
	logic := r.FormValue("logic")
	if logic == "" {
		logic = "and"
	}

	display := r.FormValue("display")
	if display == "" {
		display = "list"
	}

	limitStr := r.FormValue("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	var criteria []Criteria

	// Extract all indexed form values
	formData := make(map[int]map[string]string)

	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}

		var field string
		var index int
		var err error

		if strings.HasPrefix(key, "metadata[") && strings.HasSuffix(key, "]") {
			field = "metadata"
			indexStr := key[9 : len(key)-1] // extract index from metadata[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "operator[") && strings.HasSuffix(key, "]") {
			field = "operator"
			indexStr := key[9 : len(key)-1] // extract index from operator[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "value[") && strings.HasSuffix(key, "]") {
			field = "value"
			indexStr := key[6 : len(key)-1] // extract index from value[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "action[") && strings.HasSuffix(key, "]") {
			field = "action"
			indexStr := key[7 : len(key)-1] // extract index from action[index]
			index, err = strconv.Atoi(indexStr)
		} else {
			continue
		}

		if err != nil {
			continue
		}

		if formData[index] == nil {
			formData[index] = make(map[string]string)
		}
		formData[index][field] = values[0]
	}

	// fallback to array form if no indexed data found
	if len(formData) == 0 {
		metadata := r.Form["metadata[]"]
		operators := r.Form["operator[]"]
		values := r.Form["value[]"]
		actions := r.Form["action[]"]

		maxLen := len(metadata)
		if len(operators) > maxLen {
			maxLen = len(operators)
		}
		if len(values) > maxLen {
			maxLen = len(values)
		}
		if len(actions) > maxLen {
			maxLen = len(actions)
		}

		for i := 0; i < maxLen; i++ {
			if formData[i] == nil {
				formData[i] = make(map[string]string)
			}

			if i < len(metadata) {
				formData[i]["metadata"] = metadata[i]
			}
			if i < len(operators) {
				formData[i]["operator"] = operators[i]
			} else {
				formData[i]["operator"] = "equals"
			}
			if i < len(values) {
				formData[i]["value"] = values[i]
			}
			if i < len(actions) {
				formData[i]["action"] = actions[i]
			} else {
				formData[i]["action"] = "include"
			}
		}
	}

	// build criteria from grouped form data
	for _, data := range formData {
		metadata := data["metadata"]
		operator := data["operator"]
		value := data["value"]
		action := data["action"]

		if metadata != "" && operator != "" && value != "" {
			if operator == "" {
				operator = "equals"
			}
			if action == "" {
				action = "include"
			}

			criteria = append(criteria, Criteria{
				Metadata: metadata,
				Operator: operator,
				Value:    value,
				Action:   action,
			})
		}
	}

	logging.LogDebug("parsed %d filter criteria", len(criteria))
	for i, c := range criteria {
		logging.LogDebug("criteria %d: %s %s %s (%s)", i, c.Metadata, c.Operator, c.Value, c.Action)
	}

	return &Config{
		Criteria: criteria,
		Logic:    logic,
		Display:  display,
		Limit:    limit,
	}
}
