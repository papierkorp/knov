// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"
)

// RenderBrowseHTML creates HTML list for metadata browsing with counts
func RenderBrowseHTML(items map[string]int, urlPrefix string) string {
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

// RenderMetadataLinksHTML creates HTML links for metadata items (tags, folders, collections)
func RenderMetadataLinksHTML(items []string, browseType string) string {
	if len(items) == 0 {
		return `<span class="meta-empty">-</span>`
	}

	var html strings.Builder
	for i, item := range items {
		if i > 0 {
			html.WriteString(", ")
		}
		html.WriteString(fmt.Sprintf(`<a href="/browse/%s/%s" class="meta-link">%s</a>`, browseType, item, item))
	}

	return html.String()
}

// RenderMetadataLinkHTML creates a single HTML link for metadata (e.g., collection)
func RenderMetadataLinkHTML(item string, browseType string) string {
	if item == "" {
		return `<span class="meta-empty">-</span>`
	}

	return fmt.Sprintf(`<a href="/browse/%s/%s" class="meta-link">%s</a>`, browseType, item, item)
}
