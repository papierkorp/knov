// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
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
		displayText := GetLinkDisplayText(file.Path)
		html.WriteString(fmt.Sprintf(`
		<li><a href="%s">%s</a></li>`, file.ViewURL(), displayText))
	}

	if len(results) > 0 {
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
	html.WriteString(RenderSearchResultsCards(results, query))
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

// RenderSearchResultsCards renders search results as clickable cards with context
func RenderSearchResultsCards(files []files.File, query string) string {
	var html strings.Builder
	html.WriteString(`<div id="search-results-cards">`)

	for _, file := range files {
		displayText := GetLinkDisplayText(file.Path)
		context := extractSearchContext(file.Path, query)

		html.WriteString(fmt.Sprintf(`
			<div class="search-result-card">
			<h4 class="search-result-title"><a href="%s">%s</a></h4>
				<div class="search-result-context">%s</div>
			</div>`,
			file.ViewURL(), displayText, context))
	}

	html.WriteString(`</div>`)
	return html.String()
}

func extractSearchContext(filePath, query string) string {
	if query == "" {
		return ""
	}

	fullPath := pathutils.ToDocsPath(filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		// file not on disk (e.g. virtual/filename match) - show label instead
		return fmt.Sprintf(`<span class="search-match-filename">%s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "filename match"))
	}

	originalContent := string(content)
	queryLower := strings.ToLower(query)
	words := strings.Fields(originalContent)

	hitWordIndex := -1
	for i, word := range words {
		if strings.Contains(strings.ToLower(word), queryLower) {
			hitWordIndex = i
			break
		}
	}

	if hitWordIndex == -1 {
		// query not in content but file matched by filename
		return fmt.Sprintf(`<span class="search-match-filename">%s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "filename match"))
	}

	start := hitWordIndex - 5
	if start < 0 {
		start = 0
	}
	end := hitWordIndex + 6
	if end > len(words) {
		end = len(words)
	}

	var contextParts []string

	if start < hitWordIndex {
		contextParts = append(contextParts, html.EscapeString(strings.Join(words[start:hitWordIndex], " ")))
	}

	hitWord := words[hitWordIndex]
	hitWordLower := strings.ToLower(hitWord)
	queryPos := strings.Index(hitWordLower, queryLower)
	if queryPos >= 0 {
		before := html.EscapeString(hitWord[:queryPos])
		match := html.EscapeString(hitWord[queryPos : queryPos+len(query)])
		after := html.EscapeString(hitWord[queryPos+len(query):])
		contextParts = append(contextParts, fmt.Sprintf(`%s<mark>%s</mark>%s`, before, match, after))
	} else {
		contextParts = append(contextParts, fmt.Sprintf(`<mark>%s</mark>`, html.EscapeString(hitWord)))
	}

	if hitWordIndex+1 < end {
		contextParts = append(contextParts, html.EscapeString(strings.Join(words[hitWordIndex+1:end], " ")))
	}

	context := strings.Join(contextParts, " ")
	if start > 0 {
		context = "..." + context
	}
	if end < len(words) {
		context = context + "..."
	}
	return context
}
