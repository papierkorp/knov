// Package render - Basic filter HTML rendering functions
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/mapping"
	"knov/internal/translation"
	"knov/internal/utils"
)

// ----------------------------------------------------------------------------------------
// ------------------------------------------ OPTS ----------------------------------------
// ----------------------------------------------------------------------------------------

// FilterFormContext defines how the filter form should behave
type FilterFormContext int

const (
	FilterFormContextApply     FilterFormContext = iota // standalone apply (browse/filter page)
	FilterFormContextSave                               // save as named filter (new/edit)
	FilterFormContextDashboard                          // embedded in dashboard widget config
	FilterFormContextKanban                             // embedded in kanban board toolbar
)

// FilterFormOpts configures the filter form rendering
type FilterFormOpts struct {
	Context     FilterFormContext
	Config      *filter.Config
	FilterID    string // for Save context: shown as input (new) or hidden (edit)
	IsEdit      bool   // for Save context: true = editing existing filter
	WidgetIndex int    // for Dashboard context
	Collection  string // for Kanban context
}

// ----------------------------------------------------------------------------------------
// ------------------------------------------ FORM ----------------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterForm renders a filter form using the provided options
func RenderFilterForm(opts FilterFormOpts) string {
	var html strings.Builder

	submitLabel, criteriaTarget := resolveFilterFormContext(opts)

	if opts.Context != FilterFormContextDashboard {
		action, submitTarget := resolveFilterFormActionTarget(opts)
		html.WriteString(fmt.Sprintf(`<form id="%s" hx-post="%s" hx-target="%s">`,
			filterFormID(opts), action, submitTarget))
	}

	// id field (save context only)
	if opts.Context == FilterFormContextSave {
		if !opts.IsEdit {
			html.WriteString(`<div class="form-group">`)
			html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter name") + `:</label>`)
			datalistInput := GenerateDatalistInput("filterid-input", "filterid", opts.FilterID,
				translation.SprintfForRequest(configmanager.GetLanguage(), "my-filter"), "/api/files/folder-suggestions")
			datalistInput = strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1)
			html.WriteString(datalistInput)
			html.WriteString(`</div>`)
		} else {
			html.WriteString(fmt.Sprintf(`<input type="hidden" name="filterid" value="%s" />`, opts.FilterID))
		}
	}

	// controls row
	html.WriteString(`<div class="filter-controls">`)
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary">%s</button>`, submitLabel))
	if opts.Context == FilterFormContextSave {
		html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/filters" hx-include="closest form" hx-target="#filter-results" class="btn-secondary">%s</button>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "preview results")))
		if opts.IsEdit {
			html.WriteString(fmt.Sprintf(`<button type="button" data-href="/files/%s" onclick="window.location.href=this.dataset.href" class="btn-secondary">%s</button>`,
				filter.FilterIndexPath(opts.FilterID),
				translation.SprintfForRequest(configmanager.GetLanguage(), "cancel")))
		}
	}
	html.WriteString(fmt.Sprintf(
		`<button type="button" hx-post="/api/filters/add-criteria" hx-target="#%s" hx-swap="beforeend"%s class="btn-secondary">%s</button>`,
		criteriaTarget,
		widgetIndexVals(opts),
		translation.SprintfForRequest(configmanager.GetLanguage(), "add filter")))
	html.WriteString(renderLogicToggle(opts))
	if opts.Context != FilterFormContextKanban {
		html.WriteString(`<span class="filter-controls-sep"></span>`)
		html.WriteString(renderDisplaySelect(opts))
		html.WriteString(fmt.Sprintf(`<input type="number" name="%s" value="%s" min="1" class="form-input filter-limit-input" title="%s"/>`,
			filterFieldName(opts, "limit"), resolvedLimitValue(opts.Config),
			translation.SprintfForRequest(configmanager.GetLanguage(), "limit")))
	}
	html.WriteString(`</div>`)

	// criteria
	html.WriteString(fmt.Sprintf(`<div id="%s" class="filter-criteria-container">`, criteriaTarget))
	if opts.Config != nil && len(opts.Config.Criteria) > 0 {
		for i, c := range opts.Config.Criteria {
			html.WriteString(RenderFilterCriteriaRow(widgetIndex(opts), i, &c))
		}
	} else {
		html.WriteString(RenderFilterCriteriaRow(widgetIndex(opts), 0, nil))
	}
	html.WriteString(`</div>`)

	if opts.Context != FilterFormContextDashboard {
		html.WriteString(`</form>`)
	}
	return html.String()
}

