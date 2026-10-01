package server

import (
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/export"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/thememanager"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	if id := configmanager.GetHomeDashboard(); id != "" {
		dash, err := dashboard.Get(id)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "home dashboard %q not found, falling back to help page: %v", id, err)
		} else {
			tm := thememanager.GetThemeManager()
			data := thememanager.NewDashboardTemplateData(dash)
			if err := tm.Render(w, "dashboardview", data); err != nil {
				writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
			}
			return
		}
	}

	handleHelp(w, r)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewSettingsTemplateData()

	err := tm.Render(w, "settings", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewSettingsTemplateData()
	data.Title = "Admin"
	// a running pdf export keeps polling after a page reload, otherwise show its archive
	if id := job.RunningExport(); id != "" {
		_, data.Data.ExportStatus = renderRunningJob(configmanager.GetLanguage(), id, job.JobTypeExport)
	} else if export.Available() {
		data.Data.ExportStatus = render.RenderExportDone(configmanager.GetLanguage())
	}

	err := tm.Render(w, "admin", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleHelp(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("help")

	err := tm.Render(w, "help", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("chat")
	if err := tm.Render(w, "chat", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}
