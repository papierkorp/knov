package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/search"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Search files
// @Tags search
// @Param q query string true "Search query"
// @Param format query string false "Output format: dropdown, list, cards, json" Enums(dropdown, list, cards, json)
// @Param titleonly query bool false "Search file titles only (no content)"
// @Param history query bool false "Search deleted files in git history"
// @Produce json,html
// @Router /api/search [get]
func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")
	titleOnly := r.URL.Query().Get("titleonly") == "true"
	history := r.URL.Query().Get("history") == "true"
	if format == "" {
		format = "dropdown"
	}

	if query == "" {
		writeResponse(w, r, []files.File{}, render.RenderSearchHint())
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

	// history search — returns git.GitHistoryFile results, rendered as list
	if history {
		var histResults []git.GitHistoryFile
		var err error
		if titleOnly {
			histResults, err = search.SearchDeletedFilesByTitle(query, limit)
		} else {
			histResults, err = search.SearchDeletedFilesByContent(query, limit)
		}
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "history search failed"), http.StatusInternalServerError)
			return
		}
		writeResponse(w, r, histResults, render.RenderSearchHistoryResults(histResults, query))
		return
	}

	var results []files.File
	var err error
	if titleOnly {
		results, err = search.SearchFilesByTitle(query, limit)
	} else {
		results, err = search.SearchFiles(query, limit)
	}
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "search failed"), http.StatusInternalServerError)
		return
	}

	var html string
	switch format {
	case "json":
	case "list":
		html = render.RenderSearchList(results, query)
	case "cards":
		html = render.RenderSearchCards(results, query)
	default:
		html = render.RenderSearchDropdown(results, query)
	}
	writeResponse(w, r, results, html)
}
