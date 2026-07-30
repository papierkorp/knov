package server

import (
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/logging"
	"knov/internal/thememanager"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	if id := configmanager.GetHomeDashboard(); id != "" {
		dash, err := dashboard.Get(id)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "home dashboard %q not found, falling back to home page: %v", id, err)
		} else {
			tm := thememanager.GetThemeManager()
			data := thememanager.NewDashboardTemplateData(dash)
			if err := tm.Render(w, "dashboardview", data); err != nil {
				http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("home")
	if err := tm.Render(w, "home", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewSettingsTemplateData()

	err := tm.Render(w, "settings", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewSettingsTemplateData()
	data.Title = "Admin"

	err := tm.Render(w, "admin", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleHelp(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("help")

	err := tm.Render(w, "help", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("playground")

	err := tm.Render(w, "playground", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("chat")
	if err := tm.Render(w, "chat", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}
