package server

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/thememanager"
)

// @Summary Get available file views for current theme
// @Tags config
// @Produce json,html
// @Router /api/config/getAvailableFileViews [get]
func handleAPIGetAvailableFileViews(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	views := tm.GetAvailableViews("file")
	currentView := configmanager.GetFileView()

	if !slices.Contains(views, currentView) && len(views) > 0 {
		currentView = views[0]
		configmanager.SetFileView(currentView)
	}

	var html strings.Builder
	for _, view := range views {
		selected := ""
		if view == currentView {
			selected = "selected"
		}
		displayName := strings.Title(view)
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, view, selected, displayName))
	}

	writeResponse(w, r, views, html.String())
}

// @Summary Set file view
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/setFileView [post]
func handleAPISetFileView(w http.ResponseWriter, r *http.Request) {
	view := r.FormValue("fileView")

	if view != "" {
		configmanager.SetFileView(view)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
