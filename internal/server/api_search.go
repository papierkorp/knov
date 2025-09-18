// Package server ..
package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/search"
)

// @Summary Search files
// @Tags search
// @Param q query string true "Search query"
// @Param format query string false "Output format: dropdown, list, cards, json" Enums(dropdown, list, cards, json)
// @Produce json,html
// @Router /api/search [get]
func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "dropdown"
	}

	if query == "" {
		emptyHTML := `<div class="search-hint">start typing to search...</div>`
		if format == "json" {
			writeResponse(w, r, []files.File{}, emptyHTML)
		} else {
			w.Write([]byte(emptyHTML))
		}
		return
	}

	limit := 6

	switch format {
	case "dropdown":
		limit = 6
	case "list":
		limit = 50
	case "cards":
		limit = 20
	case "json":
		limit = 100
	default:
		limit = 6
	}

	results, err := search.SearchFiles(query, limit)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		writeResponse(w, r, results, "")
	case "dropdown":
		html := buildDropdownHTML(results, query)
		w.Write([]byte(html))
	case "list":
		html := buildListHTML(results, query)
		writeResponse(w, r, results, html)
	case "cards":
		html := buildCardsHTML(results, query)
		writeResponse(w, r, results, html)
	default:
		html := buildDropdownHTML(results, query)
		w.Write([]byte(html))
	}
}

func buildDropdownHTML(results []files.File, query string) string {
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
		totalResults, _ := search.SearchFiles(query, 100)
		totalCount := len(totalResults)
		html.WriteString(fmt.Sprintf(`
			<li class="component-search-more-item">
				<a href="/search?q=%s" class="component-search-more-link">view all %d results →</a>
			</li>`,
			query, totalCount))
	}

	if len(results) == 0 {
		html.WriteString(`<li class="component-search-hint">no results found</li>`)
	}

	html.WriteString(`</ul>`)
	return html.String()
}

func buildCardsHTML(results []files.File, query string) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<p>found %d results for "%s"</p>`, len(results), query))
	html.WriteString(`<div class="search-results-cards">`)

	for _, file := range results {
		context := getSearchContext(file.Path, query, 30)
		html.WriteString(fmt.Sprintf(`
			<div class="search-card">
				<h4><a href="/files/%s">%s</a></h4>
				<p class="card-context">...%s...</p>
			</div>`,
			file.Path, file.Path, context))
	}

	html.WriteString(`</div>`)
	return html.String()
}

func buildListHTML(results []files.File, query string) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<p>found %d results for "%s"</p>`, len(results), query))
	html.WriteString(`<ul class="search-results-simple-list">`)

	for _, file := range results {
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, file.Path))
	}

	html.WriteString(`</ul>`)
	return html.String()
}
func getSearchContext(filePath, query string, contextLength int) string {
	dataDir := configmanager.GetAppConfig().DataPath
	fullPath := filepath.Join(dataDir, filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "content unavailable"
	}

	contentStr := strings.ToLower(string(content))
	queryLower := strings.ToLower(query)

	index := strings.Index(contentStr, queryLower)
	if index == -1 {
		return fmt.Sprintf("found in filename: %s", filepath.Base(filePath))
	}

	start := max(0, index-contextLength)
	end := min(len(content), index+len(query)+contextLength)

	context := string(content[start:end])
	context = strings.ReplaceAll(context, "\n", " ")
	context = strings.ReplaceAll(context, "\t", " ")

	return strings.TrimSpace(context)
}
