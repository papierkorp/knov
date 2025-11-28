// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"net/url"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderSearchHint renders an empty search hint message
func RenderSearchHint() string {
	return `<div class="search-hint">` + translation.SprintfForRequest(configmanager.GetLanguage(), "start typing to search...") + `</div>`
}

// RenderSearchDropdown creates dropdown HTML for file results with search features
func RenderSearchDropdown(results []files.File, query string) string {
	var html strings.Builder
	html.WriteString(`<ul class="component-search-dropdown-list">`)

	displayCount := 5
	for i, file := range results {
		if i >= displayCount {
			break
		}
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, file.Name))
	}

	if len(results) > displayCount {
		html.WriteString(fmt.Sprintf(`
			<li class="component-search-more-item">
								<a href="/search?q=%s" class="component-search-more-link">view all %d results →</a>
							</li>`,
			url.QueryEscape(query), len(results)))
	}

	if len(results) == 0 {
		html.WriteString(`<li class="component-search-hint">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no results found") + `</li>`)
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// RenderSearchCards creates cards HTML for file results with search context
func RenderSearchCards(results []files.File, query string) string {
	var html strings.Builder
	if query != "" {
		html.WriteString(fmt.Sprintf(`<p>%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "found %d results for \"%s\"", len(results), query)))
	}
	html.WriteString(RenderFileCards(results))
	return html.String()
}

// RenderSearchList creates simple list HTML for file results with search context
func RenderSearchList(results []files.File, query string) string {
	var html strings.Builder
	if query != "" {
		html.WriteString(fmt.Sprintf(`<p>%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "found %d results for \"%s\"", len(results), query)))
	}
	html.WriteString(RenderFileList(results))
	return html.String()
}
