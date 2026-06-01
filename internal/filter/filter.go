// Package filter handles filtering operations across the application
package filter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
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

	allFiles = files.FilterByVisibility(allFiles)

	if len(criteria) == 0 {
		return allFiles, nil
	}

	var filteredFiles []files.File
	for _, file := range allFiles {
		if file.Metadata == nil { // already loaded by GetAllFiles
			continue
		}
		if matchesFilter(file.Metadata, criteria, logic) {
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
	case "title":
		metadataValue = metadata.Title
	case "collection":
		metadataValue = metadata.Collection
	case "tags":
		for _, tag := range metadata.Tags {
			if matchesOperator(tag, criterion.Operator, criterion.Value) {
				return true
			}
		}
		return false
	case "editor":
		metadataValue = string(metadata.Editor)
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
	case "child-of":
		for _, p := range metadata.Parents {
			if matchesOperator(pathutils.ToRelative(p), criterion.Operator, pathutils.ToRelative(criterion.Value)) {
				return true
			}
		}
		return false
	case "parent-of":
		for _, k := range metadata.Kids {
			if matchesOperator(pathutils.ToRelative(k), criterion.Operator, pathutils.ToRelative(criterion.Value)) {
				return true
			}
		}
		return false
	case "ancestor-of":
		for _, a := range metadata.Ancestor {
			if matchesOperator(pathutils.ToRelative(a), criterion.Operator, pathutils.ToRelative(criterion.Value)) {
				return true
			}
		}
		return false
	case "references":
		for _, ref := range metadata.References {
			if matchesOperator(ref.URL, criterion.Operator, criterion.Value) {
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
		"title",
		"collection",
		"tags",
		"editor",
		"createdAt",
		"lastEdited",
		"folders",
		"child-of",
		"parent-of",
		"ancestor-of",
		"references",
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
		if !utils.Contains(validFields, criteria.Metadata) {
			return fmt.Errorf("invalid metadata field: %s", criteria.Metadata)
		}
		if !utils.Contains(validOperators, criteria.Operator) {
			return fmt.Errorf("invalid operator: %s", criteria.Operator)
		}
		if !utils.Contains(validActions, criteria.Action) {
			return fmt.Errorf("invalid action: %s", criteria.Action)
		}
	}

	return nil
}

// filterKey returns the configStorage key for a filter ID
func filterKey(id string) string {
	return "filter/" + id
}

// filterIndexPath returns the docs-relative path of the index file paired with a filter.
// Respects KNOV_USE_EXTENSION_INDEX: returns e.g. "my/filter.index" or "my/filter.md".
func FilterIndexPath(filterID string) string {
	return filterID + configmanager.ExtensionForEditor("index")
}

// GenerateFilterIndex runs the filter and writes the results as a physical index file.
// The file is always overwritten so it stays in sync with the filter config.
func GenerateFilterIndex(filterID string, config *Config) error {
	result, err := FilterFilesWithConfig(config)
	if err != nil {
		return fmt.Errorf("failed to execute filter %s: %w", filterID, err)
	}

	// build markdown link list
	var sb strings.Builder
	for _, file := range result.Files {
		rel := pathutils.ToRelative(file.Path)
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", rel, rel))
	}

	indexPath := FilterIndexPath(filterID)
	fullPath := pathutils.ToDocsPath(indexPath)

	if err := contentStorage.WriteFile(fullPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write filter index %s: %w", indexPath, err)
	}

	// save metadata marking this as a filter-editor file so the filter editor opens
	metadata := &files.Metadata{
		Path:   pathutils.ToWithPrefix(indexPath),
		Editor: files.EditorTypeFilter,
	}
	if err := files.MetaDataSave(metadata); err != nil {
		logging.LogWarning("failed to save metadata for filter index %s: %v", indexPath, err)
	}

	logging.LogInfo("generated filter index: %s (%d files)", indexPath, len(result.Files))
	return nil
}

// SaveFilterConfig validates and saves a filter configuration to configStorage
func SaveFilterConfig(config *Config, filterID string) error {
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid filter config: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal filter config: %w", err)
	}

	if err := configStorage.Set(filterKey(filterID), data); err != nil {
		return fmt.Errorf("failed to save filter config: %w", err)
	}

	// generate the paired index file immediately
	if err := GenerateFilterIndex(filterID, config); err != nil {
		logging.LogWarning("failed to generate filter index for %s: %v", filterID, err)
	}

	logging.LogInfo("saved filter: %s", filterID)
	return nil
}

// GetFilterConfig loads a filter configuration from configStorage
func GetFilterConfig(filterID string) (*Config, error) {
	data, err := configStorage.Get(filterKey(filterID))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal filter config: %w", err)
	}
	return &config, nil
}

