// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderDashboardsList renders the list of dashboards with optional short names
func RenderDashboardsList(dashboards []dashboard.Dashboard, shortNames bool) string {
	var html strings.Builder
	for _, dash := range dashboards {
		displayName := dash.Name

		if shortNames && len(displayName) > 3 {
			// truncate to 3 characters and add tooltip with full name
			displayName = displayName[:3]
			html.WriteString(fmt.Sprintf(`<a href="/dashboard/%s" title="%s">%s</a>`, dash.ID, dash.Name, displayName))
		} else {
			// show full name
			html.WriteString(fmt.Sprintf(`<a href="/dashboard/%s">%s</a>`, dash.ID, dash.Name))
		}
	}
	return html.String()
}

// RenderDashboardCreated renders success message for created dashboard
func RenderDashboardCreated(dashID string) string {
	return fmt.Sprintf(`<div class="success-message">%s <a href="/dashboard/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard created successfully!"),
		dashID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view dashboard"))
}

// RenderDashboardUpdated renders success message for updated dashboard
func RenderDashboardUpdated(dashID string) string {
	return fmt.Sprintf(`<div class="success-message">%s <a href="/dashboard/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard updated successfully!"),
		dashID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view dashboard"))
}

// RenderDashboardInfo renders basic dashboard information
func RenderDashboardInfo(dash *dashboard.Dashboard) string {
	return fmt.Sprintf(`<div><h3>%s</h3><p>Layout: %s</p></div>`, dash.Name, dash.Layout)
}

// RenderDashboardDeleted renders success message for deleted dashboard
func RenderDashboardDeleted() string {
	return fmt.Sprintf(`<div>%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard deleted"))
}

// RenderDashboardRenamed renders success message for renamed dashboard
func RenderDashboardRenamed() string {
	return `<div>dashboard renamed successfully</div>`
}

// RenderDashboardForm renders the complete dashboard form for create or edit
func RenderDashboardForm(dash *dashboard.Dashboard, isEdit bool) string {
	var action, method string
	if isEdit {
		action = fmt.Sprintf("/api/dashboards/%s", dash.ID)
		method = "hx-patch"
	} else {
		action = "/api/dashboards"
		method = "hx-post"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<form %s="%s" hx-target="#dashboard-result" hx-swap="innerHTML" class="dashboard-form">`, method, action))

	// dashboard settings section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(fmt.Sprintf(`<h4>%s</h4>`, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard settings")))
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="name">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard name")))

	nameValue := ""
	if dash != nil {
		nameValue = dash.Name
	}
	html.WriteString(fmt.Sprintf(`<input type="text" id="name" name="name" required value="%s" class="form-input"/>`, nameValue))
	html.WriteString(`</div>`)

	// layout and global checkbox
	html.WriteString(`<div class="form-row">`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label for="layout">%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "layout")))
	html.WriteString(`<select id="layout" name="layout" required class="form-select">`)

	layoutOptions := []string{"oneColumn", "twoColumns", "threeColumns", "fourColumns"}
	selectedLayout := "twoColumns"
	if dash != nil {
		selectedLayout = string(dash.Layout)
	}

	for _, layout := range layoutOptions {
		selected := ""
		if layout == selectedLayout {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, layout, selected, layout))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// global checkbox
	html.WriteString(`<div class="form-group checkbox-group">`)
	html.WriteString(`<label class="checkbox-label">`)
	globalChecked := ""
	if dash != nil && dash.Global {
		globalChecked = "checked"
	}
	html.WriteString(fmt.Sprintf(`<input type="checkbox" name="global" value="true" %s class="form-checkbox"/>`, globalChecked))
	html.WriteString(`<span class="checkmark"></span>`)
	html.WriteString(translation.SprintfForRequest(configmanager.GetLanguage(), "global dashboard"))
	html.WriteString(fmt.Sprintf(`<small>%s</small>`, translation.SprintfForRequest(configmanager.GetLanguage(), "visible to all users")))
	html.WriteString(`</label>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// widgets section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(`<div class="section-header">`)
	html.WriteString(fmt.Sprintf(`<h4>%s</h4>`, translation.SprintfForRequest(configmanager.GetLanguage(), "widgets")))
	html.WriteString(`<button type="button" hx-post="/api/dashboards/widget-form" hx-target="#widgets-container" hx-swap="beforeend">+ add widget</button>`)
	html.WriteString(`</div>`)
	html.WriteString(`<div id="widgets-container">`)

	// add existing widgets if editing
	if dash != nil && len(dash.Widgets) > 0 {
		for i, widget := range dash.Widgets {
			html.WriteString(RenderWidgetForm(i, &widget))
		}
	} else {
		// add one empty widget for new dashboard
		html.WriteString(RenderWidgetForm(0, nil))
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// form actions
	html.WriteString(`<div class="form-actions">`)
	submitText := translation.SprintfForRequest(configmanager.GetLanguage(), "create dashboard")
	if isEdit {
		submitText = translation.SprintfForRequest(configmanager.GetLanguage(), "save changes")
	}
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary"><span>%s</span></button>`, submitText))
	html.WriteString(`</div>`)
	html.WriteString(`</form>`)

	return html.String()
}

