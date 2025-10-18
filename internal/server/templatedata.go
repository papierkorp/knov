package server

import (
	"html/template"
	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// TemplateData holds common data for all templates
type TemplateData struct {
	Title              string
	ThemeName          string
	DarkMode           bool
	ColorScheme        string
	Language           string
	T                  func(string, ...interface{}) string // Translation function
	View               string                              // Current view variant
	CustomCSSEditor    template.HTML
	AvailableLanguages []configmanager.Language
	CurrentLanguage    string
	GitRepoURL         string
	DataPath           string

	// Page-specific data
	Query        string               // For search page
	FileContent  *files.FileContent   // For file view
	FilePath     string               // For file view/edit
	Dashboard    *dashboard.Dashboard // For dashboard
	Mode         string               // For dashboard (view, edit, new)
	MetadataType string               // For browse files
	Value        string               // For browse files
}

// NewTemplateData creates base template data with common fields
func NewTemplateData(title string) *TemplateData {
	tm := thememanager.GetThemeManager()
	appConfig := configmanager.GetAppConfig()

	return &TemplateData{
		Title:              title,
		ThemeName:          tm.GetCurrentThemeName(),
		DarkMode:           configmanager.GetDarkMode(),
		ColorScheme:        configmanager.GetColorScheme(),
		Language:           configmanager.GetLanguage(),
		T:                  translation.Sprintf,
		AvailableLanguages: configmanager.GetAvailableLanguages(),
		CurrentLanguage:    configmanager.GetLanguage(),
		GitRepoURL:         appConfig.GitRepoURL,
		DataPath:           appConfig.DataPath,
	}
}

// SetFileData sets file-specific data
func (td *TemplateData) SetFileData(fileContent *files.FileContent, filePath string) *TemplateData {
	td.FileContent = fileContent
	td.FilePath = filePath
	return td
}

// SetDashboardData sets dashboard-specific data
func (td *TemplateData) SetDashboardData(dash *dashboard.Dashboard) *TemplateData {
	td.Dashboard = dash
	return td
}

// SetSearchData sets search-specific data
func (td *TemplateData) SetSearchData(query string) *TemplateData {
	td.Query = query
	return td
}

// SetBrowseFilesData sets browse files-specific data
func (td *TemplateData) SetBrowseFilesData(metadataType, value string) *TemplateData {
	td.MetadataType = metadataType
	td.Value = value
	return td
}

// SetView sets the view variant
func (td *TemplateData) SetView(view string) *TemplateData {
	td.View = view
	return td
}

// SetCustomCSSEditor sets the custom CSS editor HTML
func (td *TemplateData) SetCustomCSSEditor(editor template.HTML) *TemplateData {
	td.CustomCSSEditor = editor
	return td
}

// SetMode sets the page mode (e.g., "view", "edit", "new")
func (td *TemplateData) SetMode(mode string) *TemplateData {
	td.Mode = mode
	return td
}
