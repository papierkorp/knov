// Package render - HTMX HTML rendering functions for dashboard filter components
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/translation"
	"knov/internal/utils"
)

func renderFilterWidget(config *dashboard.FilterConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("filter config is required")
	}

	filteredFiles, err := files.FilterFilesByMetadata(config.Criteria, config.Logic)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		return "", err
	}

	limit := config.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(filteredFiles) > limit {
		filteredFiles = filteredFiles[:limit]
	}

	switch config.Display {
	case "cards":
		return RenderFileCards(filteredFiles), nil
	case "dropdown":
		return RenderFileDropdown(filteredFiles, 10), nil
	default:
		return RenderFileList(filteredFiles), nil
	}
}

// RenderMetadataFieldOptions returns HTML options for metadata field selectors
func RenderMetadataFieldOptions(selectedValue string) string {
	var html strings.Builder

	options := []string{"collection", "tags", "type", "status", "priority", "createdAt", "lastEdited", "folders", "boards", "para_projects", "para_areas", "para_resources", "para_archive"}

	for _, option := range options {
		selected := ""
		if option == selectedValue {
			selected = "selected"
		}
		displayText := option
		if strings.HasPrefix(option, "para_") {
			displayText = "para: " + strings.TrimPrefix(option, "para_")
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, displayText))
	}

	return html.String()
}

// RenderOperatorOptions returns HTML options for operator selectors
func RenderOperatorOptions(selectedValue string) string {
	var html strings.Builder

	options := []string{"equals", "contains", "greater", "less", "in"}
	displayTexts := []string{"equals", "contains", "greater than", "less than", "in array"}

	for i, option := range options {
		selected := ""
		if option == selectedValue {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, displayTexts[i]))
	}

	return html.String()
}

// RenderActionOptions returns HTML options for action selectors
func RenderActionOptions(selectedValue string) string {
	var html strings.Builder

	options := []string{"include", "exclude"}

	for _, option := range options {
		selected := ""
		if option == selectedValue {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}

	return html.String()
}

// RenderMetadataFieldSelect returns a complete metadata field select element
func RenderMetadataFieldSelect(name, id, selectedValue string, onChange string) string {
	onChangeAttr := ""
	if onChange != "" {
		onChangeAttr = fmt.Sprintf(` onchange="%s"`, onChange)
	}

	return fmt.Sprintf(`<select name="%s" id="%s"%s>
		<option value="">select field</option>
		%s
	</select>`, name, id, onChangeAttr, RenderMetadataFieldOptions(selectedValue))
}

// RenderFilterWidgetConfig renders the filter widget configuration form
func RenderFilterWidgetConfig(index int, config *dashboard.WidgetConfig) string {
	var html strings.Builder

	html.WriteString(`<div class="config-form">`)
	html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "filter configuration")))
	html.WriteString(`<div class="config-section">`)
	html.WriteString(fmt.Sprintf(`<h6>%s</h6>`, translation.SprintfForRequest(configmanager.GetLanguage(), "display options")))

	html.WriteString(`<div class="config-row">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "display")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][display]" class="form-select">`, index))

	displayOptions := []string{"list", "cards", "dropdown"}
	selectedDisplay := "list"
	if config != nil && config.Filter != nil {
		selectedDisplay = config.Filter.Display
	}

	for _, option := range displayOptions {
		selected := ""
		if option == selectedDisplay {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	html.WriteString(`<div class="config-row">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "limit")))
	limit := "10"
	if config != nil && config.Filter != nil && config.Filter.Limit > 0 {
		limit = fmt.Sprintf("%d", config.Filter.Limit)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][config][limit]" value="%s" min="1" class="form-input"/>`, index, limit))
	html.WriteString(`</div>`)

	html.WriteString(`<div class="config-row">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "logic")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][logic]" class="form-select">`, index))

	selectedLogic := "and"
	if config != nil && config.Filter != nil {
		selectedLogic = config.Filter.Logic
	}

	html.WriteString(fmt.Sprintf(`<option value="and" %s>and</option>`, utils.Ternary(selectedLogic == "and", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="or" %s>or</option>`, utils.Ternary(selectedLogic == "or", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// filter criteria section
	html.WriteString(`<div class="config-section">`)
	html.WriteString(fmt.Sprintf(`<h6>%s</h6>`, translation.SprintfForRequest(configmanager.GetLanguage(), "filter criteria")))
	html.WriteString(fmt.Sprintf(`<div id="filter-criteria-container-%d">`, index))

	// add existing criteria or one empty criteria
	if config != nil && config.Filter != nil && len(config.Filter.Criteria) > 0 {
		for i, criteria := range config.Filter.Criteria {
			html.WriteString(RenderFilterCriteriaRow(index, i, &criteria))
		}
	} else {
		html.WriteString(RenderFilterCriteriaRow(index, 0, nil))
	}

	html.WriteString(`</div>`)
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/dashboards/filter-criteria" hx-target="#filter-criteria-container-%d" hx-swap="beforeend" hx-vals='{"widget_index": "%d"}' class="btn-add-criteria">%s</button>`, index, index, translation.SprintfForRequest(configmanager.GetLanguage(), "+ add criteria")))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}

