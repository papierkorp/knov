// Package main ..
package main

import (
	"github.com/a-h/templ"
	"knov/internal/thememanager"
	"knov/themes/billo/templates"
)

type Billo struct{}

var Theme Billo

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

func (t *Billo) Playground() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Playground(td), nil
}

func (t *Billo) LatestChanges() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.LatestChanges(td), nil
}

func (t *Billo) History() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.History(td), nil
}

func (t *Billo) Overview() (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.Overview(td), nil
}

func (t *Billo) FileView(content string, filename string) (templ.Component, error) {
	tm := thememanager.GetThemeManager()
	td := thememanager.TemplateData{
		ThemeToUse:      tm.GetCurrentThemeName(),
		AvailableThemes: tm.GetAvailableThemes(),
	}

	return templates.FileView(content, filename, td), nil
}

func main() {}
