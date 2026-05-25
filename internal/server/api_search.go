package server

import (
	"net/http"

	"knov/internal/files"
	"knov/internal/search"
	"knov/internal/server/render"
)

// @Summary Search files
// @Tags search
// @Param q query string true "Search query"
// @Param format query string false "Output format: dropdown, list, cards, json" Enums(dropdown, list, cards, json)
// @Param titleonly query bool false "Search file titles only (no content)"
// @Produce json,html
// @Router /api/search [get]
func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")
	titleOnly := r.URL.Query().Get("titleonly") == "true"
	if format == "" {
		format = "dropdown"
	}

	if query == "" {
		emptyHTML := render.RenderSearchHint()
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

	var results []files.File
	var err error
	if titleOnly {
		results, err = search.SearchFilesByTitle(query, limit)
	} else {
		results, err = search.SearchFiles(query, limit)
	}
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		writeResponse(w, r, results, "")
	case "dropdown":
		html := render.RenderSearchDropdown(results, query)
		w.Write([]byte(html))
	case "list":
		html := render.RenderSearchList(results, query)
		writeResponse(w, r, results, html)
	case "cards":
		html := render.RenderSearchCards(results, query)
		writeResponse(w, r, results, html)
	default:
		html := render.RenderSearchDropdown(results, query)
		w.Write([]byte(html))
	}
}
