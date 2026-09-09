package server

import (
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/server/render"
	"knov/internal/thememanager"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("Create New Dashboard")

	err := tm.Render(w, "dashboardnew", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dash, err := dashboard.Get(id)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard not found"))
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardEditTemplateData(dash)

	err = tm.Render(w, "dashboardedit", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleDashboardView(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = "home"
	}

	dash, err := dashboard.Get(id)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, "dashboard not found")
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardTemplateData(dash)

	err = tm.Render(w, "dashboardview", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleKanbanSelect(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewKanbanSelectTemplateData(configmanager.GetKanbanBoards())
	if err := tm.Render(w, "kanban", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleKanbanBoard(w http.ResponseWriter, r *http.Request) {
	board, ok := configmanager.GetKanbanBoardBySlug(chi.URLParam(r, "board"))
	if !ok {
		writeAPIError(w, r, http.StatusNotFound, "unknown board")
		return
	}
	tm := thememanager.GetThemeManager()
	filterPanel := render.RenderKanbanFilterPanel(board.Slug)
	data := thememanager.NewKanbanTemplateData(board, nil, filterPanel)
	if err := tm.Render(w, "kanban", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}