// ----------------------------------------------------------------------------------------
// ----------------------------------- FORM HELPERS ----------------------------------------
// ----------------------------------------------------------------------------------------

func filterFormID(opts FilterFormOpts) string {
	if opts.Context == FilterFormContextDashboard {
		return fmt.Sprintf("filter-form-%d", opts.WidgetIndex)
	}
	return "filter-form"
}

func widgetIndex(opts FilterFormOpts) int {
	if opts.Context == FilterFormContextDashboard {
		return opts.WidgetIndex
	}
	return -1
}

func widgetIndexVals(opts FilterFormOpts) string {
	if opts.Context == FilterFormContextDashboard {
		return fmt.Sprintf(` hx-vals='{"widget_index": "%d"}'`, opts.WidgetIndex)
	}
	return ""
}

func filterFieldName(opts FilterFormOpts, field string) string {
	if opts.Context == FilterFormContextDashboard {
		return fmt.Sprintf("widgets[%d][config][%s]", opts.WidgetIndex, field)
	}
	return field
}

func resolveFilterFormContext(opts FilterFormOpts) (submitLabel, criteriaTarget string) {
	switch opts.Context {
	case FilterFormContextSave:
		return translation.SprintfForRequest(configmanager.GetLanguage(), "save filter"),
			"filter-criteria-container"
	case FilterFormContextDashboard:
		return translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter"),
			fmt.Sprintf("filter-criteria-container-%d", opts.WidgetIndex)
	case FilterFormContextKanban:
		return translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter"),
			"filter-criteria-container"
	default: // FilterFormContextApply
		return translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter"),
			"filter-criteria-container"
	}
}

func resolveFilterFormActionTarget(opts FilterFormOpts) (action, submitTarget string) {
	switch opts.Context {
	case FilterFormContextSave:
		return "/api/filters/save", "#editor-status"
	case FilterFormContextKanban:
		return fmt.Sprintf("/api/kanban/%s/filter", opts.Collection), "#view-kanban-board-wrap"
	default: // FilterFormContextApply
		return "/api/filters", "#filter-results"
	}
}

func renderLogicToggle(opts FilterFormOpts) string {
	selected := "and"
	if opts.Config != nil {
		selected = opts.Config.Logic
	}
	name := filterFieldName(opts, "logic")
	andLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "and")
	orLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "or")
	return fmt.Sprintf(
		`<span class="filter-logic-switch" onclick="this.querySelectorAll('.filter-logic-opt').forEach(l=>l.classList.toggle('active',l.querySelector('input').checked))">
			<label class="filter-logic-opt%s"><input type="radio" name="%s" value="and" %s>%s</label>
			<label class="filter-logic-opt%s"><input type="radio" name="%s" value="or" %s>%s</label>
		</span>`,
		utils.Ternary(selected == "and", " active", ""), name, utils.Ternary(selected == "and", "checked", ""), andLabel,
		utils.Ternary(selected == "or", " active", ""), name, utils.Ternary(selected == "or", "checked", ""), orLabel)
}

func renderDisplaySelect(opts FilterFormOpts) string {
	selected := "list"
	if opts.Config != nil {
		selected = opts.Config.Display
	}
	name := filterFieldName(opts, "display")
	displayOpts := []struct{ v, l string }{
		{"list", translation.SprintfForRequest(configmanager.GetLanguage(), "list")},
		{"list2", translation.SprintfForRequest(configmanager.GetLanguage(), "list (2 col)")},
		{"list3", translation.SprintfForRequest(configmanager.GetLanguage(), "list (3 col)")},
		{"list4", translation.SprintfForRequest(configmanager.GetLanguage(), "list (4 col)")},
		{"cards", translation.SprintfForRequest(configmanager.GetLanguage(), "cards")},
		{"dropdown", translation.SprintfForRequest(configmanager.GetLanguage(), "dropdown")},
		{"content", translation.SprintfForRequest(configmanager.GetLanguage(), "content")},
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<select name="%s" class="form-select">`, name))
	for _, o := range displayOpts {
		b.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`,
			o.v, utils.Ternary(selected == o.v, "selected", ""), o.l))
	}
	b.WriteString(`</select>`)
	return b.String()
}

