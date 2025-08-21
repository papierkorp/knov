// Package defaulttheme ..
package main

import (
	"github.com/a-h/templ"
	"knov/internal/thememanager"
	"knov/themes/billo/templates"
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

func main() {}
