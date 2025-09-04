// Package main ..
package main

import (
	"github.com/a-h/templ"
	"knov/internal/thememanager"
	"knov/themes/billo/templates"
)

// Billo ..
type Billo struct{}

// Theme ..
var Theme Billo

// Home ...
func (t *Billo) Home() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

// Settings ...
func (t *Billo) Settings() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Settings(td), nil
}

// Playground ...
func (t *Billo) Playground() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Playground(td), nil
}

// LatestChanges ...
func (t *Billo) LatestChanges() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.LatestChanges(td), nil
}

// History ...
func (t *Billo) History() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.History(td), nil
}
func main() {}
