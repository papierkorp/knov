// Package main ..
package main

import (
	"path/filepath"

	"knov/internal/dashboard"
	"knov/internal/logging"
	"knov/internal/thememanager"
	"knov/themes/builtin/templates"

	"github.com/a-h/templ"
)

// Builtin ..
type Builtin struct{}

// Theme ..
var Theme Builtin

var Metadata = thememanager.ThemeMetadata{
	AvailableFileViews:          []string{"detailed", "compact", "minimal", "reader", "debug"},
	AvailableHomeViews:          []string{"default"},
	AvailableSearchViews:        []string{"default"},
	AvailableOverviewViews:      []string{"default"},
	AvailableDashboardViews:     []string{"default"},
	AvailableSettingsViews:      []string{"default"},
	AvailableAdminViews:         []string{"default"},
	AvailablePlaygroundViews:    []string{"default"},
	AvailableHistoryViews:       []string{"default"},
	AvailableLatestChangesViews: []string{"default"},
	AvailableBrowseFilesViews:   []string{"default"},
	SupportsDarkMode:            true,
	AvailableColorSchemes: []thememanager.ColorScheme{
		{
			Name:  "default",
			Label: "Default Blue",
			Colors: map[string]string{
				"primary": "#3b82f6",
				"accent":  "#8b5cf6",
				"neutral": "#64748b",
			},
		},
		{
			Name:  "green",
			Label: "Forest Green",
			Colors: map[string]string{
				"primary": "#65a30d",
				"accent":  "#a3e635",
				"neutral": "#475569",
			},
		},
		{
			Name:  "red",
			Label: "Ruby Red",
			Colors: map[string]string{
				"primary": "#dc2626",
				"accent":  "#f87171",
				"neutral": "#6b7280",
			},
		},
		{
			Name:  "purple",
			Label: "Royal Purple",
			Colors: map[string]string{
				"primary": "#a855f7",
				"accent":  "#c084fc",
				"neutral": "#64748b",
			},
		},
	},
}

// Home ...
func (t *Builtin) Home(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

// Settings ...
func (t *Builtin) Settings(viewName string) (templ.Component, error) {
	return templates.Settings(), nil
}

// Admin ...
func (t *Builtin) Admin(viewName string) (templ.Component, error) {
	return templates.Admin(), nil
}

// Playground ...
func (t *Builtin) Playground(viewName string) (templ.Component, error) {
	return templates.Playground(), nil
}

// LatestChanges ...
func (t *Builtin) LatestChanges(viewName string) (templ.Component, error) {
	return templates.LatestChanges(), nil
}

// History ...
func (t *Builtin) History(viewName string) (templ.Component, error) {
	return templates.History(), nil
}

// Overview ...
func (t *Builtin) Overview(viewName string) (templ.Component, error) {
	return templates.Overview(), nil
}

// Search ..
func (t *Builtin) Search(viewName string, query string) (templ.Component, error) {
	return templates.Search(query), nil
}

// RenderFileView renders the specified file view
func (t *Builtin) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
	filename := filepath.Base(filePath)

	switch viewName {
	case "detailed":
		return templates.FileViewDetailed(content, filePath, filename), nil
	case "compact":
		return templates.FileViewCompact(content, filePath, filename), nil
	case "minimal":
		return templates.FileViewMinimal(content, filePath, filename), nil
	case "reader":
		return templates.FileViewReader(content, filePath, filename), nil
	case "debug":
		return templates.FileViewDebug(content, filePath, filename), nil
	default:
		return templates.FileViewDetailed(content, filePath, filename), nil
	}
}

// Dashboard renders a specific dashboard by id
func (t *Builtin) Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error) {
	if action == "new" {
		return templates.DashboardNew(), nil
	}

	if action == "edit" {
		if dash == nil {
			logging.LogWarning("dashboard not found: %s", id)
			return t.Home("default")
		}
		return templates.DashboardEdit(dash), nil
	}

	if dash == nil {
		logging.LogWarning("dashboard not found: %s", id)
		return t.Home("default")
	}

	return templates.Dashboard(dash), nil
}

// BrowseFiles renders filtered file browse page
func (t *Builtin) BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error) {
	return templates.BrowseFiles(metadataType, value, query), nil
}
