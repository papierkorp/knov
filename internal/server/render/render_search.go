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
	"knov/internal/git"
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

// extractSnippet returns an HTML snippet of originalContent around hitPos with
// a <mark> around the matched term of matchLen bytes.
func extractSnippet(originalContent, contentLower string, hitPos, matchLen int) string {
	const window = 60
	start := hitPos - window
	if start < 0 {
		start = 0
	}
	end := hitPos + matchLen + window
	if end > len(originalContent) {
		end = len(originalContent)
	}
	for start > 0 && originalContent[start] != ' ' && originalContent[start] != '\n' {
		start--
	}
	for end < len(originalContent) && originalContent[end] != ' ' && originalContent[end] != '\n' {
		end++
	}

	prefix := strings.Join(strings.Fields(strings.TrimSpace(originalContent[start:hitPos])), " ")
	match := originalContent[hitPos : hitPos+matchLen]
	suffix := strings.Join(strings.Fields(strings.TrimSpace(originalContent[hitPos+matchLen:end])), " ")

	var b strings.Builder
	if start > 0 {
		b.WriteString("...")
	}
	b.WriteString(html.EscapeString(prefix))
	if prefix != "" {
		b.WriteString(" ")
	}
	b.WriteString(fmt.Sprintf(`<mark>%s</mark>`, html.EscapeString(match)))
	if suffix != "" {
		b.WriteString(" ")
	}
	b.WriteString(html.EscapeString(suffix))
	if end < len(originalContent) {
		b.WriteString("...")
	}
	return b.String()
}

func extractSearchContext(filePath, query string) string {
	if query == "" {
		return ""
	}

	fullPath := pathutils.ToDocsPath(filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf(`<span class="search-match-filename">%s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "filename match"))
	}

	originalContent := string(content)
	contentLower := strings.ToLower(originalContent)
	queryLower := strings.ToLower(query)

	hitPos := strings.Index(contentLower, queryLower)
	if hitPos != -1 {
		return extractSnippet(originalContent, contentLower, hitPos, len(query))
	}

	// Phrase not found as a unit — collect one snippet per query word.
	words := strings.Fields(queryLower)
	var snippets []string
	for _, word := range words {
		pos := strings.Index(contentLower, word)
		if pos == -1 {
			continue
		}
		snippets = append(snippets, extractSnippet(originalContent, contentLower, pos, len(word)))
	}
	if len(snippets) == 0 {
		return fmt.Sprintf(`<span class="search-match-filename">%s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "content match"))
	}
	return strings.Join(snippets, ` <span class="search-snippet-sep">·</span> `)
}

// RenderSearchHistoryResults renders deleted-file history search results as HTML
func RenderSearchHistoryResults(results []git.GitHistoryFile, query string) string {
	var b strings.Builder
	if query != "" {
		fmt.Fprintf(&b, `<p>%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "found %d deleted files for \"%s\"", len(results), query))
	}
	if len(results) == 0 {
		fmt.Fprintf(&b, `<div class="search-hint">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no deleted files found"))
		return b.String()
	}
	b.WriteString(`<ul class="search-history-list">`)
	for _, f := range results {
		fmt.Fprintf(&b, `<li class="search-history-item"><a class="search-history-name" href="/files/history/%s?commit=%s">%s</a><span class="search-history-meta">%s &mdash; %s</span></li>`,
			f.Path,
			html.EscapeString(f.Commit),
			html.EscapeString(f.Name),
			html.EscapeString(configmanager.FormatDateTime(f.Date)),
			html.EscapeString(strings.TrimSpace(f.Message)),
		)
	}
	b.WriteString(`</ul>`)
	return b.String()
}
