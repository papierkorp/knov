// Package defaulttheme ..
package main

import (
	"github.com/a-h/templ"
	"knov/data/themes/dark/templates"
	"knov/internal/thememanager"
)

// Dark ..
type Dark struct{}

// Theme ..
var Theme Dark

// Home ...
func (t *Dark) Home() (templ.Component, error) {

	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Home(td), nil
}

func main() {}
