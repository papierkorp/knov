package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"knov/internal/dashboards"
	"knov/internal/files"
)

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
	criteria, logic, err := files.ParseFilterCriteria(r)
	if err != nil {
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

	// Create widget if filter criteria exist
	if len(criteria) > 0 {
		dashboard.Widgets = append(dashboard.Widgets, dashboards.DashboardWidget{
			ID:       "main-filter",
			Type:     "file-filter",
			Position: map[string]interface{}{"x": 0, "y": 0},
			Config: map[string]interface{}{
				"filter":  criteria,
				"logic":   logic,
				"display": r.FormValue("display"),
			},
		})
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

	dashboard, err := dashboards.GetByID(id)
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
// @Accept json
// @Produce json,html
// @Param dashboard body dashboards.Dashboard true "Dashboard object"
// @Router /api/dashboards/{id} [put]
func handleAPIUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	if id == "" {
		http.Error(w, "missing dashboard id", http.StatusBadRequest)
		return
	}

	if !dashboards.Exists(id) {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	var dashboard dashboards.Dashboard
	if err := json.NewDecoder(r.Body).Decode(&dashboard); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if dashboard.ID != "" && dashboard.ID != id {
		http.Error(w, "cannot change dashboard id via update", http.StatusBadRequest)
		return
	}

	dashboard.ID = id
	if err := dashboards.Save(&dashboard); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	html := `<span class="status-ok">dashboard updated</span>`
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

	if !dashboards.Exists(id) {
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
