// Package testtheme ..
package main

import (
	"knov/internal/dashboard"
	"knov/internal/thememanager"
	"knov/themes/test/templates"

	"github.com/a-h/templ"
)

// TestTheme ..
type TestTheme struct{}

// Theme ..
var Theme TestTheme

var Metadata = thememanager.ThemeMetadata{
	AvailableFileViews:          []string{"default"},
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
	SupportsDarkMode:            false,
	AvailableColorSchemes:       []thememanager.ColorScheme{},
}

func (t *TestTheme) Home(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) Settings(viewName string) (templ.Component, error) {
	return templates.Settings(), nil
}

func (t *TestTheme) Admin(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) Playground(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) LatestChanges(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) History(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) Overview(viewName string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) Search(viewName string, query string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error) {
	return templates.Home(), nil
}

func (t *TestTheme) BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error) {
	return templates.Home(), nil
}
