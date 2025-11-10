package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"knov/internal/dashboard"
	"knov/internal/files"
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

func parseFilterConfigFromForm(r *http.Request, widgetIndex int) *dashboard.FilterConfig {
	display := r.FormValue(fmt.Sprintf("widgets[%d][config][display]", widgetIndex))
	limitStr := r.FormValue(fmt.Sprintf("widgets[%d][config][limit]", widgetIndex))
	logic := r.FormValue(fmt.Sprintf("widgets[%d][config][logic]", widgetIndex))

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}
	if logic == "" {
		logic = "and"
	}
	if display == "" {
		display = "list"
	}

	// Parse criteria from form
	var criteria []files.FilterCriteria
	criteriaIndex := 0
	for {
		metadata := r.FormValue(fmt.Sprintf("widgets[%d][config][criteria][%d][metadata]", widgetIndex, criteriaIndex))
		if metadata == "" {
			break // no more criteria
		}

		operator := r.FormValue(fmt.Sprintf("widgets[%d][config][criteria][%d][operator]", widgetIndex, criteriaIndex))
		value := r.FormValue(fmt.Sprintf("widgets[%d][config][criteria][%d][value]", widgetIndex, criteriaIndex))
		action := r.FormValue(fmt.Sprintf("widgets[%d][config][criteria][%d][action]", widgetIndex, criteriaIndex))

		if operator == "" {
			operator = "equals"
		}
		if action == "" {
			action = "include"
		}

		if value != "" { // only add criteria with values
			criteria = append(criteria, files.FilterCriteria{
				Metadata: metadata,
				Operator: operator,
				Value:    value,
				Action:   action,
			})
		}

		criteriaIndex++
		if criteriaIndex > 50 { // safety limit
			break
		}
	}

	return &dashboard.FilterConfig{
		Display:  display,
		Limit:    limit,
		Logic:    logic,
		Criteria: criteria,
	}
}

