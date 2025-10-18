package server

import (
	"encoding/json"
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
// @Description Create a new dashboard with optional widgets
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param name formData string true "Dashboard name"
// @Param layout formData string true "Dashboard layout (oneColumn, twoColumns, threeColumns, fourColumns)"
// @Param global formData string false "Global dashboard (true/false)"
// @Param widgets[0][type] formData string false "Widget type (filter, filterForm, fileContent, static, tags, collections, folders)"
// @Param widgets[0][position][x] formData int false "Widget X position"
// @Param widgets[0][position][y] formData int false "Widget Y position"
// @Param widgets[0][config] formData string false "Widget configuration JSON"
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

	// Parse widgets from form data
	var widgets []dashboard.Widget
	form := r.PostForm

	// Find the highest widget index
	maxIndex := -1
	for key := range form {
		if strings.HasPrefix(key, "widgets[") && strings.Contains(key, "][type]") {
			// Extract index from widgets[N][type]
			start := strings.Index(key, "[") + 1
			end := strings.Index(key[start:], "]")
			if end > 0 {
				if idx, err := strconv.Atoi(key[start : start+end]); err == nil && idx > maxIndex {
					maxIndex = idx
				}
			}
		}
	}

	// Build widgets from form data
	for i := 0; i <= maxIndex; i++ {
		widgetType := dashboard.WidgetType(r.FormValue(fmt.Sprintf("widgets[%d][type]", i)))
		if widgetType == "" {
			continue // skip empty widget types
		}

		xPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][x]", i)))
		yPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][y]", i)))
		configJSON := r.FormValue(fmt.Sprintf("widgets[%d][config]", i))

		var config dashboard.WidgetConfig
		if configJSON != "" {
			json.Unmarshal([]byte(configJSON), &config)
		}

		widget := dashboard.Widget{
			Type: widgetType,
			Position: dashboard.WidgetPosition{
				X: xPos,
				Y: yPos,
			},
			Config: config,
		}
		widgets = append(widgets, widget)
	}

	dash := &dashboard.Dashboard{
		Name:    name,
		Layout:  layout,
		Global:  global,
		Widgets: widgets,
	}

	if err := dashboard.Create(dash); err != nil {
		logging.LogError("failed to create dashboard: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := "dashboard created"
	html := `<div>dashboard created successfully</div>`
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
// @Param name formData string false "Dashboard name"
// @Param layout formData string false "Dashboard layout"
// @Param global formData string false "Global dashboard"
// @Param widgets[0][type] formData string false "Widget type"
// @Param widgets[0][position][x] formData int false "Widget X position"
// @Param widgets[0][position][y] formData int false "Widget Y position"
// @Param widgets[0][config] formData string false "Widget configuration JSON"
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
	} else {
		dash.Global = false
	}

	var widgets []dashboard.Widget
	form := r.PostForm

	maxIndex := -1
	for key := range form {
		if strings.HasPrefix(key, "widgets[") && strings.Contains(key, "][type]") {
			start := strings.Index(key, "[") + 1
			end := strings.Index(key[start:], "]")
			if end > 0 {
				if idx, err := strconv.Atoi(key[start : start+end]); err == nil && idx > maxIndex {
					maxIndex = idx
				}
			}
		}
	}

	for i := 0; i <= maxIndex; i++ {
		widgetType := dashboard.WidgetType(r.FormValue(fmt.Sprintf("widgets[%d][type]", i)))
		if widgetType == "" {
			continue
		}

		xPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][x]", i)))
		yPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][y]", i)))
		configJSON := r.FormValue(fmt.Sprintf("widgets[%d][config]", i))

		var config dashboard.WidgetConfig
		if configJSON != "" {
			json.Unmarshal([]byte(configJSON), &config)
		}

		widget := dashboard.Widget{
			ID:   fmt.Sprintf("widget-%d", i),
			Type: widgetType,
			Position: dashboard.WidgetPosition{
				X: xPos,
				Y: yPos,
			},
			Config: config,
		}
		widgets = append(widgets, widget)
	}

	if len(widgets) > 0 {
		dash.Widgets = widgets
	}

	if err := dashboard.Update(dash); err != nil {
		logging.LogError("failed to update dashboard: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	html := fmt.Sprintf(`<div class="success-message">dashboard updated successfully! <a href="/dashboard/%s">view dashboard</a></div>`, dash.ID)
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

// @Summary Render dashboard widget
// @Description Render a specific widget by ID from a dashboard
// @Tags widgets
// @Param dashboardId formData string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Accept application/x-www-form-urlencoded
// @Produce text/html
// @Success 200 {string} string "rendered widget html"
// @Failure 400 {string} string "missing parameters"
// @Failure 404 {string} string "widget not found"
// @Failure 500 {string} string "failed to render widget"
// @Router /api/dashboards/widget/{widgetId} [post]
func handleAPIRenderWidget(w http.ResponseWriter, r *http.Request) {
	widgetId := strings.TrimPrefix(r.URL.Path, "/api/dashboards/widget/")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	dashboardId := r.FormValue("dashboardId")
	if dashboardId == "" {
		http.Error(w, "dashboardId is required", http.StatusBadRequest)
		return
	}

	dash, err := dashboard.Get(dashboardId)
	if err != nil {
		logging.LogError("failed to get dashboard %s: %v", dashboardId, err)
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	// find widget by id
	var widget *dashboard.Widget
	for _, w := range dash.Widgets {
		if w.ID == widgetId {
			widget = &w
			break
		}
	}

	if widget == nil {
		http.Error(w, "widget not found", http.StatusNotFound)
		return
	}

	html, err := dashboard.RenderWidget(widget.Type, widget.Config)
	if err != nil {
		logging.LogError("failed to render widget %s: %v", widgetId, err)
		http.Error(w, "failed to render widget", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// internal/server/api_dashboards.go

// @Summary Rename dashboard
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param id path string true "dashboard id"
// @Param name formData string true "new dashboard name"
// @Produce json,html
// @Success 200 {string} string "dashboard renamed"
// @Router /api/dashboards/{id}/rename [post]
func handleAPIRenameDashboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	id = strings.TrimSuffix(id, "/rename")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	dash.Name = name
	if err := dashboard.Update(dash); err != nil {
		logging.LogError("failed to rename dashboard: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := "dashboard renamed"
	html := `<div>dashboard renamed successfully</div>`
	writeResponse(w, r, data, html)
}
