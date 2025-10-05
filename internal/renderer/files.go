// Package renderer handles rendering utilities
package renderer

import (
	"fmt"
	"strings"

	"knov/internal/files"
)

// BuildDropdownHTML creates dropdown HTML for file results
func BuildDropdownHTML(results []files.File, query string) string {
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
			query, len(results)))
	}

	if len(results) == 0 {
		html.WriteString(`<li class="component-search-hint">no results found</li>`)
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// BuildCardsHTML creates cards HTML for file results
func BuildCardsHTML(results []files.File, query string) string {
	var html strings.Builder
	if query != "" {
		html.WriteString(fmt.Sprintf(`<p>found %d results for "%s"</p>`, len(results), query))
	}
	html.WriteString(`<div class="search-results-cards">`)

	for _, file := range results {
		html.WriteString(fmt.Sprintf(`
			<div class="search-card">
				<h4><a href="/files/%s">%s</a></h4>
			</div>`,
			file.Path, file.Path))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// BuildListHTML creates simple list HTML for file results
func BuildListHTML(results []files.File, query string) string {
	var html strings.Builder
	if query != "" {
		html.WriteString(fmt.Sprintf(`<p>found %d results for "%s"</p>`, len(results), query))
	}
	html.WriteString(`<ul class="search-results-simple-list">`)

	for _, file := range results {
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, file.Path))
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// BuildBrowseHTML creates HTML list for metadata browsing with counts
func BuildBrowseHTML(items map[string]int, urlPrefix string) string {
	var html strings.Builder
	html.WriteString(`<ul class="search-results-simple-list">`)

	for item, count := range items {
		html.WriteString(fmt.Sprintf(`
			<li><a href="%s/%s">%s (%d)</a></li>`,
			urlPrefix, item, item, count))
	}

	html.WriteString(`</ul>`)
	return html.String()
}
