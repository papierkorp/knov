// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------- Filter Editor -----------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterEditor renders a filter editor with form and result display
func RenderFilterEditor(filePath string) (string, error) {
	var html strings.Builder

	html.WriteString(`<div class="filter-editor" id="filter-editor">`)

	// load existing config if editing
	var config *filter.Config
	if filePath != "" {
		// for existing filter files, try to load the saved JSON
		fullPath := pathutils.ToDocsPath(filePath)
		if content, err := contentStorage.ReadFile(fullPath); err == nil {
			if len(content) == 0 {
				// use default configuration for empty files
				config = &filter.Config{
					Criteria: []filter.Criteria{},
					Logic:    "and",
					Display:  "list",
					Limit:    50,
				}
				logging.LogInfo("using default configuration for empty filter file in editor: %s", filePath)
			} else {
				config = &filter.Config{}
				if err := json.Unmarshal(content, config); err != nil {
					logging.LogError("failed to parse existing filter config: %v", err)
					config = nil
				}
			}
		}
	}

	// render the filter form with save functionality
	html.WriteString(`<div class="filter-form-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter configuration") + `</h4>`)

	// determine action - always use filter save endpoint for filter files
	isEdit := filePath != ""
	action := "/api/filter/save"
	includeFilePath := !isEdit

	// use the updated RenderFilterFormWithAction
	filterFormHTML := RenderFilterFormWithAction(config, action, filePath, includeFilePath)

	// modify the form to change button text and target
	applyFilterText := translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter")
	saveFilterText := translation.SprintfForRequest(configmanager.GetLanguage(), "save filter")

	if isEdit {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">`+applyFilterText, `class="btn-primary">`+saveFilterText, 1)
	} else {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">`+applyFilterText, `class="btn-primary">`+saveFilterText, 1)
	}

	html.WriteString(filterFormHTML)
	html.WriteString(`<div id="editor-status"></div>`)
	html.WriteString(`</div>`)

	// render results container with preview functionality
	html.WriteString(`<div class="filter-results-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter preview") + `</h4>`)
	html.WriteString(`<button type="button" hx-post="/api/filter" hx-include="#filter-form" hx-target="#filter-results" class="btn-secondary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "preview results") + `</button>`)
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "configure filter above and click preview to see results") + `</p>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	return html.String(), nil
}
