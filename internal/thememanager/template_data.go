package thememanager

import (
	"encoding/json"
	"fmt"
	"html"
	htmltemplate "html/template"
	"net/url"
	"strings"
	"text/template"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/fonts"
	"knov/internal/git"
	"knov/internal/kanban"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
	"knov/internal/version"
)

// This file defines the data every theme template renders against. Each page
// handler builds one of these and hands it to ThemeManager.Render.
// BaseTemplateData carries what every page gets (title, theme, nav, ...);
// page-specific structs (e.g. KanbanData) carry the rest and are wrapped in
// PageData[T], exposed to templates as .Data. See docs/create_your_own_theme.md
// (the "Templates & Data" section) for the theme-author-facing explanation,
// and docs/template_data.md for the generated field reference.

// -----------------------------------------------
// -------------- Base TemplateData --------------
// -----------------------------------------------

// NavLink represents a single navigation link
type NavLink struct {
	Key   string
	URL   string
	Label string
}

// navLinkPool defines all available navigation links in display order.
// "none" is a sentinel that means "leave this slot empty".
var navLinkPool = []NavLink{
	{Key: "none", URL: "", Label: ""},
	{Key: "home", URL: "/", Label: "Home"},
	{Key: "overview", URL: "/browse/files", Label: "Overview"},
	{Key: "browse", URL: "/browse", Label: "Browse"},
	{Key: "media", URL: "/browse/media", Label: "Media"},
	{Key: "chat", URL: "/chat", Label: "Chat"},
	{Key: "history", URL: "/history", Label: "Latest Changes"},
	{Key: "help", URL: "/help", Label: "Help"},
	{Key: "search", URL: "/search", Label: "Search"},
	{Key: "settings", URL: "/settings", Label: "Settings"},
	{Key: "admin", URL: "/admin", Label: "Admin"},
	{Key: "kanban", URL: "/kanban", Label: "Kanban"},
	{Key: "playground", URL: "/playground", Label: "Playground"},
	{Key: "logs", URL: "/system/logs", Label: "Logs"},
	{Key: "changelog", URL: "/system/changelog", Label: "Changelog"},
	{Key: "version", URL: "/system/version", Label: "Version"},
}

// navLinkByKey returns a NavLink for the given key, or zero value if not found.
func navLinkByKey(key string) (NavLink, bool) {
	for _, l := range navLinkPool {
		if l.Key == key {
			return l, true
		}
	}
	return NavLink{}, false
}

// computeNavLinks splits the pool into header links (per settings) and menu links (the rest).
// Notifications is always appended to the menu by the template, so it is not in this pool.
func computeNavLinks(settings map[string]interface{}) (header []NavLink, menu []NavLink) {
	slotKeys := [5]string{}
	for i := range slotKeys {
		key := fmt.Sprintf("headerLink%d", i+1)
		if v, ok := settings[key].(string); ok && v != "" {
			slotKeys[i] = v
		}
	}

	inHeader := make(map[string]bool)
	for _, k := range slotKeys {
		if k != "" && k != "none" {
			inHeader[k] = true
			if link, ok := navLinkByKey(k); ok {
				header = append(header, link)
			}
		}
	}

	for _, link := range navLinkPool {
		if link.Key == "none" || inHeader[link.Key] {
			continue
		}
		menu = append(menu, link)
	}
	return header, menu
}

// BaseTemplateData contains data needed by all templates
type BaseTemplateData struct {
	Title             string                      // page title, shown in <title> and usable in templates
	CurrentTheme      string                      // slug of the active theme, e.g. "builtin"
	ThemeSettings     map[string]interface{}      // merged theme settings (schema defaults + user overrides), keyed by setting name
	Language          string                      // active UI language code
	DateFormat        string                      // user's configured date format
	Themes            []Theme                     // all installed themes, for the theme picker
	FileType          string                      // parser handler name for the current file (e.g. "markdown"), empty outside file pages
	CodeBlockWrap     bool                        // whether code blocks should soft-wrap
	T                 func(string, ...any) string // translation helper; prefer the "T" template func in templates
	Version           string                      // app build version
	BuildTime         string                      // app build timestamp
	SystemPage        bool                        // true on /system/* pages, for theme-specific styling
	HeaderNavLinks    []NavLink                   // user-configured links shown in the header
	MenuNavLinks      []NavLink                   // remaining nav links shown in the menu
	FontStyleTag      htmltemplate.HTML           // @font-face rules + font CSS vars; must be rendered in <head>
	ShowPDFFileButton bool                        // whether to show the "convert to PDF" button on file pages
	DarkMode          bool                        // user's dark mode setting; mirror as data-dark-mode on <body>
}

