// Package defaulttheme ..
package main

import (
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

func main() {}
