package server

import (
	"net/http"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/server/render"
	"knov/internal/thememanager"
)

// @Summary Get available file views for current theme
// @Tags config
// @Produce json,html
// @Router /api/config/fileviews [get]
func handleAPIGetAvailableFileViews(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	views := tm.GetAvailableViews("fileview")
	currentView := configmanager.GetFileView()

	if !slices.Contains(views, currentView) && len(views) > 0 {
		currentView = views[0]
		configmanager.SetFileView(currentView)
	}

	html := render.RenderFileViewOptions(views, currentView)

	writeResponse(w, r, views, html)
}

// @Summary Set file view
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/fileview [post]
func handleAPISetFileView(w http.ResponseWriter, r *http.Request) {
	view := r.FormValue("fileView")

	if view != "" {
		configmanager.SetFileView(view)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
