// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"strings"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/translation"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------- Filter Editor -----------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterEditor renders a filter editor with form and result display
func RenderFilterEditor(filterID string) (string, error) {
	var html strings.Builder

	config, _ := filter.GetFilterConfig(filterID)

	html.WriteString(`<div class="filter-editor" id="filter-editor">`)
	html.WriteString(`<div class="filter-form-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter configuration") + `</h4>`)
	html.WriteString(RenderFilterForm(FilterFormOpts{
		Context:  FilterFormContextSave,
		Config:   config,
		FilterID: filterID,
		IsEdit:   filterID != "",
	}))
	html.WriteString(`<div id="editor-status"></div>`)
	html.WriteString(`</div>`)
	html.WriteString(`<div class="filter-results-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter preview") + `</h4>`)
	html.WriteString(`<button type="button" hx-post="/api/filters" hx-include="#filter-form" hx-target="#filter-results" class="btn-secondary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "preview results") + `</button>`)
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "configure filter above and click preview to see results") + `</p>`)
	html.WriteString(`</div></div></div>`)

	return html.String(), nil
}
