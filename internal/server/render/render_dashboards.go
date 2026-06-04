// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
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
	return fmt.Sprintf(`<div class="status-ok">%s <a href="/dashboard/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard created successfully!"),
		dashID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view dashboard"))
}

// RenderDashboardUpdated renders success message for updated dashboard
func RenderDashboardUpdated(dashID string) string {
	return fmt.Sprintf(`<div class="status-ok">%s <a href="/dashboard/%s">%s</a></div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard updated successfully!"),
		dashID,
		translation.SprintfForRequest(configmanager.GetLanguage(), "view dashboard"))
}

// RenderDashboardInfo renders basic dashboard information
func RenderDashboardInfo(dash *dashboard.Dashboard) string {
	return fmt.Sprintf(`<div><h3>%s</h3><p>%s: %s</p></div>`, dash.Name, translation.SprintfForRequest(configmanager.GetLanguage(), "layout"), dash.Layout)
}

// RenderDashboardDeleted renders success message for deleted dashboard
func RenderDashboardDeleted() string {
	return fmt.Sprintf(`<div>%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard deleted"))
}

// RenderDashboardRenamed renders success message for renamed dashboard
func RenderDashboardRenamed() string {
	return fmt.Sprintf(`<div>%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard renamed successfully"))
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
	if isEdit {
		html.WriteString(fmt.Sprintf(`<a href="/dashboard/%s" class="btn-secondary">%s</a>`, dash.ID, translation.SprintfForRequest(configmanager.GetLanguage(), "cancel")))
	} else {
		html.WriteString(fmt.Sprintf(`<a href="/" class="btn-secondary">%s</a>`, translation.SprintfForRequest(configmanager.GetLanguage(), "cancel")))
	}
	html.WriteString(`</div>`)
	html.WriteString(`</form>`)

	// export + delete (edit mode only)
	if isEdit {
		html.WriteString(fmt.Sprintf(
			`<div class="form-section dashboard-export-section">`+
				`<h4>%s</h4>`+
				`<div class="dashboard-export-actions">`+
				`<a href="/api/dashboards/%s/export" class="btn-secondary">%s</a>`+
				`<button type="button" class="btn-danger"`+
				` hx-delete="/api/dashboards/%s"`+
				` hx-confirm="%s"`+
				` hx-target="#dashboard-result"`+
				` hx-swap="innerHTML"`+
				`>%s</button>`+
				`</div>`+
				`</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard actions"),
			dash.ID,
			translation.SprintfForRequest(configmanager.GetLanguage(), "export"),
			dash.ID,
			translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this dashboard?"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "delete dashboard"),
		))
	}

	// import form (always visible)
	html.WriteString(fmt.Sprintf(
		`<div class="form-section dashboard-import-section">`+
			`<h4>%s</h4>`+
			`<form hx-post="/api/dashboards/import" hx-target="#import-result" hx-encoding="multipart/form-data" class="import-form">`+
			`<input type="text" name="name" placeholder="%s" class="form-input" />`+
			`<input type="file" name="file" accept=".json" required />`+
			`<button type="submit" class="btn-secondary">%s</button>`+
			`</form>`+
			`<div id="import-result"></div>`+
			`</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "import dashboard"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "new name (optional)"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "import"),
	))

	// JS: swap widget DOM nodes and renumber field names
	html.WriteString(`<script>
function moveWidget(btn, dir) {
	const el = btn.closest('.widget-form');
	const container = el.parentElement;
	const siblings = Array.from(container.querySelectorAll(':scope > .widget-form'));
	const idx = siblings.indexOf(el);
	const target = siblings[idx + dir];
	if (!target) return;
	if (dir === -1) container.insertBefore(el, target);
	else container.insertBefore(target, el);
	// renumber all widgets[N] field names to match new order
	Array.from(container.querySelectorAll(':scope > .widget-form')).forEach((w, i) => {
		w.dataset.widgetIndex = i;
		w.querySelectorAll('[name]').forEach(f => {
			f.name = f.name.replace(/widgets\[\d+\]/, 'widgets[' + i + ']');
		});
		w.querySelectorAll('[id]').forEach(f => {
			f.id = f.id.replace(/(-\d+)$/, '-' + i);
		});
	});
}
</script>`)

	return html.String()
}

// RenderWidgetForm renders a single widget form
func RenderWidgetForm(index int, widget *dashboard.Widget) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="widget-form" data-widget-index="%d">`, index))
	html.WriteString(`<div class="widget-header">`)
	html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "widget")))
	html.WriteString(`<div class="widget-header-actions">`)
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveWidget(this,-1)" class="btn-widget-move" title="%s">↑</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "move up")))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveWidget(this,1)" class="btn-widget-move" title="%s">↓</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "move down")))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="this.closest('.widget-form').remove()" class="btn-remove-widget" title="%s">✕</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "remove widget")))
	html.WriteString(`</div>`)
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
		return RenderFilterWidgetConfig(index, config)

	case "fileContent":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s</h5>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file content configuration")))
		html.WriteString(`<div class="config-row">`)
		html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file path")))
		filePathValue := ""
		if config != nil && config.FileContent != nil {
			filePathValue = config.FileContent.FilePath
		}
		selectID := fmt.Sprintf("file-selector-%d", index)
		html.WriteString(fmt.Sprintf(
			`<input type="text" name="widgets[%d][config][filePath]" id="%s" value="%s" placeholder="docs/my-file.md" class="form-input" list="file-suggestions-%d" />`,
			index, selectID, filePathValue, index,
		))
		html.WriteString(fmt.Sprintf(
			`<datalist id="file-suggestions-%d" hx-get="/api/files/list?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>`,
			index,
		))
		html.WriteString(`</div>`)
		html.WriteString(fmt.Sprintf(`<p class="config-note">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "enter the path to the file you want to display")))
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
		widgetName := string(widgetType)
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s widget configuration</h5>`, strings.ToLower(widgetName)))
		html.WriteString(fmt.Sprintf(`<p class="config-note">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no configuration needed")))
		html.WriteString(`</div>`)
	}

	return html.String()
}