func parseWidgetsFromForm(r *http.Request) ([]dashboard.Widget, error) {
	var widgets []dashboard.Widget
	form := r.PostForm

	// Find the highest widget index
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

	// Build widgets from form data
	for i := 0; i <= maxIndex; i++ {
		widgetType := dashboard.WidgetType(r.FormValue(fmt.Sprintf("widgets[%d][type]", i)))
		if widgetType == "" {
			continue // skip empty widget types
		}

		xPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][x]", i)))
		yPos, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("widgets[%d][position][y]", i)))

		// Build config from form fields
		var config dashboard.WidgetConfig
		switch widgetType {
		case dashboard.WidgetTypeFilter:
			config.Filter = parseFilterConfigFromForm(r, i)
		case dashboard.WidgetTypeFileContent:
			filePath := r.FormValue(fmt.Sprintf("widgets[%d][config][filePath]", i))
			config.FileContent = &dashboard.FileContentConfig{
				FilePath: filePath,
			}
		case dashboard.WidgetTypeStatic:
			format := r.FormValue(fmt.Sprintf("widgets[%d][config][format]", i))
			content := r.FormValue(fmt.Sprintf("widgets[%d][config][content]", i))
			config.Static = &dashboard.StaticConfig{
				Format:  format,
				Content: content,
			}
		}

		// fallback: try to parse JSON config if present
		configJSON := r.FormValue(fmt.Sprintf("widgets[%d][config]", i))
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

	return widgets, nil
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

	// Parse widgets from form data - handle both old and new format
	widgets, err := parseWidgetsFromForm(r)
	if err != nil {
		logging.LogError("failed to parse widgets: %v", err)
		http.Error(w, "failed to parse widgets", http.StatusBadRequest)
		return
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
	html := fmt.Sprintf(`<div class="success-message">dashboard created successfully! <a href="/dashboard/%s">view dashboard</a></div>`, dash.ID)
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

	widgets, err := parseWidgetsFromForm(r)
	if err != nil {
		logging.LogError("failed to parse widgets: %v", err)
		http.Error(w, "failed to parse widgets", http.StatusBadRequest)
		return
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

// @Summary Add filter criteria row
// @Description Add new filter criteria row for filter widget configuration
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param widget_index formData string true "Widget index"
// @Produce text/html
// @Success 200 {string} string "filter criteria row html"
// @Router /api/dashboards/filter-criteria [post]
func handleAPIFilterCriteria(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	widgetIndexStr := r.FormValue("widget_index")
	widgetIndex, err := strconv.Atoi(widgetIndexStr)
	if err != nil {
		http.Error(w, "invalid widget index", http.StatusBadRequest)
		return
	}

	// Generate a unique criteria index based on timestamp
	criteriaIndex := int(time.Now().Unix()) % 1000

	html := renderFilterCriteriaRow(widgetIndex, criteriaIndex, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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

// @Summary Get dashboard form
// @Description Get dashboard form for create or edit
// @Tags dashboards
// @Param id query string false "Dashboard ID for edit mode"
// @Produce text/html
// @Success 200 {string} string "dashboard form html"
// @Router /api/dashboards/form [get]
func handleAPIDashboardForm(w http.ResponseWriter, r *http.Request) {
	dashboardID := r.URL.Query().Get("id")
	var dash *dashboard.Dashboard
	var err error
	isEdit := dashboardID != ""

	if isEdit {
		dash, err = dashboard.Get(dashboardID)
		if err != nil {
			logging.LogError("failed to get dashboard %s: %v", dashboardID, err)
			http.Error(w, "dashboard not found", http.StatusNotFound)
			return
		}
	}

	var action, method string
	if isEdit {
		action = fmt.Sprintf("/api/dashboards/%s", dashboardID)
		method = "hx-patch"
	} else {
		action = "/api/dashboards"
		method = "hx-post"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<form %s="%s" hx-target="#dashboard-result" hx-swap="innerHTML" class="dashboard-form">`, method, action))

	// Dashboard Settings Section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(`<h4>dashboard settings</h4>`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label for="name">dashboard name</label>`)

	nameValue := ""
	if dash != nil {
		nameValue = dash.Name
	}
	html.WriteString(fmt.Sprintf(`<input type="text" id="name" name="name" required value="%s" class="form-input"/>`, nameValue))
	html.WriteString(`</div>`)

	// Layout and global checkbox
	html.WriteString(`<div class="form-row">`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label for="layout">layout</label>`)
	html.WriteString(`<select id="layout" name="layout" required class="form-select">`)

	layoutOptions := []string{"oneColumn", "twoColumns", "threeColumns", "fourColumns"}
	layoutIcons := []string{"📋", "📊", "🎯", "🎨"}
	selectedLayout := "twoColumns"
	if dash != nil {
		selectedLayout = string(dash.Layout)
	}

	for i, layout := range layoutOptions {
		selected := ""
		if layout == selectedLayout {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s %s</option>`, layout, selected, layoutIcons[i], layout))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// Global checkbox
	html.WriteString(`<div class="form-group checkbox-group">`)
	html.WriteString(`<label class="checkbox-label">`)
	globalChecked := ""
	if dash != nil && dash.Global {
		globalChecked = "checked"
	}
	html.WriteString(fmt.Sprintf(`<input type="checkbox" name="global" value="true" %s class="form-checkbox"/>`, globalChecked))
	html.WriteString(`<span class="checkmark"></span>`)
	html.WriteString(`global dashboard`)
	html.WriteString(`<small>visible to all users</small>`)
	html.WriteString(`</label>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Widgets Section
	html.WriteString(`<div class="form-section">`)
	html.WriteString(`<div class="section-header">`)
	html.WriteString(`<h4>widgets</h4>`)
	html.WriteString(`<button type="button" hx-post="/api/dashboards/widget-form" hx-target="#widgets-container" hx-swap="beforeend">+ add widget</button>`)
	html.WriteString(`</div>`)
	html.WriteString(`<div id="widgets-container">`)

	// Add existing widgets if editing
	if dash != nil && len(dash.Widgets) > 0 {
		for i, widget := range dash.Widgets {
			html.WriteString(renderWidgetForm(i, &widget))
		}
	} else {
		// Add one empty widget for new dashboard
		html.WriteString(renderWidgetForm(0, nil))
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Form actions
	html.WriteString(`<div class="form-actions">`)
	submitText := "🚀 create dashboard"
	if isEdit {
		submitText = "💾 save changes"
	}
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary"><span>%s</span></button>`, submitText))
	html.WriteString(`</div>`)
	html.WriteString(`</form>`)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html.String()))
}

