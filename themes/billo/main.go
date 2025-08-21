// Package defaulttheme ..
package main

import (
	"knov/internal/thememanager"
	"knov/themes/billo/templates"

	"github.com/a-h/templ"
)

// DefaultTheme ..
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

func (t *Billo) Settings() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}
	return templates.Settings(td), nil
}

func main() {}
