// Package filter handles filtering operations across the application
package filter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/utils"
)

// Criteria represents a single filter condition
type Criteria struct {
	Metadata string `json:"metadata"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

// Config represents filter configuration
type Config struct {
	Criteria []Criteria `json:"criteria"`
	Logic    string     `json:"logic"`
	Display  string     `json:"display"` // list, cards, dropdown, content
	Limit    int        `json:"limit"`
}

// Result represents filter result with metadata
type Result struct {
	Files       []files.File `json:"files"`
	Total       int          `json:"total"`
	FilterCount int          `json:"filter_count"`
	Logic       string       `json:"logic"`
}

// FilterFiles filters files based on criteria
func FilterFiles(criteria []Criteria, logic string) ([]files.File, error) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	if len(criteria) == 0 {
		return allFiles, nil
	}

	var filteredFiles []files.File

	for _, file := range allFiles {
		fileMetadata, err := files.MetaDataGet(file.Path)
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
func FilterFilesWithConfig(config *Config) (*Result, error) {
	if config == nil {
		return nil, fmt.Errorf("filter config is required")
	}

	filteredFiles, err := FilterFiles(config.Criteria, config.Logic)
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

func matchesFilter(metadata *files.Metadata, criteria []Criteria, logic string) bool {
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

func matchesCriteria(metadata *files.Metadata, criterion Criteria) bool {
	var metadataValue string

	switch criterion.Metadata {
	case "name":
		metadataValue = metadata.Name
	case "collection":
		metadataValue = metadata.Collection
	case "tags":
		for _, tag := range metadata.Tags {
			if matchesOperator(tag, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "boards":
		for _, board := range metadata.Boards {
			if matchesOperator(board, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "type":
		metadataValue = string(metadata.FileType) // convert Filetype to string
	case "status":
		metadataValue = string(metadata.Status) // convert Status to string
	case "priority":
		metadataValue = string(metadata.Priority) // convert Priority to string
	case "createdAt":
		metadataValue = metadata.CreatedAt.Format("2006-01-02")
	case "lastEdited":
		metadataValue = metadata.LastEdited.Format("2006-01-02")
	case "folders":
		for _, folder := range metadata.Folders {
			if matchesOperator(folder, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "para_projects":
		for _, project := range metadata.PARA.Projects {
			if matchesOperator(project, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "para_areas":
		for _, area := range metadata.PARA.Areas {
			if matchesOperator(area, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "para_resources":
		for _, resource := range metadata.PARA.Resources {
			if matchesOperator(resource, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "para_archive":
		for _, archive := range metadata.PARA.Archive {
			if matchesOperator(archive, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	default:
		return false
	}

	return matchesOperator(metadataValue, criterion.Operator, criterion.Value)
}

func matchesOperator(metadataValue, operator, criteriaValue string) bool {
	switch operator {
	case "equals":
		return metadataValue == criteriaValue
	case "contains":
		return strings.Contains(strings.ToLower(metadataValue), strings.ToLower(criteriaValue))
	case "regex":
		matched, err := regexp.MatchString(criteriaValue, metadataValue)
		if err != nil {
			logging.LogWarning("invalid regex pattern: %s", criteriaValue)
			return false
		}
		return matched
	case "greater":
		// try date comparison first for date-like values
		if len(metadataValue) == 10 && len(criteriaValue) == 10 &&
			strings.Contains(metadataValue, "-") && strings.Contains(criteriaValue, "-") {
			metaDate, err1 := time.Parse("2006-01-02", metadataValue)
			criteriaDate, err2 := time.Parse("2006-01-02", criteriaValue)
			if err1 == nil && err2 == nil {
				return metaDate.After(criteriaDate)
			}
		}
		return metadataValue > criteriaValue
	case "less":
		// try date comparison first for date-like values
		if len(metadataValue) == 10 && len(criteriaValue) == 10 &&
			strings.Contains(metadataValue, "-") && strings.Contains(criteriaValue, "-") {
			metaDate, err1 := time.Parse("2006-01-02", metadataValue)
			criteriaDate, err2 := time.Parse("2006-01-02", criteriaValue)
			if err1 == nil && err2 == nil {
				return metaDate.Before(criteriaDate)
			}
		}
		return metadataValue < criteriaValue
	case "in":
		values := strings.Split(criteriaValue, ",")
		for _, value := range values {
			if strings.TrimSpace(value) == metadataValue {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// GetMetadataFields returns available metadata fields for filtering
func GetMetadataFields() []string {
	return []string{
		"name",
		"collection",
		"tags",
		"boards",
		"type",
		"status",
		"priority",
		"createdAt",
		"lastEdited",
		"folders",
		"para_projects",
		"para_areas",
		"para_resources",
		"para_archive",
	}
}

// GetOperators returns available filter operators
func GetOperators() []string {
	return []string{"equals", "contains", "regex", "greater", "less", "in"}
}

// GetActions returns available filter actions
func GetActions() []string {
	return []string{"include", "exclude"}
}

// ValidateConfig validates filter configuration
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Logic != "and" && config.Logic != "or" {
		return fmt.Errorf("logic must be 'and' or 'or'")
	}

	validFields := GetMetadataFields()
	validOperators := GetOperators()
	validActions := GetActions()

	for _, criteria := range config.Criteria {
		if !contains(validFields, criteria.Metadata) {
			return fmt.Errorf("invalid metadata field: %s", criteria.Metadata)
		}
		if !contains(validOperators, criteria.Operator) {
			return fmt.Errorf("invalid operator: %s", criteria.Operator)
		}
		if !contains(validActions, criteria.Action) {
			return fmt.Errorf("invalid action: %s", criteria.Action)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SaveFilterConfig validates and saves a filter configuration to file
func SaveFilterConfig(config *Config, filePath string) error {
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
	metadata := &files.Metadata{
		Path:     filePath,
		FileType: files.FileTypeFilter,
	}
	if err := files.MetaDataSave(metadata); err != nil {
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
