package thememanager

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/parser"
	"knov/internal/pathutils"
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
	FileType      string
	CodeBlockWrap bool
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
		CodeBlockWrap: configmanager.GetUserSettings().CodeBlockWrap,
		FileType:      "",
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
		"join": func(elems []string, sep string) string {
			return strings.Join(elems, sep)
		},
		// urlPath encodes a file path for use in href attributes.
		// Uses %20 for spaces (not + like urlQuery) so browsers resolve it correctly.
		"urlPath": func(s string) string {
			return pathutils.ToFileURL(s)
		},
		// urlPathSegment encodes a single path value for embedding in a URL path
		// e.g. href="/files/history/{{urlPathSegment .FilePath}}"
		"urlPathSegment": func(s string) string {
			s = strings.ReplaceAll(s, " ", "%20")
			s = strings.ReplaceAll(s, "#", "%23")
			s = strings.ReplaceAll(s, "?", "%3F")
			s = strings.ReplaceAll(s, "&", "%26")
			return s
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
	UserSettings         configmanager.UserSettings
	AppConfig            configmanager.AppConfig
}

// NewSettingsTemplateData creates settings-specific data
func NewSettingsTemplateData() SettingsTemplateData {
	return SettingsTemplateData{
		BaseTemplateData:     NewBaseTemplateData("Settings"),
		AvailableLanguages:   configmanager.GetAvailableLanguages(),
		AvailableThemes:      themeManager.GetAvailableThemes(),
		CurrentThemeSettings: getMergedThemeSettings(),
		ThemeSettingsSchema:  themeManager.GetCurrentThemeSettingsSchema(),
		UserSettings:         configmanager.GetUserSettings(),
		AppConfig:            configmanager.GetAppConfig(),
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
	baseData := NewBaseTemplateData(title)

	// detect file type using parser registry
	if filePath != "" {
		fullPath := pathutils.ToDocsPath(filePath)
		handler := parser.GetParserRegistry().GetHandler(fullPath)
		if handler != nil {
			baseData.FileType = handler.Name()
		}
	}

	return FileViewTemplateData{
		BaseTemplateData: baseData,
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
	FilePath  string
	SectionID string
}

// NewFileEditTemplateData creates file edit specific data
func NewFileEditTemplateData(filePath, sectionID string) FileEditTemplateData {
	title := "Edit File"
	if filePath != "" {
		title = "Edit: " + filePath
	}
	return FileEditTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
		SectionID:        sectionID,
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
// -------- browsemetadata TemplateData ---------
// -----------------------------------------------

// BrowseMetadataTemplateData extends base with metadata type browsing data
type BrowseMetadataTemplateData struct {
	BaseTemplateData
	MetadataType string
}

// NewBrowseMetadataTemplateData creates browse metadata specific data
func NewBrowseMetadataTemplateData(metadataType string) BrowseMetadataTemplateData {
	title := fmt.Sprintf("Browse: %s", metadataType)
	return BrowseMetadataTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		MetadataType:     metadataType,
	}
}

// -----------------------------------------------
// ---------- FileNew TemplateData --------------
// -----------------------------------------------

// FileNewTemplateData extends base with file creation specific data
type FileNewTemplateData struct {
	BaseTemplateData
	Editor string
}

// NewFileNewTemplateData creates file creation specific data
func NewFileNewTemplateData(editor string) FileNewTemplateData {
	return FileNewTemplateData{
		BaseTemplateData: NewBaseTemplateData("create new file"),
		Editor:           editor,
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

// -----------------------------------------------
// --------- FileEditTable TemplateData ---------
// -----------------------------------------------

// FileEditTableTemplateData extends base with table editor data
type FileEditTableTemplateData struct {
	BaseTemplateData
	FilePath   string
	TableIndex int
}

// NewFileEditTableTemplateData creates table editor template data
func NewFileEditTableTemplateData(filePath string, tableIndex int) FileEditTableTemplateData {
	title := "table editor: " + filePath

	return FileEditTableTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		FilePath:         filePath,
		TableIndex:       tableIndex,
	}
}

// -----------------------------------------------
// --------- Media Overview TemplateData --------
// -----------------------------------------------

// MediaOverviewTemplateData extends base with media overview data
type MediaOverviewTemplateData struct {
	BaseTemplateData
}

// NewMediaOverviewTemplateData creates media overview specific data
func NewMediaOverviewTemplateData() MediaOverviewTemplateData {
	return MediaOverviewTemplateData{
		BaseTemplateData: NewBaseTemplateData("media overview"),
	}
}

// -----------------------------------------------
// --------- Media View TemplateData ------------
// -----------------------------------------------

// MediaViewTemplateData extends base with media view data
type MediaViewTemplateData struct {
	BaseTemplateData
	MediaPath string
}

// NewMediaViewTemplateData creates media view specific data
func NewMediaViewTemplateData(mediaPath string) MediaViewTemplateData {
	title := "Media: " + mediaPath
	return MediaViewTemplateData{
		BaseTemplateData: NewBaseTemplateData(title),
		MediaPath:        mediaPath,
	}
}

// -----------------------------------------------
// --------- Filter ------------
// -----------------------------------------------

// FilterViewTemplateData extends base with filter view data
type FilterViewTemplateData struct {
	BaseTemplateData
	FilterID    string
	ResultsHTML string
}

// in NewFilterViewTemplateData, set FileType:
func NewFilterViewTemplateData(filterID, resultsHTML string) FilterViewTemplateData {
	base := NewBaseTemplateData("filter: " + filterID)
	base.FileType = "filter"
	return FilterViewTemplateData{
		BaseTemplateData: base,
		FilterID:         filterID,
		ResultsHTML:      resultsHTML,
	}
}

// FilterEditTemplateData extends base with filter edit data
type FilterEditTemplateData struct {
	BaseTemplateData
	FilterID string
	FilePath string // id + ".filter" for the editor API
}

func NewFilterEditTemplateData(filterID string) FilterEditTemplateData {
	return FilterEditTemplateData{
		BaseTemplateData: NewBaseTemplateData("edit filter: " + filterID),
		FilterID:         filterID,
		FilePath:         filterID + ".filter",
	}
}

// -----------------------------------------------
// ------------ Kanban TemplateData --------------
// -----------------------------------------------

// KanbanCard represents a single card on the kanban board
type KanbanCard struct {
	FilePath   string
	Title      string
	Collection string
	Status     string // the kanban status value (e.g. "inbox")
	Tags       []string
	CreatedAt  string
}

// KanbanColumn represents a single column on the board
type KanbanColumn struct {
	Status string
	Label  string
	Cards  []KanbanCard
}

// KanbanTemplateData extends base with kanban board data
type KanbanTemplateData struct {
	BaseTemplateData
	Collection string
	Columns    []KanbanColumn
	Statuses   []string // all possible statuses (for move target)
	Prefix     string   // kanban tag prefix
}

// NewKanbanTemplateData creates kanban board template data
func NewKanbanTemplateData(collection string, columns []KanbanColumn) KanbanTemplateData {
	return KanbanTemplateData{
		BaseTemplateData: NewBaseTemplateData("kanban: " + collection),
		Collection:       collection,
		Columns:          columns,
		Statuses:         configmanager.GetKanbanStatuses(),
		Prefix:           configmanager.GetKanbanPrefix(),
	}
}

// KanbanSelectTemplateData extends base with collection picker data
type KanbanSelectTemplateData struct {
	BaseTemplateData
	Collection  string // always empty — signals the template to show the picker
	Collections []string
}

// NewKanbanSelectTemplateData creates the collection picker template data
func NewKanbanSelectTemplateData(collections []string) KanbanSelectTemplateData {
	return KanbanSelectTemplateData{
		BaseTemplateData: NewBaseTemplateData("kanban"),
		Collections:      collections,
	}
}
