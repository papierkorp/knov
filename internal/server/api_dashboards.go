// Package server ..
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"knov/internal/dashboards"
	"knov/internal/logging"
)

// checkDashboardExists returns dashboard and error if not found
func checkDashboardExists(id string) (*dashboards.Dashboard, error) {
	return dashboards.GetByID(id)
}

// @Summary Get all dashboards
// @Tags dashboards
// @Produce json,html
// @Router /api/dashboards [get]
func handleAPIGetDashboards(w http.ResponseWriter, r *http.Request) {
	allDashboards, err := dashboards.GetAll()
	if err != nil {
		http.Error(w, "failed to get dashboards", http.StatusInternalServerError)
		return
	}

	var html strings.Builder

	if len(allDashboards) == 0 {
		html.WriteString(`<a href="/">Home</a>`)
	} else {
		for _, dashboard := range allDashboards {
			displayText := dashboard.Name
			if len(displayText) > 3 {
				displayText = displayText[:3]
			}
			html.WriteString(fmt.Sprintf(`<a href="/dashboard/%s">%s</a>`, dashboard.ID, displayText))
		}
	}

	writeResponse(w, r, allDashboards, html.String())
}

// @Summary Create dashboard
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param id formData string true "Dashboard ID"
// @Param name formData string true "Dashboard name"
// @Param layout formData string false "Dashboard layout"
// @Router /api/dashboards [post]
func handleAPICreateDashboard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	dashboard := dashboards.Dashboard{
		ID:      r.FormValue("id"),
		Name:    r.FormValue("name"),
		Layout:  r.FormValue("layout"),
		Widgets: []dashboards.DashboardWidget{},
	}

	if dashboard.Layout == "" {
		dashboard.Layout = "single-column"
	}

	// parse widgets from form data
	for key, values := range r.Form {
		if strings.HasPrefix(key, "widget_") && len(values) > 0 {
			var widgetData struct {
				ID     string                 `json:"id"`
				Type   string                 `json:"type"`
				Column int                    `json:"column"`
				Config map[string]interface{} `json:"config"`
			}

			if err := json.Unmarshal([]byte(values[0]), &widgetData); err != nil {
				logging.LogWarning("failed to parse widget data: %v", err)
				continue
			}

			widget := dashboards.DashboardWidget{
				ID:       widgetData.ID,
				Type:     widgetData.Type,
				Position: map[string]interface{}{"x": widgetData.Column, "y": 0},
				Config:   widgetData.Config,
			}

			dashboard.Widgets = append(dashboard.Widgets, widget)
		}
	}

	if dashboard.ID == "" || dashboard.Name == "" {
		http.Error(w, "id and name are required", http.StatusBadRequest)
		return
	}

	if dashboards.Exists(dashboard.ID) {
		http.Error(w, "dashboard already exists", http.StatusConflict)
		return
	}

	if err := dashboards.Save(&dashboard); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	html := `<span class="status-ok">dashboard created</span>`
	writeResponse(w, r, dashboard, html)
}

// @Summary Get dashboard by ID
// @Tags dashboards
// @Param id path string true "Dashboard ID"
// @Produce json,html
// @Router /api/dashboards/{id} [get]
func handleAPIGetDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	if id == "" {
		http.Error(w, "missing dashboard id", http.StatusBadRequest)
		return
	}

	dashboard, err := checkDashboardExists(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	html := "<div>Dashboard: " + dashboard.Name + "</div>"
	writeResponse(w, r, dashboard, html)
}

// @Summary Update dashboard
// @Tags dashboards
// @Param id path string true "Dashboard ID"
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param name formData string false "Dashboard name"
// @Param layout formData string false "Dashboard layout"
// @Router /api/dashboards/{id} [patch]
func handleAPIUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	if id == "" {
		http.Error(w, "missing dashboard id", http.StatusBadRequest)
		return
	}

	dashboard, err := checkDashboardExists(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	// update only provided fields
	if name := r.FormValue("name"); name != "" {
		dashboard.Name = name
	}
	if layout := r.FormValue("layout"); layout != "" {
		dashboard.Layout = layout
	}

	// parse widgets from form data
	var newWidgets []dashboards.DashboardWidget
	for key, values := range r.Form {
		if strings.HasPrefix(key, "widget_") && len(values) > 0 {
			var widgetData struct {
				ID     string                 `json:"id"`
				Type   string                 `json:"type"`
				Column int                    `json:"column"`
				Config map[string]interface{} `json:"config"`
			}

			if err := json.Unmarshal([]byte(values[0]), &widgetData); err != nil {
				logging.LogWarning("failed to parse widget data: %v", err)
				continue
			}

			widget := dashboards.DashboardWidget{
				ID:       widgetData.ID,
				Type:     widgetData.Type,
				Position: map[string]interface{}{"x": widgetData.Column, "y": 0},
				Config:   widgetData.Config,
			}

			newWidgets = append(newWidgets, widget)
		}
	}

	// update widgets if any were provided
	if len(newWidgets) > 0 {
		dashboard.Widgets = newWidgets
	}

	if err := dashboards.Save(dashboard); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	html := `<span class="status-ok">dashboard updated</span>`
	writeResponse(w, r, dashboard, html)
}

// @Summary Rename dashboard
// @Tags dashboards
// @Param id path string true "Current dashboard ID"
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param new_id formData string true "New dashboard ID"
// @Router /api/dashboards/{id}/rename [patch]
func handleAPIRenameDashboard(w http.ResponseWriter, r *http.Request) {
	oldID := chi.URLParam(r, "id")

	if oldID == "" {
		http.Error(w, "missing dashboard id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	newID := r.FormValue("new_id")
	if newID == "" {
		http.Error(w, "new_id is required", http.StatusBadRequest)
		return
	}

	if newID == oldID {
		http.Error(w, "new id must be different", http.StatusBadRequest)
		return
	}

	dashboard, err := checkDashboardExists(oldID)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	if dashboards.Exists(newID) {
		http.Error(w, "new dashboard id already exists", http.StatusConflict)
		return
	}

	// create with new id
	dashboard.ID = newID
	if err := dashboards.Save(dashboard); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// delete old one
	if err := dashboards.Delete(oldID); err != nil {
		http.Error(w, "failed to delete old dashboard", http.StatusInternalServerError)
		return
	}

	html := `<span class="status-ok">dashboard renamed</span>`
	writeResponse(w, r, dashboard, html)
}

// @Summary Delete dashboard
// @Tags dashboards
// @Param id path string true "Dashboard ID"
// @Produce json,html
// @Router /api/dashboards/{id} [delete]
func handleAPIDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	if id == "" {
		http.Error(w, "missing dashboard id", http.StatusBadRequest)
		return
	}

	_, err := checkDashboardExists(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	if err := dashboards.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	html := `<span class="status-ok">dashboard deleted</span>`
	writeResponse(w, r, "deleted", html)
}
