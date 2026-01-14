// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"sort"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// DescriptionType defines how setting descriptions should be displayed
type DescriptionType string

const (
	// DescriptionTypeTooltips displays descriptions as hover tooltips
	DescriptionTypeTooltips DescriptionType = "tooltips"
	// DescriptionTypeHelpText displays descriptions as always-visible help text below form elements
	DescriptionTypeHelpText DescriptionType = "help-text"
)

// RenderThemeOptions renders theme options for select dropdown
func RenderThemeOptions(availableThemes []thememanager.Theme, currentTheme thememanager.Theme) string {
	var html strings.Builder
	for _, theme := range availableThemes {
		selected := ""
		if theme.Name == currentTheme.Name {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, theme.Name, selected, theme.Name))
	}
	return html.String()
}

// RenderThemeSettings renders theme settings as HTML for display (simple view)
func RenderThemeSettings(settings interface{}, themeName string) string {
	return fmt.Sprintf(`<div id="theme-settings-%s">
		<h4>%s</h4>
		<pre>%+v</pre>
	</div>`, themeName, translation.SprintfForRequest(configmanager.GetLanguage(), "settings for %s", themeName), settings)
}

// RenderThemeSettingsForm renders all theme settings as form elements with configurable description display
func RenderThemeSettingsForm(schema map[string]thememanager.ThemeSetting, currentValues map[string]interface{}, descriptionType DescriptionType) string {
	var html strings.Builder

	// extract and sort keys for consistent ordering
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// iterate in sorted order
	for _, key := range keys {
		setting := schema[key]
		html.WriteString(`<div class="setting-item">`)

		// get current value
		currentValue := currentValues[key]
		if currentValue == nil {
			currentValue = setting.Default
		}

		// render based on type
		switch setting.Type {
		case "boolean":
			enabled := false
			if v, ok := currentValue.(bool); ok {
				enabled = v
			}

			// render label with conditional tooltip
			if descriptionType == DescriptionTypeTooltips && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<label class="tooltip" data-tooltip="%s">%s`, setting.Description, setting.Label))
			} else {
				html.WriteString(fmt.Sprintf(`<label>%s`, setting.Label))
			}

			html.WriteString(RenderCheckbox(key, "/api/themes/settings", enabled,
				fmt.Sprintf(`hx-vals='js:{"key": "%s", "value": event.target.checked}' hx-trigger="change"`, key)))
			html.WriteString(`</label>`)

			// render help text if not using tooltips
			if descriptionType == DescriptionTypeHelpText && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, setting.Description))
			}

		case "select":
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, setting.Description, setting.Label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, setting.Label))
			}

			html.WriteString(fmt.Sprintf(`<select name="value" id="%s">`, key))

			current := ""
			if v, ok := currentValue.(string); ok {
				current = v
			}

			for _, option := range setting.Options {
				selected := ""
				if option == current {
					selected = "selected"
				}
				html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
			}
			html.WriteString(`</select>`)

			// render help text if not using tooltips
			if descriptionType == "help-text" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, setting.Description))
			}
			html.WriteString(`</form>`)

		case "text":
			current := ""
			if v, ok := currentValue.(string); ok {
				current = v
			}
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, setting.Description, setting.Label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, setting.Label))
			}

			html.WriteString(fmt.Sprintf(`<input type="text" name="value" id="%s" value="%s" />`, key, current))

			// render help text if not using tooltips
			if descriptionType == "help-text" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, setting.Description))
			}
			html.WriteString(`</form>`)

		case "textarea":
			current := ""
			if v, ok := currentValue.(string); ok {
				current = v
			}
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change delay:500ms">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, setting.Description, setting.Label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, setting.Label))
			}

			html.WriteString(fmt.Sprintf(`<textarea name="value" id="%s" rows="10" style="width: 100%%; font-family: monospace;">%s</textarea>`, key, current))

			// render help text if not using tooltips
			if descriptionType == "help-text" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, setting.Description))
			}
			html.WriteString(`</form>`)

		case "number":
			current := 0
			if v, ok := currentValue.(float64); ok {
				current = int(v)
			} else if v, ok := currentValue.(int); ok {
				current = v
			}
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, setting.Description, setting.Label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, setting.Label))
			}

			html.WriteString(fmt.Sprintf(`<input type="number" name="value" id="%s" value="%d" />`, key, current))

			// render help text if not using tooltips
			if descriptionType == "help-text" && setting.Description != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, setting.Description))
			}
			html.WriteString(`</form>`)
		}

		html.WriteString(`</div>`)
	}

	return html.String()
}