// @Summary Get widget form
// @Description Get empty widget form for adding new widget
// @Tags dashboards
// @Produce text/html
// @Success 200 {string} string "widget form html"
// @Router /api/dashboards/widget-form [post]
func handleAPIWidgetForm(w http.ResponseWriter, r *http.Request) {
	// Get next available index by counting existing widgets
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	// Simple approach: use timestamp-based index to avoid conflicts
	index := int(time.Now().Unix()) % 1000

	html := renderWidgetForm(index, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func renderWidgetForm(index int, widget *dashboard.Widget) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="widget-form" data-widget-index="%d">`, index))
	html.WriteString(`<div class="widget-header">`)
	html.WriteString(fmt.Sprintf(`<span class="widget-number">widget %d</span>`, index+1))
	if index > 0 {
		html.WriteString(`<button type="button" onclick="this.closest('.widget-form').remove()" class="btn-remove">×</button>`)
	}
	html.WriteString(`</div>`)

	// Widget type selector
	html.WriteString(`<div class="form-row">`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label>type</label>`)
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][type]" required class="form-select" hx-post="/api/dashboards/widget-config" hx-target="#config-helper-%d" hx-include="closest .widget-form" hx-vals='{"index": "%d"}' hx-trigger="change">`, index, index, index))
	html.WriteString(`<option value="">choose type...</option>`)

	widgetTypes := []string{"filter", "filterForm", "fileContent", "static", "tags", "collections", "folders"}
	widgetIcons := []string{"🔍", "📝", "📄", "📌", "🏷️", "📚", "📁"}

	selectedType := ""
	if widget != nil {
		selectedType = string(widget.Type)
	}

	for i, wType := range widgetTypes {
		selected := ""
		if wType == selectedType {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s %s</option>`, wType, selected, widgetIcons[i], wType))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Position fields
	html.WriteString(`<div class="form-row">`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label>x position</label>`)
	xPos := "0"
	if widget != nil {
		xPos = fmt.Sprintf("%d", widget.Position.X)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][position][x]" min="0" max="3" value="%s" class="form-input"/>`, index, xPos))
	html.WriteString(`</div>`)
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label>y position</label>`)
	yPos := "0"
	if widget != nil {
		yPos = fmt.Sprintf("%d", widget.Position.Y)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][position][y]" min="0" value="%s" class="form-input"/>`, index, yPos))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Configuration section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<label>configuration</label>`)
	html.WriteString(fmt.Sprintf(`<div class="config-helper" id="config-helper-%d">`, index))

	if widget != nil && widget.Type != "" {
		html.WriteString(renderWidgetConfig(index, string(widget.Type), &widget.Config))
	} else {
		html.WriteString(`<div class="config-placeholder"><p>select a widget type to see configuration options</p></div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}

// @Summary Get widget configuration form
// @Description Get configuration form for specific widget type
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param index formData string true "Widget index"
// @Produce text/html
// @Success 200 {string} string "widget config html"
// @Router /api/dashboards/widget-config [post]
func handleAPIWidgetConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	indexStr := r.FormValue("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	// Get widget type from the select element that triggered this
	widgetType := r.FormValue(fmt.Sprintf("widgets[%d][type]", index))
	if widgetType == "" {
		// Empty type, show placeholder
		html := `<div class="config-placeholder"><p>select a widget type to see configuration options</p></div>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	html := renderWidgetConfig(index, widgetType, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func renderWidgetConfig(index int, widgetType string, config *dashboard.WidgetConfig) string {
	var html strings.Builder

	switch widgetType {
	case "filter":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(`<h5>filter configuration</h5>`)

		// Display and Logic settings
		html.WriteString(`<div class="config-section">`)
		html.WriteString(`<h6>display settings</h6>`)
		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>display:</label>`)
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][display]" class="form-select">`, index))

		displayOptions := []string{"list", "cards", "dropdown"}
		selectedDisplay := "list"
		if config != nil && config.Filter != nil {
			selectedDisplay = config.Filter.Display
		}

		for _, option := range displayOptions {
			selected := ""
			if option == selectedDisplay {
				selected = "selected"
			}
			html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
		}
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)

		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>limit:</label>`)
		limit := "10"
		if config != nil && config.Filter != nil && config.Filter.Limit > 0 {
			limit = fmt.Sprintf("%d", config.Filter.Limit)
		}
		html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][config][limit]" value="%s" min="1" class="form-input"/>`, index, limit))
		html.WriteString(`</div>`)

		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>logic:</label>`)
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][logic]" class="form-select">`, index))

		selectedLogic := "and"
		if config != nil && config.Filter != nil {
			selectedLogic = config.Filter.Logic
		}

		html.WriteString(fmt.Sprintf(`<option value="and" %s>and</option>`, ternary(selectedLogic == "and", "selected", "")))
		html.WriteString(fmt.Sprintf(`<option value="or" %s>or</option>`, ternary(selectedLogic == "or", "selected", "")))
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

		// Filter Criteria section
		html.WriteString(`<div class="config-section">`)
		html.WriteString(`<h6>filter criteria</h6>`)
		html.WriteString(fmt.Sprintf(`<div id="filter-criteria-container-%d">`, index))

		// Add existing criteria or one empty criteria
		if config != nil && config.Filter != nil && len(config.Filter.Criteria) > 0 {
			for i, criteria := range config.Filter.Criteria {
				html.WriteString(renderFilterCriteriaRow(index, i, &criteria))
			}
		} else {
			html.WriteString(renderFilterCriteriaRow(index, 0, nil))
		}

		html.WriteString(`</div>`)
		html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/dashboards/filter-criteria" hx-target="#filter-criteria-container-%d" hx-swap="beforeend" hx-vals='{"widget_index": "%d"}' class="btn-add-criteria">+ add criteria</button>`, index, index))
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

	case "fileContent":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(`<h5>file content configuration</h5>`)
		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>file:</label>`)
		selectID := fmt.Sprintf("file-selector-%d", index)
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][filePath]" id="%s" class="form-select" hx-get="/api/files/list?format=options" hx-target="#%s" hx-trigger="load" hx-swap="innerHTML">`, index, selectID, selectID))
		html.WriteString(`<option value="">loading files...</option>`)
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)
		html.WriteString(`<p class="config-note">select the file you want to display</p>`)
		html.WriteString(`</div>`)

	case "static":
		html.WriteString(`<div class="config-form">`)
		html.WriteString(`<h5>static content configuration</h5>`)
		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>format:</label>`)
		html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][format]" class="form-select">`, index))

		formatOptions := []string{"html", "markdown", "text"}
		selectedFormat := "html"
		if config != nil && config.Static != nil {
			selectedFormat = config.Static.Format
		}

		for _, option := range formatOptions {
			selected := ""
			if option == selectedFormat {
				selected = "selected"
			}
			html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
		}
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)
		html.WriteString(`<div class="config-row">`)
		html.WriteString(`<label>content:</label>`)

		content := "<h3>welcome!</h3><p>your static content here</p>"
		if config != nil && config.Static != nil {
			content = config.Static.Content
		}
		html.WriteString(fmt.Sprintf(`<textarea name="widgets[%d][config][content]" rows="3" class="form-textarea">%s</textarea>`, index, content))
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

	case "filterForm", "tags", "collections", "folders":
		widgetName := strings.Title(widgetType)
		html.WriteString(`<div class="config-form">`)
		html.WriteString(fmt.Sprintf(`<h5>%s widget configuration</h5>`, strings.ToLower(widgetName)))
		html.WriteString(`<p class="config-note">no configuration needed</p>`)
		html.WriteString(`</div>`)
	}

	return html.String()
}