func resolvedLimitValue(config *filter.Config) string {
	if config != nil && config.Limit > 0 {
		return fmt.Sprintf("%d", config.Limit)
	}
	return "50"
}

// ----------------------------------------------------------------------------------------
// ----------------------------------------- RESULT ----------------------------------------
// ----------------------------------------------------------------------------------------

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
	case "list2":
		return fmt.Sprintf(`<div id="filter-results" class="filter-list-grid filter-list-grid-2">%s</div>`, renderFileListItems(result.Files))
	case "list3":
		return fmt.Sprintf(`<div id="filter-results" class="filter-list-grid filter-list-grid-3">%s</div>`, renderFileListItems(result.Files))
	case "list4":
		return fmt.Sprintf(`<div id="filter-results" class="filter-list-grid filter-list-grid-4">%s</div>`, renderFileListItems(result.Files))
	default:
		return fmt.Sprintf(`<div id="filter-results">%s</div>`, RenderFileList(result.Files))
	}
}

// renderFileListItems renders file list items as bare <a> tags for grid layouts
func renderFileListItems(fileList []files.File) string {
	var b strings.Builder
	for _, file := range fileList {
		b.WriteString(fmt.Sprintf(`<a class="filter-list-item" href="%s">%s</a>`, file.ViewURL(), GetLinkDisplayText(file.Path)))
	}
	return b.String()
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ SINGLE CRITERIA ------------------------------------
// ----------------------------------------------------------------------------------------

// criteriaFieldName returns form field name for a criteria field
func criteriaFieldName(widgetIndex, rowIndex int, field string) string {
	if widgetIndex < 0 {
		return fmt.Sprintf("%s[%d]", field, rowIndex)
	}
	return fmt.Sprintf("widgets[%d][config][criteria][%d][%s]", widgetIndex, rowIndex, field)
}

// criteriaValueContainerID returns the HTML element ID for a value input container
func criteriaValueContainerID(widgetIndex, rowIndex int) string {
	if widgetIndex < 0 {
		return fmt.Sprintf("filter-value-container-%d", rowIndex)
	}
	return fmt.Sprintf("filter-value-container-%d-%d", widgetIndex, rowIndex)
}

// RenderFilterCriteriaRow renders a single filter criteria row.
// Pass widgetIndex >= 0 for widget-namespaced fields, or -1 for standalone filter forms.
func RenderFilterCriteriaRow(widgetIndex, rowIndex int, criteria *filter.Criteria) string {
	var html strings.Builder
	containerID := criteriaValueContainerID(widgetIndex, rowIndex)

	html.WriteString(fmt.Sprintf(`<div class="filter-criteria-row" data-index="%d">`, rowIndex))

	html.WriteString(`<div class="filter-field">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "field") + `</label>`)

	hxVals := fmt.Sprintf(`{"row_index": "%d"}`, rowIndex)
	if widgetIndex >= 0 {
		hxVals = fmt.Sprintf(`{"row_index": "%d", "widget_index": "%d"}`, rowIndex, widgetIndex)
	}
	html.WriteString(fmt.Sprintf(`<select name="%s" class="form-select" hx-get="/api/filters/value-input" hx-target="#%s" hx-swap="innerHTML" hx-vals='%s' hx-include="this">`,
		criteriaFieldName(widgetIndex, rowIndex, "metadata"), containerID, hxVals))
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select field") + `</option>`)
	selectedMetadata := ""
	if criteria != nil {
		selectedMetadata = criteria.Metadata
	}
	html.WriteString(RenderMetadataFieldOptions(selectedMetadata))
	html.WriteString(`</select></div>`)

	html.WriteString(`<div class="filter-field">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "operator") + `</label>`)
	html.WriteString(fmt.Sprintf(`<select name="%s" class="form-select">`, criteriaFieldName(widgetIndex, rowIndex, "operator")))
	selectedOperator := "equals"
	if criteria != nil {
		selectedOperator = criteria.Operator
	}
	html.WriteString(RenderOperatorOptions(selectedOperator))
	html.WriteString(`</select></div>`)

	html.WriteString(`<div class="filter-field filter-field-value">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "value") + `</label>`)
	html.WriteString(fmt.Sprintf(`<div id="%s">`, containerID))
	value, metadataField := "", ""
	if criteria != nil {
		value = criteria.Value
		metadataField = criteria.Metadata
	}
	inputID := fmt.Sprintf("filter-value-%d", rowIndex)
	if widgetIndex >= 0 {
		inputID = fmt.Sprintf("filter-value-%d-%d", widgetIndex, rowIndex)
	}
	html.WriteString(RenderFilterValueInput(inputID, criteriaFieldName(widgetIndex, rowIndex, "value"), value, metadataField))
	html.WriteString(`</div></div>`)

	html.WriteString(`<div class="filter-field filter-field-action">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "action") + `</label>`)
	html.WriteString(fmt.Sprintf(`<select name="%s" class="form-select">`, criteriaFieldName(widgetIndex, rowIndex, "action")))
	selectedAction := "include"
	if criteria != nil {
		selectedAction = criteria.Action
	}
	html.WriteString(RenderActionOptions(selectedAction))
	html.WriteString(`</select>`)
	if rowIndex > 0 {
		html.WriteString(`<button type="button" onclick="this.closest('.filter-criteria-row').remove()" class="filter-remove-btn" title="` + translation.SprintfForRequest(configmanager.GetLanguage(), "remove") + `"><i class="fa fa-times"></i></button>`)
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderMetadataFieldOptions returns HTML options for metadata field selectors
func RenderMetadataFieldOptions(selectedValue string) string {
	var html strings.Builder
	for _, field := range filter.GetMetadataFields() {
		selected := ""
		if field == selectedValue {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, field, selected, mapping.GetDisplayName(field)))
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
	for _, action := range filter.GetActions() {
		selected := ""
		if action == selectedValue {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`,
			action, selected, translation.SprintfForRequest(configmanager.GetLanguage(), action)))
	}
	return html.String()
}

// RenderFilterValueInput generates an input with datalist based on metadata field type
func RenderFilterValueInput(id, name, value, metadataField string) string {
	switch metadataField {
	case "createdAt", "lastEdited", "kanbanAddedAt", "kanbanMovedAt":
		return fmt.Sprintf(`<input type="date" name="%s" id="%s" value="%s" placeholder="%s" class="form-input"/>`,
			name, id, value, translation.SprintfForRequest(configmanager.GetLanguage(), "yyyy-mm-dd"))
	}

	apiEndpoint, placeholder := filterValueInputMeta(metadataField)
	if apiEndpoint == "" {
		return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" placeholder="%s"/>`,
			id, name, value, placeholder)
	}
	return GenerateDatalistInput(id, name, value, placeholder, apiEndpoint)
}

func filterValueInputMeta(metadataField string) (apiEndpoint, placeholder string) {
	switch metadataField {
	case "collection":
		return "/api/metadata/collections?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "type or select collection")
	case "tags":
		return "/api/metadata/tags?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "type or select tag")
	case "folders":
		return "/api/metadata/folders?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "type or select folder")
	case "editor":
		return "/api/metadata/editors?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "select editor type")
	case "title":
		return "/api/metadata/titles?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "type or select title")
	case "child-of", "parent-of", "ancestor-of":
		return "/api/files/list?format=options", translation.SprintfForRequest(configmanager.GetLanguage(), "select file")
	default:
		return "", translation.SprintfForRequest(configmanager.GetLanguage(), "enter value")
	}
}
