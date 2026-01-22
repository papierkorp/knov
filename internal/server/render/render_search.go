// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
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
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, displayText))
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
				<h4 class="search-result-title"><a href="/files/%s">%s</a></h4>
				<div class="search-result-context">%s</div>
			</div>`,
			file.Path, displayText, context))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// extractSearchContext extracts 5 words before and after the search hit
func extractSearchContext(filePath, query string) string {
	if query == "" {
		return ""
	}

	// try to get file content
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	originalContent := string(content)
	queryLower := strings.ToLower(query)

	// split into words
	words := strings.Fields(originalContent)

	// find word containing the search term (case insensitive)
	hitWordIndex := -1
	for i, word := range words {
		wordLower := strings.ToLower(word)
		if strings.Contains(wordLower, queryLower) {
			hitWordIndex = i
			break
		}
	}

	if hitWordIndex == -1 {
		return ""
	}

	// extract 5 words before and after
	start := hitWordIndex - 5
	if start < 0 {
		start = 0
	}

	end := hitWordIndex + 6 // +1 because end is exclusive, +5 for the 5 words after
	if end > len(words) {
		end = len(words)
	}

	if start >= end {
		return ""
	}

	// build context with highlighting
	var contextParts []string

	// before words
	if start < hitWordIndex {
		beforeWords := strings.Join(words[start:hitWordIndex], " ")
		if beforeWords != "" {
			contextParts = append(contextParts, beforeWords)
		}
	}

	// hit word with proper highlighting
	if hitWordIndex < len(words) {
		hitWord := words[hitWordIndex]
		// highlight the actual search term within the word (case insensitive)
		hitWordLower := strings.ToLower(hitWord)
		queryPos := strings.Index(hitWordLower, queryLower)

		if queryPos >= 0 {
			before := hitWord[:queryPos]
			match := hitWord[queryPos : queryPos+len(query)]
			after := hitWord[queryPos+len(query):]
			highlightedWord := fmt.Sprintf(`%s<mark>%s</mark>%s`, before, match, after)
			contextParts = append(contextParts, highlightedWord)
		} else {
			// fallback: highlight whole word
			highlightedWord := fmt.Sprintf(`<mark>%s</mark>`, hitWord)
			contextParts = append(contextParts, highlightedWord)
		}
	}

	// after words
	if hitWordIndex+1 < end {
		afterWords := strings.Join(words[hitWordIndex+1:end], " ")
		if afterWords != "" {
			contextParts = append(contextParts, afterWords)
		}
	}

	context := strings.Join(contextParts, " ")

	// add ellipsis if we're not at the beginning/end
	if start > 0 {
		context = "..." + context
	}
	if end < len(words) {
		context = context + "..."
	}

	return context
}
