// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
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

	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	// extract and sort keys for consistent ordering
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// iterate in sorted order
	for _, key := range keys {
		setting := schema[key]
		label := t(setting.Label)
		desc := t(setting.Description)
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
			checkedAttr := ""
			if enabled {
				checkedAttr = "checked"
			}
			html.WriteString(fmt.Sprintf(
				`<label class="checkbox-label">
					<input type="checkbox" name="%s" class="form-checkbox" %s
						hx-post="/api/themes/settings"
						hx-vals='js:{"key": "%s", "value": event.target.checked}'
						hx-trigger="change" />
					<span class="checkmark"></span>
					%s
				</label>`,
				key, checkedAttr, key, label))
			if descriptionType == DescriptionTypeHelpText && desc != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, desc))
			}

		case "select":
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && desc != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, desc, label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, label))
			}

			html.WriteString(fmt.Sprintf(`<select name="value" id="%s" class="form-select">`, key))

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
			if descriptionType == "help-text" && desc != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, desc))
			}
			html.WriteString(`</form>`)

		case "text":
			current := ""
			if v, ok := currentValue.(string); ok {
				current = v
			}
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}' hx-trigger="change">`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && desc != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, desc, label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, label))
			}

			html.WriteString(fmt.Sprintf(`<input type="text" name="value" id="%s" value="%s" class="form-input" />`, key, current))

			// render help text if not using tooltips
			if descriptionType == "help-text" && desc != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, desc))
			}
			html.WriteString(`</form>`)

		case "textarea":
			// textarea values are opaque strings to this form, but a schema/config
			// default can also arrive as parsed JSON (e.g. railLayout's default in
			// theme.json is a native array) rather than an already-serialized
			// string - re-marshal it back into the same string form the textarea
			// (and rail-layout-builder.js, which parses it as JSON) expects.
			asTextareaString := func(v interface{}) string {
				if s, ok := v.(string); ok {
					return s
				}
				if b, err := json.Marshal(v); err == nil {
					return string(b)
				}
				return ""
			}
			current := asTextareaString(currentValue)
			defaultVal := asTextareaString(setting.Default)
			// no auto-submit trigger: the rail-layout drag-and-drop builder (see
			// rail-layout-builder.js) writes into this textarea on every drag, so
			// saving on "change" would reload mid-drag. saved explicitly via the button below.
			html.WriteString(fmt.Sprintf(`<form hx-post="/api/themes/settings" hx-vals='{"key": "%s"}'>`, key))

			// render label with conditional tooltip
			if descriptionType == "tooltips" && desc != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, desc, label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, label))
			}

			html.WriteString(fmt.Sprintf(`<textarea name="value" id="%s" rows="10" class="form-textarea" data-default="%s">%s</textarea>`,
				key, stdhtml.EscapeString(defaultVal), stdhtml.EscapeString(current)))

			// render help text if not using tooltips
			if descriptionType == "help-text" && desc != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, desc))
			}
			html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary">%s</button>`, t("save")))
			// plain textarea reset, theme-agnostic; themes that overlay a custom
			// widget on the textarea (e.g. builtin-reworked's rail-layout-builder.js)
			// listen for "settings-textarea-reset" to rebuild their own UI too.
			html.WriteString(fmt.Sprintf(
				`<button type="button" class="btn-secondary" style="margin-left: 8px;" data-confirm="%s" onclick="if(confirm(this.dataset.confirm)){var ta=document.getElementById('%s');ta.value=ta.dataset.default;ta.dispatchEvent(new CustomEvent('settings-textarea-reset',{bubbles:true}));}">%s</button>`,
				stdhtml.EscapeString(t("reset to default? unsaved changes will be lost")), key, t("reset to default")))
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
			if descriptionType == "tooltips" && desc != "" {
				html.WriteString(fmt.Sprintf(`<label for="%s" class="tooltip" data-tooltip="%s">%s</label>`, key, desc, label))
			} else {
				html.WriteString(fmt.Sprintf(`<label for="%s">%s</label>`, key, label))
			}

			html.WriteString(fmt.Sprintf(`<input type="number" name="value" id="%s" value="%d" class="form-input" />`, key, current))

			// render help text if not using tooltips
			if descriptionType == "help-text" && desc != "" {
				html.WriteString(fmt.Sprintf(`<div class="help-text">%s</div>`, desc))
			}
			html.WriteString(`</form>`)
		}

		html.WriteString(`</div>`)
	}

	return html.String()
}