// RenderFilterCriteriaRow renders a single filter criteria row
func RenderFilterCriteriaRow(widgetIndex, criteriaIndex int, criteria *files.FilterCriteria) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="filter-criteria-row" data-criteria-index="%d">`, criteriaIndex))

	// metadata field selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "field")))

	selectedMetadata := "collection"
	if criteria != nil {
		selectedMetadata = criteria.Metadata
	}

	valueContainerId := fmt.Sprintf("widget-%d-criteria-%d-value-container", widgetIndex, criteriaIndex)

	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][metadata]" class="form-select" hx-get="/api/dashboards/filter-value-input" hx-target="#%s" hx-swap="innerHTML" hx-vals='{"widget_index": "%d", "criteria_index": "%d"}' hx-include="this">`,
		widgetIndex, criteriaIndex, valueContainerId, widgetIndex, criteriaIndex))
	html.WriteString(`<option value="">select field</option>`)
	html.WriteString(RenderMetadataFieldOptions(selectedMetadata))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// operator selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "operator")))

	selectedOperator := "equals"
	if criteria != nil {
		selectedOperator = criteria.Operator
	}

	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][operator]" class="form-select">`, widgetIndex, criteriaIndex))
	html.WriteString(RenderOperatorOptions(selectedOperator))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// value input (wrapped in container for dynamic updates)
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "value")))
	value := ""
	if criteria != nil {
		value = criteria.Value
	}

	// wrap value input in container div for dynamic updates via HTMX
	html.WriteString(fmt.Sprintf(`<div id="%s">`, valueContainerId))
	valueInputId := fmt.Sprintf("widget-%d-criteria-%d-value", widgetIndex, criteriaIndex)
	valueInputName := fmt.Sprintf("widgets[%d][config][criteria][%d][value]", widgetIndex, criteriaIndex)
	html.WriteString(RenderFilterValueInput(valueInputId, valueInputName, value, selectedMetadata))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// action selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "action")))

	selectedAction := "include"
	if criteria != nil {
		selectedAction = criteria.Action
	}

	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][action]" class="form-select">`, widgetIndex, criteriaIndex))
	html.WriteString(RenderActionOptions(selectedAction))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// remove button
	if criteriaIndex > 0 {
		html.WriteString(`<div class="filter-field-group">`)
		html.WriteString(fmt.Sprintf(`<button type="button" onclick="this.closest('.filter-criteria-row').remove()" class="btn-remove">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "remove")))
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`<hr/>`)

	return html.String()
}

func renderFilterFormWidget() (string, error) {
	var html strings.Builder

	html.WriteString(`<div class="widget-filter-form">`)
	html.WriteString(`<form id="metadata-filter-form" hx-post="/api/files/filter" hx-target="#filter-results">`)
	html.WriteString(`<div>`)
	html.WriteString(`<button type="submit">apply filter</button>`)
	html.WriteString(`<select name="logic" id="logic-operator">`)
	html.WriteString(`<option value="and">and</option>`)
	html.WriteString(`<option value="or">or</option>`)
	html.WriteString(`</select>`)
	html.WriteString(`<button type="button" hx-post="/api/dashboards/filterform-row" hx-target="#filter-container" hx-swap="beforeend">add filter</button>`)
	html.WriteString(`</div>`)
	html.WriteString(`<div id="filter-container">`)

	// add first filter row
	html.WriteString(RenderFilterFormRow(0))

	html.WriteString(`</div>`)
	html.WriteString(`<div id="filter-results"></div>`)
	html.WriteString(`</form>`)
	html.WriteString(`</div>`)

	return html.String(), nil
}

// RenderFilterFormRow renders a single filter row for the filterForm widget
func RenderFilterFormRow(index int) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="filter-row" id="filter-row-%d">`, index))
	html.WriteString(`<hr />`)

	// metadata field select
	valueContainerId := fmt.Sprintf("filterform-value-container-%d", index)
	html.WriteString(fmt.Sprintf(`<select name="metadata[]" id="metadata-%d" class="form-select" hx-get="/api/dashboards/filterform-value-input" hx-target="#%s" hx-swap="innerHTML" hx-vals='{"row_index": "%d"}' hx-include="this">`,
		index, valueContainerId, index))
	html.WriteString(`<option value="">select field</option>`)
	html.WriteString(RenderMetadataFieldOptions(""))
	html.WriteString(`</select>`)

	// operator select
	html.WriteString(fmt.Sprintf(`<select name="operator[]" id="operator-%d" class="form-select">`, index))
	html.WriteString(RenderOperatorOptions(""))
	html.WriteString(`</select>`)

	// value input container
	html.WriteString(fmt.Sprintf(`<div id="%s">`, valueContainerId))
	html.WriteString(fmt.Sprintf(`<input type="text" name="value[]" id="value-%d" placeholder="value" class="form-input"/>`, index))
	html.WriteString(`</div>`)

	// action select
	html.WriteString(fmt.Sprintf(`<select name="action[]" id="action-%d" class="form-select">`, index))
	html.WriteString(RenderActionOptions(""))
	html.WriteString(`</select>`)

	// remove button (only for rows after the first)
	if index > 0 {
		html.WriteString(fmt.Sprintf(`<button type="button" onclick="document.getElementById('filter-row-%d').remove()">remove</button>`, index))
	}

	html.WriteString(`</div>`)

	return html.String()
}

