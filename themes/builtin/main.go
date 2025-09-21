// Package defaulttheme ..
package main

import (
	"fmt"
	"path/filepath"

	"github.com/a-h/templ"
	"knov/internal/dashboards"
	"knov/internal/files"
	"knov/internal/server"
	"knov/internal/thememanager"
	"knov/themes/builtin/templates"
)

// Builtin ..
type Builtin struct{}

// Theme ..
var Theme Builtin

// Home ...
func (t *Builtin) Home() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

// Settings ...
func (t *Builtin) Settings() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Settings(td), nil
}

// Admin ...
func (t *Builtin) Admin() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Admin(td), nil
}

// Playground ...
func (t *Builtin) Playground() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Playground(td), nil
}

// LatestChanges ...
func (t *Builtin) LatestChanges() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.LatestChanges(td), nil
}

// History ...
func (t *Builtin) History() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.History(td), nil
}

// Overview ...
func (t *Builtin) Overview() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Overview(td), nil
}

// Search ..
func (t *Builtin) Search(query string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Search(query, td), nil
}

// GetAvailableFileViews returns all available file views for this theme
func (t *Builtin) GetAvailableFileViews() []string {
	return []string{"detailed", "compact", "minimal", "reader", "debug"}
}

// RenderFileView renders the specified file view
func (t *Builtin) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}
	filename := filepath.Base(filePath)

	switch viewName {
	case "detailed":
		return templates.FileViewDetailed(content, filePath, filename, td), nil
	case "compact":
		return templates.FileViewCompact(content, filePath, filename, td), nil
	case "minimal":
		return templates.FileViewMinimal(content, filePath, filename, td), nil
	case "reader":
		return templates.FileViewReader(content, filePath, filename, td), nil
	case "debug":
		return templates.FileViewDebug(content, filePath, filename, td), nil
	default:
		return templates.FileViewDetailed(content, filePath, filename, td), nil
	}
}

func (t *Builtin) Dashboard(id string) (templ.Component, error) {
	dashboard, err := dashboards.GetByID(id)
	if err != nil {
		allDashboards, _ := dashboards.GetAll()
		if len(allDashboards) == 0 && id == "home" {
			dashboard = &dashboards.Dashboard{
				ID: "home", Name: "Home", Layout: "single-column", Widgets: []dashboards.DashboardWidget{},
			}
		} else {
			return nil, fmt.Errorf("dashboard not found: %s", id)
		}
	}

	widgetContents := make(map[string]string)
	for _, widget := range dashboard.Widgets {
		if widget.Type == "file-filter" {
			widgetContents[widget.ID] = renderFilterWidget(widget)
		}
	}

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
		Dashboard:       dashboard,
		WidgetContents:  widgetContents,
	}

	return templates.Dashboard(td), nil
}

func renderFilterWidget(widget dashboards.DashboardWidget) string {
	criteria, logic, err := files.ParseFilterCriteriaFromConfig(widget.Config)
	if err != nil {
		return "<p>invalid filter config</p>"
	}

	display, _ := widget.Config["display"].(string)
	if display == "" {
		display = "list"
	}

	filteredFiles, err := files.FilterFilesByMetadata(criteria, logic)
	if err != nil {
		return "<p>filter error</p>"
	}

	if display == "cards" {
		return server.BuildCardsHTML(filteredFiles, "")
	}
	return server.BuildListHTML(filteredFiles, "")
}

// RenderForm renders various forms
func (t *Builtin) RenderForm(formType string, data interface{}) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	switch formType {
	case "dashboard-create":
		return templates.DashboardForm(td), nil
	default:
		return nil, fmt.Errorf("unknown form type: %s", formType)
	}
}
