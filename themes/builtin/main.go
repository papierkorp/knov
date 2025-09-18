// Package defaulttheme ..
package main

import (
	"path/filepath"

	"github.com/a-h/templ"
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
