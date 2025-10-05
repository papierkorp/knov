// Package server ..
package server

import (
	"net/http"

	"knov/internal/files"
	"knov/internal/renderer"
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
		html := renderer.BuildDropdownHTML(results, query)
		w.Write([]byte(html))
	case "list":
		html := renderer.BuildListHTML(results, query)
		writeResponse(w, r, results, html)
	case "cards":
		html := renderer.BuildCardsHTML(results, query)
		writeResponse(w, r, results, html)
	default:
		html := renderer.BuildDropdownHTML(results, query)
		w.Write([]byte(html))
	}
}