// NewBaseTemplateData creates base data used by all templates
func NewBaseTemplateData(title string) BaseTemplateData {
	themeSettings := getMergedThemeSettings()
	headerLinks, menuLinks := computeNavLinks(themeSettings)
	return BaseTemplateData{
		Title:             title,
		CurrentTheme:      themeManager.GetCurrentThemeName(),
		ThemeSettings:     themeSettings,
		Language:          configmanager.GetLanguage(),
		DateFormat:        configmanager.GetDateFormat(),
		Themes:            themeManager.GetAvailableThemes(),
		CodeBlockWrap:     configmanager.CodeBlockWrap.Get(),
		FileType:          "",
		T:                 translation.Sprintf,
		Version:           version.Version,
		BuildTime:         version.BuildTime,
		HeaderNavLinks:    headerLinks,
		MenuNavLinks:      menuLinks,
		FontStyleTag:      fontStyleTag(),
		ShowPDFFileButton: configmanager.PDFShowFileButton.Get(),
		DarkMode:          configmanager.DarkMode.Get(),
	}
}

// PageData wraps the data every page gets (BaseTemplateData) with the
// page-specific payload T, available to templates as .Data.
type PageData[T any] struct {
	BaseTemplateData
	Data T
}

// newPageData builds the base data for title and attaches the page payload.
func newPageData[T any](title string, data T) PageData[T] {
	return PageData[T]{
		BaseTemplateData: NewBaseTemplateData(title),
		Data:             data,
	}
}

// fontStyleTag renders the @font-face rules and CSS custom properties for
// the configured body/heading/code fonts, so any theme can pick them up via
// var(--font-body|--font-headings|--font-code) without generating its own
// @font-face rules.
func fontStyleTag() htmltemplate.HTML {
	body := configmanager.GetFontBody()
	headings := configmanager.GetFontHeadings()
	h1 := configmanager.GetFontH1()
	code := configmanager.GetFontCode()
	return htmltemplate.HTML(fmt.Sprintf(
		`<style>%s:root{--font-body:'%s',sans-serif;--font-headings:'%s',sans-serif;--font-h1:'%s',sans-serif;--font-code:'%s',monospace;}</style>`,
		fonts.FontFaceCSS(body, headings, h1, code), body, headings, h1, code,
	))
}

// getMergedThemeSettings merges user settings with theme schema defaults
func getMergedThemeSettings() map[string]interface{} {
	userSettings := configmanager.GetCurrentThemeSettings()
	schema := themeManager.GetCurrentThemeSettingsSchema()

	merged := make(map[string]interface{})

	// first, add all defaults from schema
	for key, setting := range schema {
		merged[key] = asTextareaValue(setting.Type, setting.Default)
	}

	// then override with user settings
	for key, value := range userSettings {
		merged[key] = asTextareaValue(schema[key].Type, value)
	}

	return merged
}

