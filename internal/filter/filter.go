// Package filter handles filtering operations across the application
package filter

import (
	"fmt"
	"strings"

	"knov/internal/files"
	"knov/internal/logging"
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
	Display  string     `json:"display"` // list, cards, dropdown
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
	case "collection":
		metadataValue = metadata.Collection
	case "tags":
		for _, tag := range metadata.Tags {
			if matchesOperator(tag, criterion.Operator, criterion.Value) {
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
	case "greater":
		return metadataValue > criteriaValue
	case "less":
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
		"collection",
		"tags",
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
	return []string{"equals", "contains", "greater", "less", "in"}
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
