// Package main ..
package main

import (
	"knov/internal/files"
	"knov/internal/thememanager"
	"knov/themes/test/templates"

	"github.com/a-h/templ"
)

type test struct{}

var Theme test

var Metadata = thememanager.ThemeMetadata{
	AvailableFileViews: []string{"normal"},
}

func (t *test) Home() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func (t *test) Settings() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Settings(td), nil
}

// Admin ...
func (t *test) Admin() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func (t *test) Playground() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func (t *test) LatestChanges() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func (t *test) History() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func (t *test) Overview() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

// Search ..
func (t *test) Search(query string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

// RenderFileView renders the specified file view
func (t *test) RenderFileView(_ string, fileContent *files.FileContent, filePath string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.FileView(fileContent, filePath, td), nil
}

func (t *test) BrowseFiles(metadataType string, value string, query string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.BrowseFiles(metadataType, value, query, td), nil
}

func (t *test) Dashboard(id string, action string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func main() {}
