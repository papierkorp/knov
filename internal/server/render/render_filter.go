// Package render - Basic filter HTML rendering functions
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/translation"
	"knov/internal/utils"
)

// ----------------------------------------------------------------------------------------
// ------------------------------------------ FORM ----------------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterForm renders a standalone filter form
func RenderFilterForm(config *filter.Config) string {
	return RenderFilterFormWithAction(config, "/api/filter", "", false)
}

// RenderFilterFormWithAction renders a filter form with custom action and options
func RenderFilterFormWithAction(config *filter.Config, action string, filePath string, includeFilePathInput bool) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<form id="filter-form" hx-post="%s" hx-target="#filter-results">`, action))

	// add filepath input for new files
	if includeFilePathInput {
		html.WriteString(`<div class="form-group">`)
		html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "file path") + `:</label>`)
		datalistInput := GenerateDatalistInput("filepath-input", "filepath", "", translation.SprintfForRequest(configmanager.GetLanguage(), "filters/my-filter"), "/api/files/folder-suggestions")
		// add required attribute
		datalistInput = strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1)
		html.WriteString(datalistInput)
		html.WriteString(`</div>`)
	} else if filePath != "" {
		// for editing, include filepath as hidden input
		html.WriteString(fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, filePath))
	}

	// controls
	html.WriteString(`<div class="filter-controls">`)
	html.WriteString(`<button type="submit" class="btn-primary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter") + `</button>`)
	html.WriteString(`<select name="logic" class="form-select">`)

	selectedLogic := "and"
	if config != nil {
		selectedLogic = config.Logic
	}

	html.WriteString(fmt.Sprintf(`<option value="and" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "and")+`</option>`, utils.Ternary(selectedLogic == "and", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="or" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "or")+`</option>`, utils.Ternary(selectedLogic == "or", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`<button type="button" hx-post="/api/filter/add-criteria" hx-target="#filter-criteria-container" hx-swap="beforeend" class="btn-secondary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "add filter") + `</button>`)
	html.WriteString(`</div>`)

	// display & limits section
	html.WriteString(`<div class="filter-controls">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "display") + `:</label>`)
	html.WriteString(`<select name="display" class="form-select">`)

	selectedDisplay := "list"
	if config != nil {
		selectedDisplay = config.Display
	}

	html.WriteString(fmt.Sprintf(`<option value="list" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "list")+`</option>`, utils.Ternary(selectedDisplay == "list", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="cards" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "cards")+`</option>`, utils.Ternary(selectedDisplay == "cards", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="dropdown" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "dropdown")+`</option>`, utils.Ternary(selectedDisplay == "dropdown", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="content" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "content")+`</option>`, utils.Ternary(selectedDisplay == "content", "selected", "")))
	html.WriteString(`</select>`)

	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "limit") + `:</label>`)
	limitValue := "50"
	if config != nil && config.Limit > 0 {
		limitValue = fmt.Sprintf("%d", config.Limit)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="limit" value="%s" min="1" class="form-input"/>`, limitValue))
	html.WriteString(`</div>`)

	// criteria
	html.WriteString(`<div id="filter-criteria-container" class="filter-criteria-container">`)
	if config != nil && len(config.Criteria) > 0 {
		for i, criteria := range config.Criteria {
			html.WriteString(RenderFilterCriteriaRow(i, &criteria))
		}
	} else {
		html.WriteString(RenderFilterCriteriaRow(0, nil))
	}
	html.WriteString(`</div>`)

	html.WriteString(`</form>`)
	return html.String()
}

// RenderFilterResult renders filter results based on display type
func RenderFilterResult(result *filter.Result, display string) string {
	if result == nil || len(result.Files) == 0 {
		return `<div id="filter-results" class="filter-no-results">
			<p>` + translation.SprintfForRequest(configmanager.GetLanguage(), "no files found matching filter criteria") + `</p>
		</div>`
	}

	switch display {
	case "cards":
		return fmt.Sprintf(`<div id="filter-results">%s</div>`, RenderFileCards(result.Files))
	case "dropdown":
		return RenderFileDropdown(result.Files, result.Total)
	case "content":
		return RenderFileContent(result.Files)
	case "table":
		return fmt.Sprintf(`<div id="filter-results">%s</div>`, RenderFileList(result.Files))
	default:
		return fmt.Sprintf(`<div id="filter-results">%s</div>`, RenderFileList(result.Files))
	}
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ single criteria ----------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterCriteriaRow renders a single filter criteria row
func RenderFilterCriteriaRow(index int, criteria *filter.Criteria) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="filter-criteria-row" data-index="%d">`, index))

	// metadata field select
	html.WriteString(`<div class="filter-field">`)
	if index > 0 {
		html.WriteString(`<hr />`)
	}
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "field") + `</label>`)
	html.WriteString(fmt.Sprintf(`<select name="metadata[%d]" class="form-select" hx-get="/api/filter/value-input" hx-target="#filter-value-container-%d" hx-swap="innerHTML" hx-vals='{"row_index": "%d"}' hx-include="this">`,
		index, index, index))
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select field") + `</option>`)

	selectedMetadata := ""
	if criteria != nil {
		selectedMetadata = criteria.Metadata
	}
	html.WriteString(RenderMetadataFieldOptions(selectedMetadata))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// operator select
	html.WriteString(`<div class="filter-field">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "operator") + `</label>`)
	html.WriteString(fmt.Sprintf(`<select name="operator[%d]" class="form-select">`, index))

	selectedOperator := "equals"
	if criteria != nil {
		selectedOperator = criteria.Operator
	}
	html.WriteString(RenderOperatorOptions(selectedOperator))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// value input
	html.WriteString(`<div class="filter-field">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "value") + `</label>`)
	html.WriteString(fmt.Sprintf(`<div id="filter-value-container-%d">`, index))

	value := ""
	metadataField := ""
	if criteria != nil {
		value = criteria.Value
		metadataField = criteria.Metadata
	}

	inputId := fmt.Sprintf("filter-value-%d", index)
	inputName := fmt.Sprintf("value[%d]", index)
	html.WriteString(RenderFilterValueInput(inputId, inputName, value, metadataField))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// action select
	html.WriteString(`<div class="filter-field">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "action") + `</label>`)
	html.WriteString(fmt.Sprintf(`<select name="action[%d]" class="form-select">`, index))

	selectedAction := "include"
	if criteria != nil {
		selectedAction = criteria.Action
	}
	html.WriteString(RenderActionOptions(selectedAction))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// remove button
	if index > 0 {
		html.WriteString(`<div class="filter-field">`)
		html.WriteString(`<button type="button" onclick="this.closest('.filter-criteria-row').remove()" class="btn-danger btn-small">` + translation.SprintfForRequest(configmanager.GetLanguage(), "remove") + `</button>`)
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderMetadataFieldOptions returns HTML options for metadata field selectors
func RenderMetadataFieldOptions(selectedValue string) string {
	var html strings.Builder

	fields := filter.GetMetadataFields()
	for _, field := range fields {
		selected := ""
		if field == selectedValue {
			selected = "selected"
		}
		displayText := field
		if strings.HasPrefix(field, "para_") {
			displayText = translation.SprintfForRequest(configmanager.GetLanguage(), "para") + ": " + strings.TrimPrefix(field, "para_")
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, field, selected, displayText))
	}

	return html.String()
}

// RenderOperatorOptions returns HTML options for operator selectors
func RenderOperatorOptions(selectedValue string) string {
	var html strings.Builder

	operators := filter.GetOperators()
	displayTexts := []string{
		translation.SprintfForRequest(configmanager.GetLanguage(), "equals"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "contains"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "regex"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "greater than"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "less than"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "in array"),
	}

	for i, operator := range operators {
		selected := ""
		if operator == selectedValue {
			selected = "selected"
		}
		displayText := operator
		if i < len(displayTexts) {
			displayText = displayTexts[i]
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, operator, selected, displayText))
	}

	return html.String()
}

// RenderActionOptions returns HTML options for action selectors
func RenderActionOptions(selectedValue string) string {
	var html strings.Builder

	actions := filter.GetActions()
	for _, action := range actions {
		selected := ""
		if action == selectedValue {
			selected = "selected"
		}
		displayText := translation.SprintfForRequest(configmanager.GetLanguage(), action)
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, action, selected, displayText))
	}

	return html.String()
}

// RenderFilterValueInput generates an input with datalist based on metadata field type
func RenderFilterValueInput(id, name, value, metadataField string) string {
	var apiEndpoint string
	var placeholder string

	switch metadataField {
	case "collection":
		apiEndpoint = "/api/metadata/collections?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select collection")
	case "tags":
		apiEndpoint = "/api/metadata/tags?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select tag")
	case "folders":
		apiEndpoint = "/api/metadata/folders?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select folder")
	case "type":
		apiEndpoint = "/api/metadata/filetypes?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "select file type")
	case "status":
		apiEndpoint = "/api/metadata/statuses?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "select status")
	case "priority":
		apiEndpoint = "/api/metadata/priorities?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "select priority")
	case "para_projects":
		apiEndpoint = "/api/metadata/para/projects?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select project")
	case "para_areas":
		apiEndpoint = "/api/metadata/para/areas?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select area")
	case "para_resources":
		apiEndpoint = "/api/metadata/para/resources?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select resource")
	case "para_archive":
		apiEndpoint = "/api/metadata/para/archive?format=options"
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "type or select archive item")
	case "createdAt", "lastEdited":
		return fmt.Sprintf(`<input type="date" name="%s" id="%s" value="%s" placeholder="%s" class="form-input"/>`,
			name, id, value, translation.SprintfForRequest(configmanager.GetLanguage(), "yyyy-mm-dd"))
	default:
		placeholder = translation.SprintfForRequest(configmanager.GetLanguage(), "enter value")
		return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" placeholder="%s"/>`,
			id, name, value, placeholder)
	}

	// use GenerateDatalistInput without save functionality
	return GenerateDatalistInput(id, name, value, placeholder, apiEndpoint)
}
