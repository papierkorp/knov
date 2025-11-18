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

	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][metadata]" class="form-select">`, widgetIndex, criteriaIndex))
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

	// value input
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "value")))
	value := ""
	if criteria != nil {
		value = criteria.Value
	}
	html.WriteString(fmt.Sprintf(`<input type="text" name="widgets[%d][config][criteria][%d][value]" value="%s" class="form-input" placeholder="value"/>`, widgetIndex, criteriaIndex, value))
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
	metadataSelect := RenderMetadataFieldSelect("metadata[]", "metadata-0", "", "updateValueField(0, this.value)")
	operatorOptions := RenderOperatorOptions("")
	actionOptions := RenderActionOptions("")

	return fmt.Sprintf(`<div class="widget-filter-form">
		<form id="metadata-filter-form" hx-post="/api/files/filter" hx-target="#filter-results">
			<div>
				<button type="submit">apply filter</button>
				<select name="logic" id="logic-operator">
					<option value="and">and</option>
					<option value="or">or</option>
				</select>
				<button type="button" onclick="addFilterRow()">add filter</button>
			</div>
			<div id="filter-container">
				<div class="filter-row" id="filter-row-0">
					%s
					<select name="operator[]" id="operator-0">
						%s
					</select>
					<div id="value-container-0">
						<input type="text" name="value[]" id="value-0" placeholder="value"/>
					</div>
					<select name="action[]" id="action-0">
						%s
					</select>
					<button type="button" onclick="removeFilterRow(0)">remove</button>
				</div>
			</div>
			<div id="filter-results"></div>
		</form>

		<script>
			let filterRowCount = 1;

			function addFilterRow() {
				const container = document.getElementById('filter-container');
				const newRow = document.createElement('div');
				newRow.className = 'filter-row';
				newRow.id = 'filter-row-' + filterRowCount;

				const selectHTML = '<select name="metadata[]" id="metadata-' + filterRowCount + '" onchange="updateValueField(' + filterRowCount + ', this.value)">' +
					'<option value="">select field</option>' +
					'%s' +
					'</select>';

				const operatorHTML = '<select name="operator[]" id="operator-' + filterRowCount + '">' +
					'%s' +
					'</select>';

				const valueHTML = '<div id="value-container-' + filterRowCount + '">' +
					'<input type="text" name="value[]" id="value-' + filterRowCount + '" placeholder="value"/>' +
					'</div>';

				const actionHTML = '<select name="action[]" id="action-' + filterRowCount + '">' +
					'%s' +
					'</select>';

				const removeHTML = '<button type="button" onclick="removeFilterRow(' + filterRowCount + ')">remove</button>';

				newRow.innerHTML = selectHTML + operatorHTML + valueHTML + actionHTML + removeHTML;
				container.appendChild(newRow);
				filterRowCount++;
			}

			function removeFilterRow(index) {
				const row = document.getElementById('filter-row-' + index);
				if (row) row.remove();
			}

		function updateValueField(rowIndex, fieldType) {
			const container = document.getElementById('value-container-' + rowIndex);

			if (fieldType === 'collection') {
				container.innerHTML = '<input type="text" name="value[]" autocomplete="off" id="value-' + rowIndex + '" list="collections-' + rowIndex + '" placeholder="type or select collection (supports wildcards: project*)">' +
					'<datalist id="collections-' + rowIndex + '" hx-get="/api/metadata/options/collections" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading collections...</option>' +
					'</datalist>';
			} else if (fieldType === 'tags') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="tags-' + rowIndex + '" placeholder="type or select tag (supports wildcards: para/p/*, zk/*)">' +
					'<datalist id="tags-' + rowIndex + '" hx-get="/api/metadata/options/tags" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading tags...</option>' +
					'</datalist>';
			} else if (fieldType === 'folders') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="folders-' + rowIndex + '" placeholder="type or select folder (supports wildcards: guides/*, *temp*)">' +
					'<datalist id="folders-' + rowIndex + '" hx-get="/api/metadata/options/folders" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading folders...</option>' +
					'</datalist>';
			} else if (fieldType === 'type') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/filetypes" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading types...</option>' +
					'</select>';
			} else if (fieldType === 'status') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/status" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading status...</option>' +
					'</select>';
			} else if (fieldType === 'priority') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/priorities" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading priorities...</option>' +
					'</select>';
			} else if (fieldType === 'para_projects') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="para-projects-' + rowIndex + '" placeholder="type or select project">' +
					'<datalist id="para-projects-' + rowIndex + '" hx-get="/api/metadata/para/projects?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading projects...</option>' +
					'</datalist>';
			} else if (fieldType === 'para_areas') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="para-areas-' + rowIndex + '" placeholder="type or select area">' +
					'<datalist id="para-areas-' + rowIndex + '" hx-get="/api/metadata/para/areas?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading areas...</option>' +
					'</datalist>';
			} else if (fieldType === 'para_resources') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="para-resources-' + rowIndex + '" placeholder="type or select resource">' +
					'<datalist id="para-resources-' + rowIndex + '" hx-get="/api/metadata/para/resources?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading resources...</option>' +
					'</datalist>';
			} else if (fieldType === 'para_archive') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="para-archive-' + rowIndex + '" placeholder="type or select archive">' +
					'<datalist id="para-archive-' + rowIndex + '" hx-get="/api/metadata/para/archive?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading archive...</option>' +
					'</datalist>';
			} else if (fieldType === 'createdAt' || fieldType === 'lastEdited') {
				container.innerHTML = '<input type="date" name="value[]" id="value-' + rowIndex + '" placeholder="yyyy-mm-dd"/>';
			} else {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" placeholder="value"/>';
			}

			// trigger htmx processing for new elements
			if (window.htmx) {
				htmx.process(container);
			}
		}

			// initialize first row
			document.addEventListener('DOMContentLoaded', function() {
				updateValueField(0, document.getElementById('metadata-0').value);
			});
		</script>
	</div>`,
		metadataSelect,
		operatorOptions,
		actionOptions,
		RenderMetadataFieldOptions(""),
		RenderOperatorOptions(""),
		RenderActionOptions(""),
	), nil
}