// RenderFilterFormValueInput generates an input with datalist for filterForm widget
func RenderFilterFormValueInput(index int, metadataField string) string {
	inputId := fmt.Sprintf("value-%d", index)
	inputName := "value[]"

	var apiEndpoint string
	var placeholder string

	switch metadataField {
	case "collection":
		apiEndpoint = "/api/metadata/collections?format=options"
		placeholder = "type or select collection (supports wildcards: project*)"
	case "tags":
		apiEndpoint = "/api/metadata/tags?format=options"
		placeholder = "type or select tag (supports wildcards: para/p/*, zk/*)"
	case "folders":
		apiEndpoint = "/api/metadata/folders?format=options"
		placeholder = "type or select folder (supports wildcards: guides/*, *temp*)"
	case "type":
		apiEndpoint = "/api/metadata/filetypes?format=options"
		placeholder = "select file type"
	case "status":
		apiEndpoint = "/api/metadata/statuses?format=options"
		placeholder = "select status"
	case "priority":
		apiEndpoint = "/api/metadata/priorities?format=options"
		placeholder = "select priority"
	case "para_projects":
		apiEndpoint = "/api/metadata/para/projects?format=options"
		placeholder = "type or select project"
	case "para_areas":
		apiEndpoint = "/api/metadata/para/areas?format=options"
		placeholder = "type or select area"
	case "para_resources":
		apiEndpoint = "/api/metadata/para/resources?format=options"
		placeholder = "type or select resource"
	case "para_archive":
		apiEndpoint = "/api/metadata/para/archive?format=options"
		placeholder = "type or select archive"
	case "createdAt", "lastEdited":
		return fmt.Sprintf(`<input type="date" name="%s" id="%s" placeholder="yyyy-mm-dd" class="form-input"/>`,
			inputName, inputId)
	default:
		placeholder = "value"
		return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="" class="form-input" placeholder="%s"/>`,
			inputId, inputName, placeholder)
	}

	// use GenerateDatalistInput without save functionality
	return GenerateDatalistInput(inputId, inputName, "", placeholder, apiEndpoint)
}

// RenderFilterValueInput generates an input with datalist based on metadata field type
func RenderFilterValueInput(id, name, value, metadataField string) string {
	var apiEndpoint string
	var placeholder string

	switch metadataField {
	case "collection":
		apiEndpoint = "/api/metadata/collections?format=options"
		placeholder = "type or select collection"
	case "tags":
		apiEndpoint = "/api/metadata/tags?format=options"
		placeholder = "type or select tag"
	case "folders":
		apiEndpoint = "/api/metadata/folders?format=options"
		placeholder = "type or select folder"
	case "type":
		apiEndpoint = "/api/metadata/filetypes?format=options"
		placeholder = "select file type"
	case "status":
		apiEndpoint = "/api/metadata/statuses?format=options"
		placeholder = "select status"
	case "priority":
		apiEndpoint = "/api/metadata/priorities?format=options"
		placeholder = "select priority"
	case "para_projects":
		apiEndpoint = "/api/metadata/para/projects?format=options"
		placeholder = "type or select project"
	case "para_areas":
		apiEndpoint = "/api/metadata/para/areas?format=options"
		placeholder = "type or select area"
	case "para_resources":
		apiEndpoint = "/api/metadata/para/resources?format=options"
		placeholder = "type or select resource"
	case "para_archive":
		apiEndpoint = "/api/metadata/para/archive?format=options"
		placeholder = "type or select archive item"
	default:
		placeholder = "enter value"
		// no datalist for other fields
		return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" placeholder="%s"/>`,
			id, name, value, placeholder)
	}

	// use GenerateDatalistInput without save functionality
	return GenerateDatalistInput(id, name, value, placeholder, apiEndpoint)
}
