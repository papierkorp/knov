package thememanager

import (
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
	"net/url"
	"text/template"
)

// BaseTemplateData contains data needed by all templates
type BaseTemplateData struct {
	Title        string
	CurrentTheme string
	DarkMode     bool
	ColorScheme  string
	Language     string
	Themes       []Theme
	T            func(string, ...interface{}) string
}

// SettingsTemplateData extends base with settings-specific data
type SettingsTemplateData struct {
	BaseTemplateData
	AvailableLanguages []configmanager.Language
	AvailableThemes    []Theme
}

// FileViewTemplateData extends base with file-specific data
type FileViewTemplateData struct {
	BaseTemplateData
	FilePath    string
	FileContent *files.FileContent
	ViewName    string
}

// NewBaseTemplateData creates base data used by all templates
func NewBaseTemplateData(title string) BaseTemplateData {
	return BaseTemplateData{
		Title:        title,
		CurrentTheme: themeManager.GetCurrentThemeName(),
		DarkMode:     configmanager.GetDarkMode(),
		ColorScheme:  configmanager.GetColorScheme(),
		Language:     configmanager.GetLanguage(),
		Themes:       themeManager.GetAvailableThemes(),
		T:            translation.Sprintf,
	}
}

// NewSettingsTemplateData creates settings-specific data
func NewSettingsTemplateData() SettingsTemplateData {
	return SettingsTemplateData{
		BaseTemplateData:   NewBaseTemplateData("Settings"),
		AvailableLanguages: configmanager.GetAvailableLanguages(),
		AvailableThemes:    themeManager.GetAvailableThemes(),
	}
}

// NewFileViewTemplateData creates file view specific data
func NewFileViewTemplateData(title, filePath string, fileContent *files.FileContent, viewName string) FileViewTemplateData {
	return FileViewTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
		FileContent:      fileContent,
		ViewName:         viewName,
	}
}

// CreateFuncMap creates template function map for HTML templates
func CreateFuncMap() template.FuncMap {
	return template.FuncMap{
		"T": translation.Sprintf,
		"mul": func(a, b int) int {
			return a * b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"urlQuery": func(s string) string {
			return url.QueryEscape(s)
		},
	}
}
