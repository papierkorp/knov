package thememanager

import (
	"encoding/json"
	"net/url"
	"text/template"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/translation"
)

// -----------------------------------------------
// -------------- Base TemplateData --------------
// -----------------------------------------------

// BaseTemplateData contains data needed by all templates
type BaseTemplateData struct {
	Title         string
	CurrentTheme  string
	ThemeSettings map[string]interface{}
	Language      string
	Themes        []Theme
	T             func(string, ...any) string
}

// NewBaseTemplateData creates base data used by all templates
func NewBaseTemplateData(title string) BaseTemplateData {
	return BaseTemplateData{
		Title:         title,
		CurrentTheme:  themeManager.GetCurrentThemeName(),
		ThemeSettings: getMergedThemeSettings(),
		Language:      configmanager.GetLanguage(),
		Themes:        themeManager.GetAvailableThemes(),
		T:             translation.Sprintf,
	}
}

// getMergedThemeSettings merges user settings with theme schema defaults
func getMergedThemeSettings() map[string]interface{} {
	userSettings := configmanager.GetCurrentThemeSettings()
	schema := themeManager.GetCurrentThemeSettingsSchema()

	merged := make(map[string]interface{})

	// first, add all defaults from schema
	for key, setting := range schema {
		merged[key] = setting.Default
	}

	// then override with user settings
	for key, value := range userSettings {
		merged[key] = value
	}

	return merged
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
		"add": func(a, b int) int {
			return a + b
		},
		"urlQuery": func(s string) string {
			return url.QueryEscape(s)
		},
		"marshalJSON": func(v interface{}) string {
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return "{}"
			}
			return string(data)
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				if i+1 < len(values) {
					dict[key] = values[i+1]
				}
			}
			return dict
		},
	}
}

// -----------------------------------------------
// ------------ Settings TemplateData ------------
// -----------------------------------------------

// SettingsTemplateData extends base with settings-specific data
type SettingsTemplateData struct {
	BaseTemplateData
	AvailableLanguages   []configmanager.Language
	AvailableThemes      []Theme
	CurrentThemeSettings map[string]interface{}
	ThemeSettingsSchema  map[string]ThemeSetting
}

// NewSettingsTemplateData creates settings-specific data
func NewSettingsTemplateData() SettingsTemplateData {
	return SettingsTemplateData{
		BaseTemplateData:     NewBaseTemplateData("Settings"),
		AvailableLanguages:   configmanager.GetAvailableLanguages(),
		AvailableThemes:      themeManager.GetAvailableThemes(),
		CurrentThemeSettings: getMergedThemeSettings(),
		ThemeSettingsSchema:  themeManager.GetCurrentThemeSettingsSchema(),
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
func NewFileViewTemplateData(title, filePath string, fileContent *files.FileContent) FileViewTemplateData {
	return FileViewTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
		FileContent:      fileContent,
	}
}

// -----------------------------------------------
// ---------- FileEdit TemplateData -------------
// -----------------------------------------------

// FileEditTemplateData extends base with file edit specific data
type FileEditTemplateData struct {
	BaseTemplateData
	FilePath string
}

// NewFileEditTemplateData creates file edit specific data
func NewFileEditTemplateData(filePath string) FileEditTemplateData {
	title := "Edit File"
	if filePath != "" {
		title = "Edit: " + filePath
	}
	return FileEditTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
	}
}

// -----------------------------------------------
// ---------- browsefiles TemplateData ----------
// -----------------------------------------------

// BrowseFilesTemplateData extends base with browse-specific data
type BrowseFilesTemplateData struct {
	BaseTemplateData
	MetadataType string
	Value        string
}

// NewBrowseFilesTemplateData creates browse files specific data
func NewBrowseFilesTemplateData(metadataType, value string) BrowseFilesTemplateData {
	return BrowseFilesTemplateData{
		BaseTemplateData: NewBaseTemplateData("Browse Files"),
		MetadataType:     metadataType,
		Value:            value,
	}
}

// -----------------------------------------------
// ---------- FileNew TemplateData --------------
// -----------------------------------------------

// FileNewTemplateData extends base with file creation specific data
type FileNewTemplateData struct {
	BaseTemplateData
	FileType string
}

// NewFileNewTemplateData creates file creation specific data
func NewFileNewTemplateData(fileType string) FileNewTemplateData {
	title := "Create New " + fileType
	return FileNewTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FileType:         fileType,
	}
}

// -----------------------------------------------
// ---------- Dashboard TemplateData -------------
// -----------------------------------------------

// DashboardTemplateData extends base with dashboard-specific data
type DashboardTemplateData struct {
	BaseTemplateData
	Dashboard *dashboard.Dashboard
}

// NewDashboardTemplateData creates dashboard view specific data
func NewDashboardTemplateData(dash *dashboard.Dashboard) DashboardTemplateData {
	title := "Dashboard"
	if dash != nil {
		title = dash.Name
	}
	return DashboardTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		Dashboard:        dash,
	}
}

// DashboardEditTemplateData extends base with dashboard edit specific data
type DashboardEditTemplateData struct {
	BaseTemplateData
	Dashboard *dashboard.Dashboard
}

// NewDashboardEditTemplateData creates dashboard edit specific data
func NewDashboardEditTemplateData(dash *dashboard.Dashboard) DashboardEditTemplateData {
	title := "Edit Dashboard"
	if dash != nil {
		title = "Edit Dashboard: " + dash.Name
	}
	return DashboardEditTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		Dashboard:        dash,
	}
}

// -----------------------------------------------
// ------------ Search TemplateData -------------
// -----------------------------------------------

// SearchPageData extends base with search-specific data
type SearchPageData struct {
	BaseTemplateData
	SearchQuery string
}

// NewSearchPageData creates search page specific data
func NewSearchPageData(searchQuery string) SearchPageData {
	return SearchPageData{
		BaseTemplateData: NewBaseTemplateData("Search"),
		SearchQuery:      searchQuery,
	}
}

// -----------------------------------------------
// ------------ History TemplateData ------------
// -----------------------------------------------

// HistoryTemplateData extends base with file history specific data
type HistoryTemplateData struct {
	BaseTemplateData
	FilePath        string
	CurrentVersion  string
	SelectedVersion string
	AllVersions     git.FileVersionList // FileVersion from git package
	ShowDiff        bool
	SingleVersion   bool // true if only one version exists
}

// NewHistoryTemplateData creates file history specific data
func NewHistoryTemplateData(filePath, currentVersion, selectedVersion string, allVersions git.FileVersionList, showDiff bool) HistoryTemplateData {
	title := "History"
	if filePath != "" {
		title = "History: " + filePath
	}

	// determine if this is a single version file
	singleVersion := len(allVersions) <= 1

	return HistoryTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
		CurrentVersion:   currentVersion,
		SelectedVersion:  selectedVersion,
		AllVersions:      allVersions,
		ShowDiff:         showDiff,
		SingleVersion:    singleVersion,
	}
}
