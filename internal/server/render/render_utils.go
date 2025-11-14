// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/files"
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

// RenderFileCards renders files as cards without search context
func RenderFileCards(files []files.File) string {
	var html strings.Builder
	html.WriteString(`<div class="search-results-cards">`)

	for _, file := range files {
		html.WriteString(fmt.Sprintf(`
			<div class="search-card">
				<h4><a href="/files/%s">%s</a></h4>
			</div>`,
			file.Path, file.Path))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderFileList renders files as simple list without search context
func RenderFileList(files []files.File) string {
	var html strings.Builder
	html.WriteString(`<ul class="search-results-simple-list">`)

	for _, file := range files {
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, file.Path))
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// RenderFileDropdown renders files as dropdown list with limit
func RenderFileDropdown(files []files.File, limit int) string {
	var html strings.Builder
	html.WriteString(`<ul class="component-search-dropdown-list">`)

	for i, file := range files {
		if i >= limit {
			break
		}
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, file.Name))
	}

	if len(files) == 0 {
		html.WriteString(`<li class="component-search-hint">no results found</li>`)
	}

	html.WriteString(`</ul>`)
	return html.String()
}
