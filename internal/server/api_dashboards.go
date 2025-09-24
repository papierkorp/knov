package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"knov/internal/dashboard"
	"knov/internal/logging"
)

// @Summary Get all dashboards
// @Description Get list of all dashboards for current user
// @Tags dashboards
// @Produce json,html
// @Success 200 {array} dashboard.Dashboard
// @Router /api/dashboards [get]
func handleAPIGetDashboards(w http.ResponseWriter, r *http.Request) {
	dashboards, err := dashboard.GetAll()
	if err != nil {
		logging.LogError("failed to get dashboards: %v", err)
		http.Error(w, "failed to get dashboards", http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	for _, dash := range dashboards {
		html.WriteString(fmt.Sprintf(`<a href="/dashboard/%s">%s</a>`, dash.ID, dash.Name))
	}

	writeResponse(w, r, dashboards, html.String())
}

// @Summary Create new dashboard
// @Description Create a new dashboard
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param name formData string true "Dashboard name"
// @Param layout formData string true "Dashboard layout (oneColumn, twoColumns, threeColumns, fourColumns)"
// @Param global formData string false "Global dashboard (true/false)"
// @Success 200 {string} string "dashboard created"
// @Router /api/dashboards [post]
func handleAPICreateDashboard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	layout := dashboard.Layout(r.FormValue("layout"))
	globalStr := r.FormValue("global")

	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	global, _ := strconv.ParseBool(globalStr)

	dash := &dashboard.Dashboard{
		Name:   name,
		Layout: layout,
		Global: global,
	}

	if err := dashboard.Create(dash); err != nil {
		logging.LogError("failed to create dashboard: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := "dashboard created"
	html := `<div>dashboard created</div>`
	writeResponse(w, r, data, html)
}

// @Summary Get specific dashboard
// @Description Get dashboard by ID
// @Tags dashboards
// @Param id path string true "Dashboard ID"
// @Produce json,html
// @Success 200 {object} dashboard.Dashboard
// @Router /api/dashboards/{id} [get]
func handleAPIGetDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")

	dash, err := dashboard.Get(id)
	if err != nil {
		logging.LogError("failed to get dashboard %s: %v", id, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	html := fmt.Sprintf(`<div><h3>%s</h3><p>Layout: %s</p></div>`, dash.Name, dash.Layout)
	writeResponse(w, r, dash, html)
}

// @Summary Update dashboard
// @Description Update existing dashboard
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param id path string true "Dashboard ID"
// @Param name formData string true "Dashboard name"
// @Param layout formData string true "Dashboard layout"
// @Param global formData string false "Global dashboard"
// @Produce json,html
// @Success 200 {object} dashboard.Dashboard
// @Router /api/dashboards/{id} [patch]
func handleAPIUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	if name := r.FormValue("name"); name != "" {
		dash.Name = name
	}
	if layout := r.FormValue("layout"); layout != "" {
		dash.Layout = dashboard.Layout(layout)
	}
	if globalStr := r.FormValue("global"); globalStr != "" {
		global, _ := strconv.ParseBool(globalStr)
		dash.Global = global
	}

	if err := dashboard.Update(dash); err != nil {
		logging.LogError("failed to update dashboard: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	html := fmt.Sprintf(`<div>%s updated</div>`, dash.Name)
	writeResponse(w, r, dash, html)
}

// @Summary Delete dashboard
// @Description Delete dashboard by ID
// @Tags dashboards
// @Param id path string true "Dashboard ID"
// @Produce json,html
// @Success 200 {string} string "dashboard deleted"
// @Router /api/dashboards/{id} [delete]
func handleAPIDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")

	if err := dashboard.Delete(id); err != nil {
		logging.LogError("failed to delete dashboard %s: %v", id, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	data := "dashboard deleted"
	html := `<div>dashboard deleted</div>`
	writeResponse(w, r, data, html)
}
