package thememanager

import (
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
	"net/url"
	"text/template"
)

// -----------------------------------------------
// -------------- Base TemplateData --------------
// -----------------------------------------------

// BaseTemplateData contains data needed by all templates
type BaseTemplateData struct {
	Title        string
	CurrentTheme string
	DarkMode     bool
	ColorScheme  string
	Language     string
	Themes       []Theme
	ViewName     string
	T            func(string, ...interface{}) string
}

// NewBaseTemplateData creates base data used by all templates
func NewBaseTemplateData(title, viewName string) BaseTemplateData {
	return BaseTemplateData{
		Title:        title,
		CurrentTheme: themeManager.GetCurrentThemeName(),
		DarkMode:     configmanager.GetDarkMode(),
		ColorScheme:  configmanager.GetColorScheme(),
		Language:     configmanager.GetLanguage(),
		Themes:       themeManager.GetAvailableThemes(),
		ViewName:     viewName,
		T:            translation.Sprintf,
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

// -----------------------------------------------
// ------------ Settings TemplateData ------------
// -----------------------------------------------

// SettingsTemplateData extends base with settings-specific data
type SettingsTemplateData struct {
	BaseTemplateData
	AvailableLanguages []configmanager.Language
	AvailableThemes    []Theme
}

// NewSettingsTemplateData creates settings-specific data
func NewSettingsTemplateData(viewName string) SettingsTemplateData {
	return SettingsTemplateData{
		BaseTemplateData:   NewBaseTemplateData("Settings", viewName),
		AvailableLanguages: configmanager.GetAvailableLanguages(),
		AvailableThemes:    themeManager.GetAvailableThemes(),
	}
}

// -----------------------------------------------
// ------------ FileView TemplateData ------------
// -----------------------------------------------

// FileViewTemplateData extends base with file-specific data
type FileViewTemplateData struct {
	BaseTemplateData
	FilePath    string
	FileContent *files.FileContent
}

// NewFileViewTemplateData creates file view specific data
func NewFileViewTemplateData(title, filePath string, fileContent *files.FileContent, viewName string) FileViewTemplateData {
	return FileViewTemplateData{
		BaseTemplateData: NewBaseTemplateData(title, viewName),
		FilePath:         filePath,
		FileContent:      fileContent,
	}
}