// RenderWidgetForm renders a single widget form
func RenderWidgetForm(index int, widget *dashboard.Widget) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="widget-form" data-widget-index="%d">`, index))
	html.WriteString(`<div class="widget-header">`)
	html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "widget")))
	html.WriteString(`<button type="button" onclick="this.parentElement.parentElement.remove()" class="btn-remove-widget">×</button>`)
	html.WriteString(`</div>`)

	// widget type selector
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "widget type")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][type]" required class="form-select widget-type-select" hx-get="/api/dashboards/widget-config" hx-target="#widget-config-%d" hx-swap="innerHTML" hx-vals='{"index": "%d"}' hx-include="[name='widgets[%d][type]']">`, index, index, index, index))

	widgetTypes := []string{"filter", "filterForm", "fileContent", "static", "tags", "collections", "folders"}
	selectedType := ""
	if widget != nil {
		selectedType = string(widget.Type)
	}

	html.WriteString(fmt.Sprintf(`<option value="">%s</option>`, translation.SprintfForRequest(configmanager.GetLanguage(), "select widget type")))
	for _, wType := range widgetTypes {
		selected := ""
		if wType == selectedType {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, wType, selected, wType))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// widget title
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "widget title")))
	titleValue := ""
	if widget != nil {
		titleValue = widget.Title
	}
	html.WriteString(fmt.Sprintf(`<input type="text" name="widgets[%d][title]" value="%s" placeholder="%s" class="form-input"/>`, index, titleValue, translation.SprintfForRequest(configmanager.GetLanguage(), "optional title")))
	html.WriteString(`</div>`)

	// widget position
	html.WriteString(`<div class="form-row">`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "position x")))
	xValue := "0"
	if widget != nil {
		xValue = fmt.Sprintf("%d", widget.Position.X)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][position][x]" value="%s" min="0" class="form-input"/>`, index, xValue))
	html.WriteString(`</div>`)

	html.WriteString(`<div class="form-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "position y")))
	yValue := "0"
	if widget != nil {
		yValue = fmt.Sprintf("%d", widget.Position.Y)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][position][y]" value="%s" min="0" class="form-input"/>`, index, yValue))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// widget config container
	html.WriteString(fmt.Sprintf(`<div id="widget-config-%d" class="widget-config-container">`, index))
	if widget != nil {
		html.WriteString(RenderWidgetConfig(index, string(widget.Type), &widget.Config))
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderWidgetConfig renders widget-specific configuration forms
func RenderWidgetConfig(index int, widgetType string, config *dashboard.WidgetConfig) string {
	var html strings.Builder

	switch widgetType {
	case "filter":
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

		html.WriteString(fmt.Sprintf(`<option value="and" %s>and</option>`, ternary(selectedLogic == "and", "selected", "")))
		html.WriteString(fmt.Sprintf(`<option value="or" %s>or</option>`, ternary(selectedLogic == "or", "selected", "")))
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

	case "fileContent":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file content configuration")))
		html.WriteString(`<div class="config-row">`)
		html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file")))
		selectID := fmt.Sprintf("file-selector-%d", index)
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][filePath]" id="%s" class="form-select" hx-get="/api/files/list?format=options" hx-target="#%s" hx-trigger="load" hx-swap="innerHTML">`, index, selectID, selectID))
		html.WriteString(fmt.Sprintf(`<option value="">%s</option>`, translation.SprintfForRequest(configmanager.GetLanguage(), "loading files...")))
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)
		html.WriteString(fmt.Sprintf(`<p class="config-note">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "select the file you want to display")))
		html.WriteString(`</div>`)

	case "static":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "static content configuration")))
		html.WriteString(`<div class="config-row">`)
		html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "format")))
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][format]" class="form-select">`, index))

		formatOptions := []string{"html", "markdown", "text"}
		selectedFormat := "html"
		if config != nil && config.Static != nil {
			selectedFormat = config.Static.Format
		}

		for _, option := range formatOptions {
			selected := ""
			if option == selectedFormat {
				selected = "selected"
			}
			html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
		}
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)
		html.WriteString(`<div class="config-row">`)
		html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "content")))

		content := translation.SprintfForRequest(configmanager.GetLanguage(), "<h3>welcome!</h3><p>your static content here</p>")
		if config != nil && config.Static != nil {
			content = config.Static.Content
		}
		html.WriteString(fmt.Sprintf(`<textarea name="widgets[%d][config][content]" rows="3" class="form-textarea">%s</textarea>`, index, content))
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

	case "filterForm", "tags", "collections", "folders":
		widgetName := strings.Title(widgetType)
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s widget configuration</h5>`, strings.ToLower(widgetName)))
		html.WriteString(fmt.Sprintf(`<p class="config-note">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no configuration needed")))
		html.WriteString(`</div>`)
	}

	return html.String()
}

