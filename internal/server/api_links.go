// Package server ..
package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"knov/internal/files"
	"knov/internal/utils"
)

// Helper function to create HTMX-enabled file links
func createFileLink(linkPath, filename string) string {
	return fmt.Sprintf(
		`<a href="/files/%s" hx-get="/files/%s" hx-target="#app-main" hx-push-url="true" hx-swap="innerHTML swap:0.2s" class="htmx-nav" title="%s">%s</a>`,
		linkPath, linkPath, linkPath, filename,
	)
}

// @Summary Get parent links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/parents [get]
func handleAPIGetParents(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := `<div class="component-no-links">no parents found</div>`
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Parents) == 0 {
		data := []string{}
		html := `<div class="component-no-links">no parents</div>`
		writeResponse(w, r, data, html)
		return
	}
	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, parent := range metadata.Parents {
		linkPath := utils.ToRelativePath(parent)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, createFileLink(linkPath, filename)))
	}
	html.WriteString(`</ul>`)
	writeResponse(w, r, metadata.Parents, html.String())
}

// @Summary Get ancestor links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/ancestors [get]
func handleAPIGetAncestors(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := `<div class="component-no-links">no ancestors found</div>`
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Ancestor) == 0 {
		data := []string{}
		html := `<div class="component-no-links">no ancestors</div>`
		writeResponse(w, r, data, html)
		return
	}
	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, ancestor := range metadata.Ancestor {
		linkPath := utils.ToRelativePath(ancestor)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, createFileLink(linkPath, filename)))
	}
	html.WriteString(`</ul>`)
	writeResponse(w, r, metadata.Ancestor, html.String())
}

// @Summary Get kids links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/kids [get]
func handleAPIGetKids(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := `<div class="component-no-links">no children found</div>`
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Kids) == 0 {
		data := []string{}
		html := `<div class="component-no-links">no children</div>`
		writeResponse(w, r, data, html)
		return
	}
	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, kid := range metadata.Kids {
		linkPath := utils.ToRelativePath(kid)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, createFileLink(linkPath, filename)))
	}
	html.WriteString(`</ul>`)
	writeResponse(w, r, metadata.Kids, html.String())
}

// @Summary Get used links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/used [get]
func handleAPIGetUsedLinks(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := `<div class="component-no-links">no outbound links found</div>`
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.UsedLinks) == 0 {
		data := []string{}
		html := `<div class="component-no-links">no outbound links</div>`
		writeResponse(w, r, data, html)
		return
	}
	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, usedLink := range metadata.UsedLinks {
		linkPath := utils.ToRelativePath(usedLink)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, createFileLink(linkPath, filename)))
	}
	html.WriteString(`</ul>`)
	writeResponse(w, r, metadata.UsedLinks, html.String())
}

// @Summary Get links to here for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/linkstohere [get]
func handleAPIGetLinksToHere(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := `<div class="component-no-links">no inbound links found</div>`
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.LinksToHere) == 0 {
		data := []string{}
		html := `<div class="component-no-links">no inbound links</div>`
		writeResponse(w, r, data, html)
		return
	}
	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, linkToHere := range metadata.LinksToHere {
		linkPath := utils.ToRelativePath(linkToHere)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, createFileLink(linkPath, filename)))
	}
	html.WriteString(`</ul>`)
	writeResponse(w, r, metadata.LinksToHere, html.String())
}
