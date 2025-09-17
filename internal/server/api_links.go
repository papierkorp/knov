package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/files"
)

// @Summary Get parent links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/parents [get]
func handleAPIGetParents(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		data := []string{}
		html := "<li>no metadata found</li>"
		writeResponse(w, r, data, html)
		return
	}

	if len(metadata.Parents) == 0 {
		data := []string{}
		html := "<li>no parents</li>"
		writeResponse(w, r, data, html)
		return
	}

	var html strings.Builder
	for _, parent := range metadata.Parents {
		linkPath := strings.TrimPrefix(parent, "data/")
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`, linkPath, linkPath))
	}

	writeResponse(w, r, metadata.Parents, html.String())
}

// @Summary Get ancestor links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/ancestors [get]
func handleAPIGetAncestors(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		data := []string{}
		html := "<li>no metadata found</li>"
		writeResponse(w, r, data, html)
		return
	}

	if len(metadata.Ancestor) == 0 {
		data := []string{}
		html := "<li>no ancestors</li>"
		writeResponse(w, r, data, html)
		return
	}

	var html strings.Builder
	for _, ancestor := range metadata.Ancestor {
		linkPath := strings.TrimPrefix(ancestor, "data/")
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`, linkPath, linkPath))
	}

	writeResponse(w, r, metadata.Ancestor, html.String())
}

// @Summary Get kids links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/kids [get]
func handleAPIGetKids(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		data := []string{}
		html := "<li>no metadata found</li>"
		writeResponse(w, r, data, html)
		return
	}

	if len(metadata.Kids) == 0 {
		data := []string{}
		html := "<li>no kids</li>"
		writeResponse(w, r, data, html)
		return
	}

	var html strings.Builder
	for _, kid := range metadata.Kids {
		linkPath := strings.TrimPrefix(kid, "data/")
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`, linkPath, linkPath))
	}

	writeResponse(w, r, metadata.Kids, html.String())
}

// @Summary Get used links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/used [get]
func handleAPIGetUsedLinks(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		data := []string{}
		html := "<li>no metadata found</li>"
		writeResponse(w, r, data, html)
		return
	}

	if len(metadata.UsedLinks) == 0 {
		data := []string{}
		html := "<li>no used links</li>"
		writeResponse(w, r, data, html)
		return
	}

	var html strings.Builder
	for _, usedLink := range metadata.UsedLinks {
		linkPath := strings.TrimPrefix(usedLink, "data/")
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`, linkPath, linkPath))
	}

	writeResponse(w, r, metadata.UsedLinks, html.String())
}

// @Summary Get links to here for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/linkstohere [get]
func handleAPIGetLinksToHere(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil || metadata == nil {
		data := []string{}
		html := "<li>no metadata found</li>"
		writeResponse(w, r, data, html)
		return
	}

	if len(metadata.LinksToHere) == 0 {
		data := []string{}
		html := "<li>no links to here</li>"
		writeResponse(w, r, data, html)
		return
	}

	var html strings.Builder
	for _, linkToHere := range metadata.LinksToHere {
		linkPath := strings.TrimPrefix(linkToHere, "data/")
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`, linkPath, linkPath))
	}

	writeResponse(w, r, metadata.LinksToHere, html.String())
}