// asTextareaValue stringifies textarea settings so templates can treat them
// as opaque strings - a JSON-object default (e.g. railLayout) is unmarshaled
// from theme.json as a map/slice, not a string, unless re-marshaled here.
func asTextareaValue(settingType string, value interface{}) interface{} {
	if settingType != "textarea" {
		return value
	}
	if s, ok := value.(string); ok {
		return s
	}
	if b, err := json.Marshal(value); err == nil {
		return string(b)
	}
	return ""
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
		// htmlAttr escapes a string for safe embedding inside an HTML attribute
		// value. Theme templates use text/template (not html/template — themes
		// commonly embed pre-rendered trusted HTML from the render package,
		// which html/template would double-escape), so nothing auto-escapes;
		// any value that isn't from a closed, server-controlled set (an enum,
		// a boolean) needs this explicitly.
		"htmlAttr": func(s string) string {
			return html.EscapeString(s)
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

// SettingsData is the page payload for settings and admin pages.
type SettingsData struct {
	AvailableThemes      []Theme                 // installed themes, for the theme picker
	CurrentThemeSettings map[string]interface{}  // merged settings for the active theme, for the settings-editor form
	ThemeSettingsSchema  map[string]ThemeSetting // the active theme's declared setting definitions (type, default, options)
	AppConfig            configmanager.AppConfig // full app configuration, shown on the admin environment panel
	CustomFaviconExt     string                  // file extension of the user's uploaded custom favicon, if any
}

// NewSettingsTemplateData creates settings-specific data
func NewSettingsTemplateData() PageData[SettingsData] {
	return newPageData("Settings", SettingsData{
		AvailableThemes:      themeManager.GetAvailableThemes(),
		CurrentThemeSettings: getMergedThemeSettings(),
		ThemeSettingsSchema:  themeManager.GetCurrentThemeSettingsSchema(),
		AppConfig:            configmanager.GetAppConfig(),
		CustomFaviconExt:     configmanager.GetCustomFaviconExt(),
	})
}

// -----------------------------------------------
// ------------ FileView TemplateData ------------
// -----------------------------------------------

// FileViewData is the page payload for the file view page.
type FileViewData struct {
	FilePath    string             // relative path of the file being viewed
	FileContent *files.FileContent // rendered HTML + table of contents for the file
}

// NewFileViewTemplateData creates file view specific data
func NewFileViewTemplateData(title, filePath string, fileContent *files.FileContent) PageData[FileViewData] {
	data := newPageData(title, FileViewData{
		FilePath:    filePath,
		FileContent: fileContent,
	})

	// detect file type using parser registry
	if filePath != "" {
		fullPath := pathutils.ToDocsPath(filePath)
		handler := parser.GetParserRegistry().GetHandler(fullPath)
		if handler != nil {
			data.FileType = handler.Name()
		}
	}

	return data
}

// -----------------------------------------------
// ---------- FileEdit TemplateData -------------
// -----------------------------------------------

// FileEditData is the page payload for the file edit page.
type FileEditData struct {
	FilePath  string // relative path of the file being edited
	SectionID string // optional section anchor to edit directly (list/todo files)
}

// NewFileEditTemplateData creates file edit specific data
func NewFileEditTemplateData(filePath, sectionID string) PageData[FileEditData] {
	title := "Edit File"
	if filePath != "" {
		title = "Edit: " + filePath
	}
	return newPageData(title, FileEditData{
		FilePath:  filePath,
		SectionID: sectionID,
	})
}

// -----------------------------------------------
// ---------- browsefiles TemplateData ----------
// -----------------------------------------------

// BrowseFilesData is the page payload for the browse-files page.
type BrowseFilesData struct {
	MetadataType string // metadata dimension being browsed (collection, tag, folder, editor)
	Value        string // the specific metadata value being filtered on
}

// NewBrowseFilesTemplateData creates browse files specific data
func NewBrowseFilesTemplateData(metadataType, value string) PageData[BrowseFilesData] {
	return newPageData("Browse Files", BrowseFilesData{
		MetadataType: metadataType,
		Value:        value,
	})
}

// -----------------------------------------------
// -------- browsemetadata TemplateData ---------
// -----------------------------------------------

// BrowseMetadataData is the page payload for the browse-metadata page.
type BrowseMetadataData struct {
	MetadataType string // metadata dimension being browsed (collection, tag, folder, editor)
}

// NewBrowseMetadataTemplateData creates browse metadata specific data
func NewBrowseMetadataTemplateData(metadataType string) PageData[BrowseMetadataData] {
	title := fmt.Sprintf("Browse: %s", metadataType)
	return newPageData(title, BrowseMetadataData{
		MetadataType: metadataType,
	})
}

// -----------------------------------------------
// ---------- FileNew TemplateData --------------
// -----------------------------------------------

// FileNewData is the page payload for the file creation page.
type FileNewData struct {
	Editor      string // editor type to open (e.g. "codemirror-editor", "list-editor")
	PrefillPath string // pre-populates the file path input (e.g. a kanban board's folder)
}

// NewFileNewTemplateData creates file creation specific data
func NewFileNewTemplateData(editor string) PageData[FileNewData] {
	return newPageData("create new file", FileNewData{
		Editor: editor,
	})
}

// -----------------------------------------------
// ---------- Dashboard TemplateData -------------
// -----------------------------------------------

// DashboardData is the page payload for dashboard view and edit pages.
type DashboardData struct {
	Dashboard *dashboard.Dashboard // the dashboard being viewed or edited; nil if not found
}

// NewDashboardTemplateData creates dashboard view specific data
func NewDashboardTemplateData(dash *dashboard.Dashboard) PageData[DashboardData] {
	title := "Dashboard"
	if dash != nil {
		title = dash.Name
	}
	return newPageData(title, DashboardData{Dashboard: dash})
}

// NewDashboardEditTemplateData creates dashboard edit specific data
func NewDashboardEditTemplateData(dash *dashboard.Dashboard) PageData[DashboardData] {
	title := "Edit Dashboard"
	if dash != nil {
		title = "Edit Dashboard: " + dash.Name
	}
	return newPageData(title, DashboardData{Dashboard: dash})
}

// -----------------------------------------------
// ------------ Search TemplateData -------------
// -----------------------------------------------

// SearchData is the page payload for the search page.
type SearchData struct {
	SearchQuery string // the current search term, if any
}

// NewSearchPageData creates search page specific data
func NewSearchPageData(searchQuery string) PageData[SearchData] {
	return newPageData("Search", SearchData{SearchQuery: searchQuery})
}

// -----------------------------------------------
// ------------ History TemplateData ------------
// -----------------------------------------------

// HistoryData is the page payload for the file history page.
type HistoryData struct {
	FilePath        string              // relative path of the file; empty for the general "latest changes" view
	CurrentVersion  string              // the file's most recent commit hash
	SelectedVersion string              // commit hash selected for the diff view, if any
	AllVersions     git.FileVersionList // FileVersion from git package
	ShowDiff        bool                // whether the diff panel should render expanded
	SingleVersion   bool                // true if only one version exists
	FileDeleted     bool                // true if the file no longer exists on disk
	Collection      string              // optional collection filter for the general history view
	Folder          string              // optional folder filter (recursive) for the general history view, e.g. from a kanban board
	CompareFrom     string              // optional commit for an explicit from/to diff (arbitrary version pair, not just vs current)
	CompareTo       string              // optional commit for an explicit from/to diff
}

// NewHistoryTemplateData creates file history specific data
func NewHistoryTemplateData(filePath, currentVersion, selectedVersion string, allVersions git.FileVersionList, showDiff bool) PageData[HistoryData] {
	title := "History"
	if filePath != "" {
		title = "History: " + filePath
	}

	// determine if this is a single version file
	singleVersion := len(allVersions) <= 1

	return newPageData(title, HistoryData{
		FilePath:        filePath,
		CurrentVersion:  currentVersion,
		SelectedVersion: selectedVersion,
		AllVersions:     allVersions,
		ShowDiff:        showDiff,
		SingleVersion:   singleVersion,
	})
}

// -----------------------------------------------
// --------- FileEditTable TemplateData ---------
// -----------------------------------------------

// FileEditTableData is the page payload for the table editor page.
type FileEditTableData struct {
	FilePath   string // relative path of the file containing the table
	TableIndex int    // index of the table within the file being edited
}

// NewFileEditTableTemplateData creates table editor template data
func NewFileEditTableTemplateData(filePath string, tableIndex int) PageData[FileEditTableData] {
	title := "table editor: " + filePath

	return newPageData(title, FileEditTableData{
		FilePath:   filePath,
		TableIndex: tableIndex,
	})
}

// -----------------------------------------------
// --------- Media Overview TemplateData --------
// -----------------------------------------------

// NewMediaOverviewTemplateData creates media overview specific data
func NewMediaOverviewTemplateData() BaseTemplateData {
	return NewBaseTemplateData("media overview")
}

// -----------------------------------------------
// --------- Media View TemplateData ------------
// -----------------------------------------------

// MediaViewData is the page payload for the media view page.
type MediaViewData struct {
	MediaPath string // relative path of the media file, under data/media
}

// NewMediaViewTemplateData creates media view specific data
func NewMediaViewTemplateData(mediaPath string) PageData[MediaViewData] {
	title := "Media: " + mediaPath
	return newPageData(title, MediaViewData{MediaPath: mediaPath})
}

// -----------------------------------------------
// --------- Filter ------------
// -----------------------------------------------

// FilterViewData is the page payload for the filter view page.
type FilterViewData struct {
	FilterID    string // id of the filter being viewed
	ResultsHTML string // pre-rendered HTML of the filter's matching files
}

// NewFilterViewTemplateData creates filter view specific data
func NewFilterViewTemplateData(filterID, resultsHTML string) PageData[FilterViewData] {
	data := newPageData("filter: "+filterID, FilterViewData{
		FilterID:    filterID,
		ResultsHTML: resultsHTML,
	})
	data.FileType = "filter"
	return data
}

// FilterEditData is the page payload for the filter edit page.
type FilterEditData struct {
	FilterID string // id of the filter being edited
	FilePath string // id + ".filter" for the editor API
}

// NewFilterEditTemplateData creates filter edit specific data
func NewFilterEditTemplateData(filterID string) PageData[FilterEditData] {
	return newPageData("edit filter: "+filterID, FilterEditData{
		FilterID: filterID,
		FilePath: filterID + ".filter",
	})
}

// -----------------------------------------------
// ------------ Kanban TemplateData --------------
// -----------------------------------------------

// KanbanData is the page payload for a single kanban board.
type KanbanData struct {
	Board           string          // URL slug
	FolderPath      string          // configured folder path (for new-file prefill etc. and the board history link)
	DisplayName     string          // human-readable board name
	Columns         []kanban.Column // the board's columns and their cards
	Statuses        []string        // all possible statuses (for move target)
	Prefix          string          // kanban tag prefix
	FilterPanelHTML string          // pre-rendered advanced filter panel HTML
	ArchiveStatus   string          // status that hides cards from the board
}

// NewKanbanTemplateData creates kanban board template data
func NewKanbanTemplateData(board configmanager.KanbanBoard, columns []kanban.Column, filterPanelHTML string) PageData[KanbanData] {
	return newPageData("kanban: "+board.DisplayName, KanbanData{
		Board:           board.Slug,
		FolderPath:      board.FolderPath,
		DisplayName:     board.DisplayName,
		Columns:         columns,
		Statuses:        configmanager.GetKanbanStatuses(),
		Prefix:          configmanager.GetKanbanPrefix(),
		FilterPanelHTML: filterPanelHTML,
		ArchiveStatus:   configmanager.GetKanbanArchiveStatus(),
	})
}

// KanbanSelectData is the page payload for the board picker page.
type KanbanSelectData struct {
	Board         string                      // always empty — signals the template to show the picker
	Boards        []configmanager.KanbanBoard // configured kanban boards, for the picker
	ArchiveStatus string                      // status that hides cards from the board
}

// NewKanbanSelectTemplateData creates the board picker template data
func NewKanbanSelectTemplateData(boards []configmanager.KanbanBoard) PageData[KanbanSelectData] {
	return newPageData("kanban", KanbanSelectData{
		Boards:        boards,
		ArchiveStatus: configmanager.GetKanbanArchiveStatus(),
	})
}
