// Package server ..
package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/search"
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
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no children found"))
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.Kids) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no children"))
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderKidsLinks(metadata.Kids)
	writeResponse(w, r, metadata.Kids, html)
}

// @Summary Get grandchildren links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/grandchildren [get]
func handleAPIGetGrandchildren(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		writeResponse(w, r, []string{}, render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no grandchildren")))
		return
	}
	var grandchildren []string
	for _, kid := range metadata.Kids {
		kidMeta, err := files.MetaDataGet(kid)
		if err != nil || kidMeta == nil {
			continue
		}
		grandchildren = append(grandchildren, kidMeta.Kids...)
	}
	if len(grandchildren) == 0 {
		writeResponse(w, r, []string{}, render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no grandchildren")))
		return
	}
	writeResponse(w, r, grandchildren, render.RenderLinksList(grandchildren, false))
}

// @Summary Get used links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/used [get]
func handleAPIGetUsedLinks(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no outbound links found"))
		writeResponse(w, r, data, html)
		return
	}
	if len(metadata.UsedLinks) == 0 {
		data := []string{}
		html := render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no outbound links"))
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderUsedLinks(metadata.UsedLinks)
	writeResponse(w, r, metadata.UsedLinks, html)
}

// @Summary Get outbound media links for a file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/media [get]
func handleAPIGetMediaLinks(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil {
		data := []string{}
		html := render.RenderMediaLinks(data)
		writeResponse(w, r, data, html)
		return
	}
	html := render.RenderMediaLinks(metadata.UsedLinks)
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

// @Summary Get ancestor files within a folder
// @Description Returns unique ancestor paths for all files in the given folder (and its subfolders)
// @Tags links
// @Param folder query string true "Folder path"
// @Produce json,html
// @Success 200 {array} string
// @Failure 400 {string} string "missing folder parameter"
// @Failure 500 {string} string "failed to get ancestors"
// @Router /api/links/ancestors-in-folder [get]
func handleAPIGetAncestorsInFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("folder")
	if folderPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing folder parameter"), http.StatusBadRequest)
		return
	}
	ancestors, err := files.GetAncestorsInFolder(folderPath)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get ancestors"), http.StatusInternalServerError)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "options" {
		var html strings.Builder
		for _, a := range ancestors {
			rel := pathutils.ToRelative(a)
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, rel, rel)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
		return
	}

	if len(ancestors) == 0 {
		writeResponse(w, r, []string{}, render.RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no ancestors found")))
		return
	}
	writeResponse(w, r, ancestors, render.RenderLinksList(ancestors, false))
}

// @Description Returns files that share link neighbors with the given file
// @Tags links
// @Param filepath query string true "File path"
// @Produce json,html
// @Router /api/links/related [get]
func handleAPIGetRelatedFiles(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}
	paths, err := search.GetRelatedFiles(filePath, 5)
	if err != nil || len(paths) == 0 {
		writeResponse(w, r, []string{}, render.RenderRelatedFiles(nil))
		return
	}
	writeResponse(w, r, paths, render.RenderRelatedFiles(paths))
}

// @Summary Get live diff between a file and its conflict copy
// @Description Compares current file on disk with a .conflict.md copy using text diff
// @Tags links
// @Param filepath query string true "Original file path (docs/-prefixed)"
// @Param conflict query string true "Conflict file path (docs/-prefixed)"
// @Produce html
// @Success 200 {string} string "diff HTML"
// @Router /api/links/conflicts/diff [get]
func handleAPIGetConflictDiff(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	conflictPath := r.URL.Query().Get("conflict")
	if filePath == "" || conflictPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or conflict parameter"), http.StatusBadRequest)
		return
	}

	originalFull := pathutils.ToFullPath(pathutils.ToRelative(filePath))
	conflictFull := pathutils.ToFullPath(pathutils.ToRelative(conflictPath))

	html := render.RenderConflictDiff(originalFull, conflictFull)
	writeResponse(w, r, nil, html)
}

// @Summary Get conflict banner for a file
// @Description Returns a prominent warning banner if the file has unresolved conflicts
// @Tags links
// @Param filepath query string true "File path"
// @Produce html
// @Success 200 {string} string "banner HTML or empty"
// @Router /api/links/conflicts/banner [get]
func handleAPIGetConflictBanner(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeResponse(w, r, nil, "")
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil || metadata.ConflictFile == "" {
		writeResponse(w, r, nil, "")
		return
	}
	html := render.RenderConflictBanner(filePath, metadata.ConflictFile)
	writeResponse(w, r, nil, html)
}

// @Summary Get conflict-of banner for a conflict copy file
// @Description Returns a banner showing this file is a conflict copy, with diff link to original
// @Tags links
// @Param filepath query string true "Conflict file path (docs/-prefixed)"
// @Produce html
// @Success 200 {string} string "banner HTML or empty"
// @Router /api/links/conflicts/of-banner [get]
func handleAPIGetConflictOfBanner(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeResponse(w, r, nil, "")
		return
	}
	metadata, err := files.MetaDataGet(filePath)
	if err != nil || metadata == nil || metadata.ConflictOf == "" {
		writeResponse(w, r, nil, "")
		return
	}
	html := render.RenderConflictOfBanner(filePath, metadata.ConflictOf)
	writeResponse(w, r, nil, html)
}