// GetAllFilters returns all filter IDs from configStorage
func GetAllFilters() ([]string, error) {
	keys, err := configStorage.List("filter/")
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = strings.TrimPrefix(k, "filter/")
	}
	return ids, nil
}

// DeleteFilterConfig removes a filter from configStorage and its paired index file.
func DeleteFilterConfig(filterID string) error {
	// delete the index file and its metadata
	indexPath := FilterIndexPath(filterID)
	fullPath := pathutils.ToDocsPath(indexPath)
	if err := contentStorage.DeleteFile(fullPath); err != nil {
		logging.LogWarning("failed to delete filter index file %s: %v", fullPath, err)
	}
	if err := files.MetaDataDelete(pathutils.ToWithPrefix(indexPath)); err != nil {
		logging.LogWarning("failed to delete filter index metadata %s: %v", indexPath, err)
	}

	return configStorage.Delete(filterKey(filterID))
}

// filterFieldName returns the form field name scoped to a widget, or standalone if widgetIndex < 0.
func filterFieldName(widgetIndex int, name string) string {
	if widgetIndex < 0 {
		return name
	}
	return fmt.Sprintf("widgets[%d][config][%s]", widgetIndex, name)
}

// ParseFilterConfigFromForm parses filter configuration from form data.
// Pass widgetIndex >= 0 to parse widget-namespaced fields, or -1 for standalone filter forms.
func ParseFilterConfigFromForm(r *http.Request, widgetIndex int) *Config {
	logic := r.FormValue(filterFieldName(widgetIndex, "logic"))
	if logic == "" {
		logic = "and"
	}
	display := r.FormValue(filterFieldName(widgetIndex, "display"))
	if display == "" {
		display = "list"
	}
	limitStr := r.FormValue(filterFieldName(widgetIndex, "limit"))
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	formData := make(map[int]map[string]string)

	if widgetIndex >= 0 {
		criteriaPrefix := fmt.Sprintf("widgets[%d][config][criteria]", widgetIndex)
		for key, values := range r.Form {
			if !strings.HasPrefix(key, criteriaPrefix) || len(values) == 0 {
				continue
			}
			rest := key[len(criteriaPrefix):]
			if !strings.HasPrefix(rest, "[") {
				continue
			}
			rest = rest[1:]
			end := strings.Index(rest, "]")
			if end < 0 {
				continue
			}
			rowIdx, err := strconv.Atoi(rest[:end])
			if err != nil {
				continue
			}
			fieldPart := rest[end+1:]
			if !strings.HasPrefix(fieldPart, "[") || !strings.HasSuffix(fieldPart, "]") {
				continue
			}
			field := fieldPart[1 : len(fieldPart)-1]
			if formData[rowIdx] == nil {
				formData[rowIdx] = make(map[string]string)
			}
			formData[rowIdx][field] = values[0]
		}
	} else {
		for key, values := range r.Form {
			if len(values) == 0 {
				continue
			}
			var field string
			var index int
			var err error
			if strings.HasPrefix(key, "metadata[") && strings.HasSuffix(key, "]") {
				field = "metadata"
				index, err = strconv.Atoi(key[9 : len(key)-1])
			} else if strings.HasPrefix(key, "operator[") && strings.HasSuffix(key, "]") {
				field = "operator"
				index, err = strconv.Atoi(key[9 : len(key)-1])
			} else if strings.HasPrefix(key, "value[") && strings.HasSuffix(key, "]") {
				field = "value"
				index, err = strconv.Atoi(key[6 : len(key)-1])
			} else if strings.HasPrefix(key, "action[") && strings.HasSuffix(key, "]") {
				field = "action"
				index, err = strconv.Atoi(key[7 : len(key)-1])
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

		// fallback to array form
		if len(formData) == 0 {
			metadata := r.Form["metadata[]"]
			operators := r.Form["operator[]"]
			values := r.Form["value[]"]
			actions := r.Form["action[]"]
			maxLen := len(metadata)
			for _, l := range []int{len(operators), len(values), len(actions)} {
				if l > maxLen {
					maxLen = l
				}
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
	}

	var criteria []Criteria
	for _, data := range formData {
		if data["metadata"] == "" || data["operator"] == "" || data["value"] == "" {
			continue
		}
		action := data["action"]
		if action == "" {
			action = "include"
		}
		criteria = append(criteria, Criteria{
			Metadata: data["metadata"],
			Operator: data["operator"],
			Value:    data["value"],
			Action:   action,
		})
	}

	logging.LogDebug("parsed %d filter criteria", len(criteria))
	return &Config{Criteria: criteria, Logic: logic, Display: display, Limit: limit}
}
