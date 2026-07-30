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
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard not found"), http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardEditTemplateData(dash)

	err = tm.Render(w, "dashboardedit", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
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
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardTemplateData(dash)

	err = tm.Render(w, "dashboardview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleKanbanSelect(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewKanbanSelectTemplateData(configmanager.GetKanbanBoards())
	if err := tm.Render(w, "kanban", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleKanbanBoard(w http.ResponseWriter, r *http.Request) {
	board, ok := configmanager.GetKanbanBoardBySlug(chi.URLParam(r, "board"))
	if !ok {
		http.Error(w, "unknown board", http.StatusNotFound)
		return
	}
	tm := thememanager.GetThemeManager()
	filterPanel := render.RenderKanbanFilterPanel(board.Slug)
	data := thememanager.NewKanbanTemplateData(board, nil, filterPanel)
	if err := tm.Render(w, "kanban", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}