// RenderFilterCriteriaRow renders a single filter criteria row
func RenderFilterCriteriaRow(widgetIndex, criteriaIndex int, criteria *files.FilterCriteria) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="filter-criteria-row" data-criteria-index="%d">`, criteriaIndex))

	// metadata field selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "field")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][metadata]" class="form-select">`, widgetIndex, criteriaIndex))

	metadataOptions := []string{"collection", "tags", "type", "status", "priority", "createdAt", "lastEdited", "folders", "boards"}
	selectedMetadata := "collection"
	if criteria != nil {
		selectedMetadata = criteria.Metadata
	}

	for _, option := range metadataOptions {
		selected := ""
		if option == selectedMetadata {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// operator selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "operator")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][operator]" class="form-select">`, widgetIndex, criteriaIndex))

	operatorOptions := []string{"equals", "contains", "greater", "less", "in"}
	selectedOperator := "equals"
	if criteria != nil {
		selectedOperator = criteria.Operator
	}

	for _, option := range operatorOptions {
		selected := ""
		if option == selectedOperator {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// value input
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "value")))
	value := ""
	if criteria != nil {
		value = criteria.Value
	}
	html.WriteString(fmt.Sprintf(`<input type="text" name="widgets[%d][config][criteria][%d][value]" value="%s" placeholder="%s" class="form-input"/>`, widgetIndex, criteriaIndex, value, translation.SprintfForRequest(configmanager.GetLanguage(), "value")))
	html.WriteString(`</div>`)

	// action selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "action")))
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][action]" class="form-select">`, widgetIndex, criteriaIndex))

	selectedAction := "include"
	if criteria != nil {
		selectedAction = criteria.Action
	}

	html.WriteString(fmt.Sprintf(`<option value="include" %s>include</option>`, ternary(selectedAction == "include", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="exclude" %s>exclude</option>`, ternary(selectedAction == "exclude", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// remove button (if not the first criteria)
	if criteriaIndex > 0 {
		html.WriteString(`<button type="button" onclick="this.closest('.filter-criteria-row').remove()" class="btn-remove-criteria">remove"</button>`)
	}

	html.WriteString(`</div>`)

	return html.String()
}

// ternary helper function
func ternary(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}
