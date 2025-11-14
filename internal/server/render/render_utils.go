// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
)

// SelectOption represents an option in a select dropdown
type SelectOption struct {
	Value string
	Label string
}

// RenderInputField renders an input field with specified type, name, id, value and placeholder
func RenderInputField(inputType, name, id, value, placeholder string, required bool) string {
	requiredAttr := ""
	if required {
		requiredAttr = "required"
	}
	return fmt.Sprintf(`<input type="%s" name="%s" id="%s" value="%s" placeholder="%s" %s />`,
		inputType, name, id, value, placeholder, requiredAttr)
}

// StatusClass represents valid status message classes
type StatusClass string

const (
	StatusOK      StatusClass = "status-ok"
	StatusError   StatusClass = "status-error"
	StatusWarning StatusClass = "status-warning"
	StatusInfo    StatusClass = "status-info"
)

// RenderStatusMessage renders a status message span with predefined status class
func RenderStatusMessage(class StatusClass, message string) string {
	return fmt.Sprintf(`<span class="%s">%s</span>`, string(class), message)
}

// RenderSelectOptions renders option elements for select dropdown
func RenderSelectOptions(options []SelectOption, selectedValue string) string {
	var html string
	for _, option := range options {
		selected := ""
		if option.Value == selectedValue {
			selected = "selected"
		}
		html += fmt.Sprintf(`<option value="%s" %s>%s</option>`, option.Value, selected, option.Label)
	}
	return html
}

// RenderCheckbox renders a checkbox input with htmx attributes
func RenderCheckbox(name, endpoint string, checked bool, extraAttrs string) string {
	checkedAttr := ""
	if checked {
		checkedAttr = "checked"
	}

	baseAttrs := fmt.Sprintf(`type="checkbox" name="%s" %s hx-post="%s"`, name, checkedAttr, endpoint)
	if extraAttrs != "" {
		baseAttrs += " " + extraAttrs
	}

	return fmt.Sprintf(`<input %s />`, baseAttrs)
}

// RenderTextarea renders a textarea with specified attributes
func RenderTextarea(name, content string, rows int, extraAttrs string) string {
	baseAttrs := fmt.Sprintf(`name="%s" rows="%d"`, name, rows)
	if extraAttrs != "" {
		baseAttrs += " " + extraAttrs
	}
	return fmt.Sprintf(`<textarea %s>%s</textarea>`, baseAttrs, content)
}
