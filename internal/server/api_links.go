// Package server ..
package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get parent links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/parents [get]
func handleAPIGetParents(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no parents found"))
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Parents) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no parents"))
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderLinksList(metadata.Parents, false)
	writeResponse(w, r, metadata.Parents, html)
}

// @Summary Get ancestor links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/ancestors [get]
func handleAPIGetAncestors(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage("no ancestors found")
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Ancestor) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage("no ancestors")
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderLinksList(metadata.Ancestor, false)
	writeResponse(w, r, metadata.Ancestor, html)
}

// @Summary Get kids links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/kids [get]
func handleAPIGetKids(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage("no children found")
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Kids) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage("no children")
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderLinksList(metadata.Kids, false)
	writeResponse(w, r, metadata.Kids, html)
}

// @Summary Get used links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Param showMedia query bool false "Include media file links"
// @Produce json,html
// @Router /api/links/used [get]
func handleAPIGetUsedLinks(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	showMedia := r.URL.Query().Get("showMedia") == "true"
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage("no outbound links found")
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.UsedLinks) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage("no outbound links")
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderUsedLinks(metadata.UsedLinks, showMedia)
	writeResponse(w, r, metadata.UsedLinks, html)
}

// @Summary Get links to here for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/linkstohere [get]
func handleAPIGetLinksToHere(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage("no inbound links found")
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.LinksToHere) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage("no inbound links")
		writeResponse(w, r, data, html)
		return
	}
	writeResponse(w, r, metadata.LinksToHere, render.RenderLinksList(metadata.LinksToHere, false))
}