func renderFilterCriteriaRow(widgetIndex, criteriaIndex int, criteria *files.FilterCriteria) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="filter-criteria-row" data-criteria-index="%d">`, criteriaIndex))

	// Metadata field selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(`<label>field:</label>`)
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][metadata]" class="form-select">`, widgetIndex, criteriaIndex))

	metadataOptions := []string{"collection", "tags", "type", "status", "priority", "createdAt", "lastEdited", "folders", "boards"}
	selectedMetadata := "collection"
	if criteria != nil {
		selectedMetadata = criteria.Metadata
	}

	for _, option := range metadataOptions {
		selected := ""
		if option == selectedMetadata {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// Operator selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(`<label>operator:</label>`)
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][operator]" class="form-select">`, widgetIndex, criteriaIndex))

	operatorOptions := []string{"equals", "contains", "greater", "less", "in"}
	selectedOperator := "equals"
	if criteria != nil {
		selectedOperator = criteria.Operator
	}

	for _, option := range operatorOptions {
		selected := ""
		if option == selectedOperator {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, option, selected, option))
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// Value input
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(`<label>value:</label>`)
	value := ""
	if criteria != nil {
		value = criteria.Value
	}
	html.WriteString(fmt.Sprintf(`<input type="text" name="widgets[%d][config][criteria][%d][value]" value="%s" placeholder="value" class="form-input"/>`, widgetIndex, criteriaIndex, value))
	html.WriteString(`</div>`)

	// Action selector
	html.WriteString(`<div class="filter-field-group">`)
	html.WriteString(`<label>action:</label>`)
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][criteria][%d][action]" class="form-select">`, widgetIndex, criteriaIndex))

	selectedAction := "include"
	if criteria != nil {
		selectedAction = criteria.Action
	}

	html.WriteString(fmt.Sprintf(`<option value="include" %s>include</option>`, ternary(selectedAction == "include", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="exclude" %s>exclude</option>`, ternary(selectedAction == "exclude", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// Remove button (if not the first criteria)
	if criteriaIndex > 0 {
		html.WriteString(`<button type="button" onclick="this.closest('.filter-criteria-row').remove()" class="btn-remove-criteria">×</button>`)
	}

	html.WriteString(`</div>`)

	return html.String()
}

func ternary(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}
