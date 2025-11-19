package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get all dashboards
// @Description Get list of all dashboards for current user
// @Tags dashboards
// @Param short query bool false "Show shortened dashboard names (3 chars max)"
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

	shortNames := r.URL.Query().Get("short") == "true"
	html := render.RenderDashboardsList(dashboards, shortNames)
	writeResponse(w, r, dashboards, html)
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
		title := r.FormValue(fmt.Sprintf("widgets[%d][title]", i))

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
			ID:    fmt.Sprintf("widget-%d", i),
			Type:  widgetType,
			Title: title,
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
// @Param widgets[0][title] formData string false "Widget title"
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

	data := translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard created")
	html := render.RenderDashboardCreated(dash.ID)
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

	html := render.RenderDashboardInfo(dash)
	writeResponse(w, r, dash, html)
}

// @Summary Update dashboard
// @Description Update existing dashboard
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param id path string true "Dashboard ID"
// @Param name formData string false "Dashboard name"
// @Param layout formData string false "Dashboard layout (oneColumn, twoColumns, threeColumns, fourColumns)"
// @Param global formData string false "Global dashboard"
// @Param widgets[0][type] formData string false "Widget type"
// @Param widgets[0][title] formData string false "Widget title"
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

	html := render.RenderDashboardUpdated(dash.ID)
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

	html := render.RenderFilterCriteriaRow(widgetIndex, criteriaIndex, nil)
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

	data := translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard deleted")
	html := render.RenderDashboardDeleted()
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

	html, err := render.RenderWidget(widget.Type, widget.Config)
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
	html := render.RenderDashboardRenamed()
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

	html := render.RenderDashboardForm(dash, isEdit)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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

	html := render.RenderWidgetForm(index, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get widget configuration form
// @Description Get configuration form for specific widget type
// @Tags dashboards
// @Accept application/x-www-form-urlencoded
// @Param index query string true "Widget index"
// @Param widgets[X][type] query string false "Widget type"
// @Produce text/html
// @Success 200 {string} string "widget config html"
// @Router /api/dashboards/widget-config [get]
// @Router /api/dashboards/widget-config [post]
func handleAPIWidgetConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	var indexStr, widgetType string

	if r.Method == "GET" {
		// Handle GET request with query parameters
		indexStr = r.URL.Query().Get("index")

		// Parse widget type from query parameters like widgets[0][type]
		for key, values := range r.URL.Query() {
			if strings.Contains(key, "][type]") && len(values) > 0 {
				widgetType = values[0]
				break
			}
		}
	} else {
		// Handle POST request with form data
		indexStr = r.FormValue("index")

		// Try to get widget type from form data
		if indexStr != "" {
			if index, err := strconv.Atoi(indexStr); err == nil {
				widgetType = r.FormValue(fmt.Sprintf("widgets[%d][type]", index))
			}
		}
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	if widgetType == "" {
		// empty type, show placeholder
		html := fmt.Sprintf(`<div class="config-placeholder"><p>%s</p></div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "select a widget type to see configuration options"))
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	html := render.RenderWidgetConfig(index, widgetType, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get filter value input HTML based on metadata field
// @Description Returns appropriate input HTML with datalist for filter value based on selected metadata field
// @Tags dashboards
// @Param widget_index formData string true "Widget index"
// @Param criteria_index formData string true "Criteria index"
// @Param widgets[X][config][criteria][Y][metadata] formData string true "Metadata field type"
// @Produce html
// @Success 200 {string} string "HTML input with datalist"
// @Router /api/dashboards/filter-value-input [get]
func handleAPIGetFilterValueInput(w http.ResponseWriter, r *http.Request) {
	widgetIndexStr := r.URL.Query().Get("widget_index")
	criteriaIndexStr := r.URL.Query().Get("criteria_index")

	widgetIndex, err := strconv.Atoi(widgetIndexStr)
	if err != nil {
		http.Error(w, "invalid widget_index", http.StatusBadRequest)
		return
	}

	criteriaIndex, err := strconv.Atoi(criteriaIndexStr)
	if err != nil {
		http.Error(w, "invalid criteria_index", http.StatusBadRequest)
		return
	}

	// get the metadata field from the select
	metadataField := r.URL.Query().Get(fmt.Sprintf("widgets[%d][config][criteria][%d][metadata]", widgetIndex, criteriaIndex))

	valueInputId := fmt.Sprintf("widget-%d-criteria-%d-value", widgetIndex, criteriaIndex)
	valueInputName := fmt.Sprintf("widgets[%d][config][criteria][%d][value]", widgetIndex, criteriaIndex)

	html := render.RenderFilterValueInput(valueInputId, valueInputName, "", metadataField)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get a new filter row for filterForm widget
// @Description Returns HTML for a new filter row to be added to filterForm widget
// @Tags dashboards
// @Produce html
// @Success 200 {string} string "HTML filter row"
// @Router /api/dashboards/filterform-row [post]
func handleAPIGetFilterFormRow(w http.ResponseWriter, r *http.Request) {
	// get current row count from request (if available) or use default
	// for simplicity, we'll use a timestamp-based index to ensure uniqueness
	index := int(time.Now().UnixNano() % 10000)

	html := render.RenderFilterFormRow(index)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get filter value input for filterForm widget
// @Description Returns appropriate input HTML with datalist for filterForm value based on selected metadata field
// @Tags dashboards
// @Param row_index formData string true "Row index"
// @Param metadata[] formData string true "Metadata field type"
// @Produce html
// @Success 200 {string} string "HTML input with datalist"
// @Router /api/dashboards/filterform-value-input [get]
func handleAPIGetFilterFormValueInput(w http.ResponseWriter, r *http.Request) {
	rowIndexStr := r.URL.Query().Get("row_index")

	rowIndex, err := strconv.Atoi(rowIndexStr)
	if err != nil {
		http.Error(w, "invalid row_index", http.StatusBadRequest)
		return
	}

	// get the metadata field from the select - note the array syntax
	metadataField := r.URL.Query().Get("metadata[]")

	html := render.RenderFilterFormValueInput(rowIndex, metadataField)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
