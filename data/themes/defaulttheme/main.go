// Package defaulttheme ..
package main

import (
	"github.com/a-h/templ"
	"knov/data/themes/defaulttheme/templates"
	"knov/internal/thememanager"
)

// DefaultTheme ..
type DefaultTheme struct{}

// Theme ..
var Theme DefaultTheme

// Home ...
func (t *DefaultTheme) Home() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}
	return templates.Home(td), nil
}

func main() {}
