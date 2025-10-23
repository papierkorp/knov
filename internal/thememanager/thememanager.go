package thememanager

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/translation"
)

// Core required templates
var RequiredTemplates = []string{
	"base.gotmpl",
	"home.gotmpl",
	"fileview.gotmpl",
	"overview.gotmpl",
	"settings.gotmpl",
}

// Optional templates that fall back to builtin
var OptionalTemplates = []string{
	"admin.gotmpl",
	"playground.gotmpl",
	"history.gotmpl",
	"latestchanges.gotmpl",
	"search.gotmpl",
	"dashboard.gotmpl",
	"fileedit.gotmpl",
	"browsefiles.gotmpl",
}

var globalThemeManager *ThemeManager

type ThemeManager struct {
	themes       map[string]*Theme
	currentTheme string
	themesPath   string
}

type Theme struct {
	Name     string
	Path     string
	Metadata ThemeMetadata
	Template *template.Template
}

type ThemeMetadata struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Author      string         `json:"author"`
	Description string         `json:"description"`
	Categories  []string       `json:"categories,omitempty"` // e.g. ["minimal", "dark", "high-contrast"]
	Views       AvailableViews `json:"views"`
	Features    ThemeFeatures  `json:"features"`
}

type AvailableViews struct {
	File          []string `json:"file,omitempty"`
	Home          []string `json:"home,omitempty"`
	Search        []string `json:"search,omitempty"`
	Overview      []string `json:"overview,omitempty"`
	Dashboard     []string `json:"dashboard,omitempty"`
	Settings      []string `json:"settings,omitempty"`
	Admin         []string `json:"admin,omitempty"`
	Playground    []string `json:"playground,omitempty"`
	History       []string `json:"history,omitempty"`
	LatestChanges []string `json:"latestchanges,omitempty"`
	BrowseFiles   []string `json:"browsefiles,omitempty"`
}

type ThemeFeatures struct {
	DarkMode      bool          `json:"dark_mode"`
	ColorSchemes  []ColorScheme `json:"color_schemes,omitempty"`
	ResponsiveCSS bool          `json:"responsive_css"`
	CustomFonts   bool          `json:"custom_fonts"`
}

type ColorScheme struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// Predefined theme categories
var PredefinedCategories = []string{
	"minimal",       // Simple, clean designs
	"dark",          // Dark mode themes
	"light",         // Light mode themes
	"high-contrast", // Accessibility focused
	"colorful",      // Rich color palettes
	"monochrome",    // Single color schemes
	"compact",       // Space-efficient layouts
	"spacious",      // Generous whitespace
	"modern",        // Contemporary design
	"classic",       // Traditional layouts
}

// Clean template data structure
type TemplateData struct {
	Title    string
	Theme    string
	Language string
	DarkMode bool
	Content  TemplateContent
}

// Union type for all possible template content
type TemplateContent struct {
	Home          *HomeContent
	FileView      *FileViewContent
	Overview      *OverviewContent
	Settings      *SettingsContent
	Dashboard     *DashboardContent
	Search        *SearchContent
	Admin         *AdminContent
	Playground    *PlaygroundContent
	History       *HistoryContent
	LatestChanges *LatestChangesContent
	BrowseFiles   *BrowseFilesContent
	FileEdit      *FileEditContent
}

// Specific content types for each view
type HomeContent struct {
	Title        string
	WelcomeText  string
	QuickActions []QuickAction
}

type FileViewContent struct {
	Title       string
	FilePath    string
	FileContent string // Changed from interface{} to string
	ViewName    string
}

type OverviewContent struct {
	Title     string
	FileCount int
	ViewName  string
}

type SettingsContent struct {
	Title          string
	CurrentTheme   string
	CurrentLang    string
	AvailableViews map[string][]string
}

type DashboardContent struct {
	Title     string
	ID        string
	Action    string
	Dashboard string // Changed from interface{} to string for simplicity
}

type SearchContent struct {
	Title   string
	Query   string
	Results []SearchResult // Changed from interface{} to specific type
}

type SearchResult struct {
	Title   string
	Path    string
	Content string
	Score   float64
}

type AdminContent struct {
	Title      string
	SystemInfo SystemInfo
	ThemeList  []string
}

type PlaygroundContent struct {
	Title string
}

type HistoryContent struct {
	Title   string
	Commits []CommitInfo // Changed from interface{} to specific type
}

type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

type LatestChangesContent struct {
	Title   string
	Changes []FileChange // Changed from interface{} to specific type
}

type FileChange struct {
	FilePath string
	Status   string
	Date     string
}

type BrowseFilesContent struct {
	Title        string
	MetadataType string
	Value        string
	Files        []FileInfo // Changed from interface{} to specific type
}

type FileInfo struct {
	Name string
	Path string
	Size int64
	Date string
}

type FileEditContent struct {
	Title    string
	FilePath string
	Content  string
}

type QuickAction struct {
	Name string
	URL  string
	Icon string
}

type SystemInfo struct {
	Version    string
	ThemeCount int
	FileCount  int
}

// Init initializes the global theme manager
func Init() {
	logging.LogInfo("initialize thememanager ...")

	themesPath := configmanager.GetThemesPath()
	globalThemeManager = NewThemeManager(themesPath)

	// Extract builtin theme if missing
	if err := initBuiltinTheme(themesPath); err != nil {
		logging.LogError("failed to extract builtin theme: %v", err)
	}

	if err := globalThemeManager.LoadThemes(); err != nil {
		logging.LogError("failed to load themes: %v", err)
	}

	// Set configured theme or default to builtin
	currentTheme := configmanager.GetTheme()
	if currentTheme == "" || !globalThemeManager.hasTheme(currentTheme) {
		currentTheme = "builtin"
		configmanager.SetTheme(currentTheme)
	}
	globalThemeManager.SetTheme(currentTheme)
	logging.LogInfo("theme loaded successfully")
}

func GetThemeManager() *ThemeManager {
	return globalThemeManager
}

func NewThemeManager(themesPath string) *ThemeManager {
	return &ThemeManager{
		themes:     make(map[string]*Theme),
		themesPath: themesPath,
	}
}

func (tm *ThemeManager) LoadThemes() error {
	entries, err := os.ReadDir(tm.themesPath)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	loadedCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()
		themePath := filepath.Join(tm.themesPath, themeName)

		theme, err := tm.loadTheme(themeName, themePath)
		if err != nil {
			logging.LogWarning("failed to load theme '%s': %v", themeName, err)
			continue
		}

		tm.themes[themeName] = theme
		loadedCount++
		logging.LogInfo("loaded theme: %s (v%s by %s)",
			theme.Metadata.Name, theme.Metadata.Version, theme.Metadata.Author)

		// Set first successfully loaded theme as default
		if tm.currentTheme == "" {
			tm.currentTheme = themeName
		}
	}

	if loadedCount == 0 {
		return fmt.Errorf("no valid themes found in %s", tm.themesPath)
	}

	logging.LogInfo("successfully loaded %d themes", loadedCount)
	return nil
}

func (tm *ThemeManager) loadTheme(name, path string) (*Theme, error) {
	theme := &Theme{
		Name: name,
		Path: path,
	}

	// Load metadata with defaults
	metadata := ThemeMetadata{
		Name:    name,
		Version: "1.0.0",
		Author:  "unknown",
		Views:   AvailableViews{
			// Default to "default" if none specified
		},
		Features: ThemeFeatures{
			DarkMode:      true,
			ResponsiveCSS: true,
		},
	}

	metadataPath := filepath.Join(path, "theme.json")
	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(data, &metadata); err != nil {
			logging.LogWarning("invalid theme.json for '%s': %v", name, err)
		}
	}

	// Ensure defaults for empty view arrays
	tm.setDefaultViews(&metadata.Views)
	theme.Metadata = metadata

	// Validate theme
	if err := ValidateTheme(name, path); err != nil {
		return nil, err
	}

	// Load templates with overwrite support
	tmpl, err := tm.loadThemeTemplates(name, path)
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	theme.Template = tmpl
	return theme, nil
}

func (tm *ThemeManager) setDefaultViews(views *AvailableViews) {
	if len(views.File) == 0 {
		views.File = []string{"default"}
	}
	if len(views.Home) == 0 {
		views.Home = []string{"default"}
	}
	if len(views.Search) == 0 {
		views.Search = []string{"default"}
	}
	if len(views.Overview) == 0 {
		views.Overview = []string{"default"}
	}
	if len(views.Dashboard) == 0 {
		views.Dashboard = []string{"default"}
	}
	if len(views.Settings) == 0 {
		views.Settings = []string{"default"}
	}
	if len(views.Admin) == 0 {
		views.Admin = []string{"default"}
	}
	if len(views.Playground) == 0 {
		views.Playground = []string{"default"}
	}
	if len(views.History) == 0 {
		views.History = []string{"default"}
	}
	if len(views.LatestChanges) == 0 {
		views.LatestChanges = []string{"default"}
	}
	if len(views.BrowseFiles) == 0 {
		views.BrowseFiles = []string{"default"}
	}
}

// Load templates with fallback to builtin for missing files
func (tm *ThemeManager) loadThemeTemplates(themeName, themePath string) (*template.Template, error) {
	tmpl := template.New(themeName)

	// Add template functions using FuncMap
	funcMap := template.FuncMap{
		"T": translation.Sprintf,
		// Add more functions as needed
	}
	tmpl.Funcs(funcMap)

	allTemplates := append(RequiredTemplates, OptionalTemplates...)

	for _, templateName := range allTemplates {
		// Try to load from current theme first (overwrite capability)
		templatePath := filepath.Join(themePath, templateName)
		var content []byte

		if data, readErr := os.ReadFile(templatePath); readErr == nil {
			content = data
			logging.LogInfo("loaded template %s from theme %s", templateName, themeName)
		} else if themeName != "builtin" {
			// Fallback to builtin theme for missing templates
			if data, fallbackErr := LoadBuiltinTemplateContent(templateName, builtinThemeFS); fallbackErr == nil {
				content = data
				logging.LogInfo("using builtin fallback for template %s in theme %s", templateName, themeName)
			} else {
				// Required templates must exist
				if contains(RequiredTemplates, templateName) {
					return nil, fmt.Errorf("required template %s not found in theme %s or builtin", templateName, themeName)
				}
				continue // Skip optional templates that don't exist
			}
		} else {
			// For builtin theme, try embedded FS first
			if data, fallbackErr := LoadBuiltinTemplateContent(templateName, builtinThemeFS); fallbackErr == nil {
				content = data
				logging.LogInfo("loaded template %s from builtin embedded FS", templateName)
			} else {
				// For builtin theme, check if template is required
				if contains(RequiredTemplates, templateName) {
					return nil, fmt.Errorf("required template %s not found in builtin theme", templateName)
				}
				continue // Skip optional templates that don't exist
			}
		}

		if _, err := tmpl.Parse(string(content)); err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}
	}

	return tmpl, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (tm *ThemeManager) SetTheme(name string) error {
	if _, exists := tm.themes[name]; !exists {
		return fmt.Errorf("theme '%s' not found", name)
	}
	tm.currentTheme = name
	logging.LogInfo("switched to theme: %s", name)
	return nil
}

func (tm *ThemeManager) GetThemeNames() []string {
	names := make([]string, 0, len(tm.themes))
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.themes[tm.currentTheme]
}

func (tm *ThemeManager) GetCurrentThemeName() string {
	return tm.currentTheme
}

func (tm *ThemeManager) hasTheme(name string) bool {
	_, exists := tm.themes[name]
	return exists
}

func (tm *ThemeManager) HasTemplate(templateName string) bool {
	theme := tm.GetCurrentTheme()
	if theme == nil {
		return false
	}
	return theme.Template.Lookup(templateName) != nil
}

// Single render function with viewName parameter
// Note: No mutex needed here as we only read from currentTheme
func (tm *ThemeManager) Render(w http.ResponseWriter, templateName, viewName string, content TemplateContent) error {
	theme := tm.GetCurrentTheme()
	if theme == nil {
		return fmt.Errorf("no theme is currently set")
	}

	// Try view-specific template first if viewName is provided and not "default"
	actualTemplateName := templateName
	if viewName != "" && viewName != "default" {
		viewTemplateName := strings.TrimSuffix(templateName, ".gotmpl") + "_" + viewName + ".gotmpl"
		if tm.HasTemplate(viewTemplateName) {
			actualTemplateName = viewTemplateName
		}
	}

	if !tm.HasTemplate(actualTemplateName) {
		return fmt.Errorf("template '%s' not found in theme '%s'", actualTemplateName, tm.currentTheme)
	}

	templateData := TemplateData{
		Theme:    tm.currentTheme,
		Language: configmanager.GetLanguage(),
		DarkMode: configmanager.GetDarkMode(),
		Content:  content,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := theme.Template.ExecuteTemplate(w, actualTemplateName, templateData); err != nil {
		return fmt.Errorf("failed to execute template '%s': %w", actualTemplateName, err)
	}

	return nil
}

// ApplyThemeOverwrites applies custom overwrite files to current theme
func (tm *ThemeManager) ApplyThemeOverwrites() error {
	overwritePath := configmanager.GetThemeOverwritePath()

	// Check if overwrite directory exists
	if _, err := os.Stat(overwritePath); os.IsNotExist(err) {
		return nil // No overwrites to apply
	}

	logging.LogInfo("applying theme overwrites from %s", overwritePath)

	theme := tm.GetCurrentTheme()
	if theme == nil {
		return fmt.Errorf("no current theme to apply overwrites to")
	}

	// Reload templates with overwrite support
	tmpl, err := tm.loadThemeTemplatesWithOverwrite(tm.currentTheme, theme.Path, overwritePath)
	if err != nil {
		return fmt.Errorf("failed to apply overwrites: %w", err)
	}

	theme.Template = tmpl
	logging.LogInfo("successfully applied theme overwrites")
	return nil
}

// loadThemeTemplatesWithOverwrite loads templates with both fallback and overwrite support
func (tm *ThemeManager) loadThemeTemplatesWithOverwrite(themeName, themePath, overwritePath string) (*template.Template, error) {
	tmpl := template.New(themeName)

	// Add template functions using FuncMap
	funcMap := template.FuncMap{
		"T": translation.Sprintf,
	}
	tmpl.Funcs(funcMap)

	allTemplates := append(RequiredTemplates, OptionalTemplates...)

	for _, templateName := range allTemplates {
		var content []byte

		// Priority: overwrite > theme > builtin
		overwriteTemplatePath := filepath.Join(overwritePath, templateName)
		themeTemplatePath := filepath.Join(themePath, templateName)

		if data, readErr := os.ReadFile(overwriteTemplatePath); readErr == nil {
			content = data
			logging.LogInfo("loaded template %s from overwrite", templateName)
		} else if data, readErr := os.ReadFile(themeTemplatePath); readErr == nil {
			content = data
			logging.LogInfo("loaded template %s from theme %s", templateName, themeName)
		} else if themeName != "builtin" {
			// Fallback to builtin theme for missing templates
			if data, fallbackErr := LoadBuiltinTemplateContent(templateName, builtinThemeFS); fallbackErr == nil {
				content = data
				logging.LogInfo("using builtin fallback for template %s in theme %s", templateName, themeName)
			} else {
				// Required templates must exist
				if contains(RequiredTemplates, templateName) {
					return nil, fmt.Errorf("required template %s not found in overwrite, theme %s, or builtin", templateName, themeName)
				}
				continue // Skip optional templates that don't exist
			}
		} else {
			// For builtin theme, try embedded FS first
			if data, fallbackErr := LoadBuiltinTemplateContent(templateName, builtinThemeFS); fallbackErr == nil {
				content = data
				logging.LogInfo("loaded template %s from builtin embedded FS", templateName)
			} else {
				// For builtin theme, check if template is required
				if contains(RequiredTemplates, templateName) {
					return nil, fmt.Errorf("required template %s not found in builtin theme", templateName)
				}
				continue // Skip optional templates that don't exist
			}
		}

		if _, err := tmpl.Parse(string(content)); err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}
	}

	return tmpl, nil
}

// GetAvailableViews returns available view variants for a specific view type
func (tm *ThemeManager) GetAvailableViews(viewType string) []string {
	theme := tm.themes[tm.currentTheme]
	if theme == nil {
		return []string{"default"}
	}

	views := &theme.Metadata.Views
	switch viewType {
	case "file":
		return views.File
	case "home":
		return views.Home
	case "search":
		return views.Search
	case "overview":
		return views.Overview
	case "dashboard":
		return views.Dashboard
	case "settings":
		return views.Settings
	case "admin":
		return views.Admin
	case "playground":
		return views.Playground
	case "history":
		return views.History
	case "latestchanges":
		return views.LatestChanges
	case "browsefiles":
		return views.BrowseFiles
	}
	return []string{"default"}
}

// GetAvailableThemes for compatibility with existing API
func (tm *ThemeManager) GetAvailableThemes() []string {
	return tm.GetThemeNames()
}

// Helper functions to create specific content types
func NewHomeContent(title, welcomeText string, quickActions []QuickAction) TemplateContent {
	return TemplateContent{
		Home: &HomeContent{
			Title:        title,
			WelcomeText:  welcomeText,
			QuickActions: quickActions,
		},
	}
}

func NewFileViewContent(title, filePath, fileContent, viewName string) TemplateContent {
	return TemplateContent{
		FileView: &FileViewContent{
			Title:       title,
			FilePath:    filePath,
			FileContent: fileContent,
			ViewName:    viewName,
		},
	}
}

func NewOverviewContent(title string, fileCount int, viewName string) TemplateContent {
	return TemplateContent{
		Overview: &OverviewContent{
			Title:     title,
			FileCount: fileCount,
			ViewName:  viewName,
		},
	}
}

func NewSettingsContent(title, currentTheme, currentLang string, availableViews map[string][]string) TemplateContent {
	return TemplateContent{
		Settings: &SettingsContent{
			Title:          title,
			CurrentTheme:   currentTheme,
			CurrentLang:    currentLang,
			AvailableViews: availableViews,
		},
	}
}

func NewDashboardContent(title, id, action, dashboard string) TemplateContent {
	return TemplateContent{
		Dashboard: &DashboardContent{
			Title:     title,
			ID:        id,
			Action:    action,
			Dashboard: dashboard,
		},
	}
}

func NewSearchContent(title, query string, results []SearchResult) TemplateContent {
	return TemplateContent{
		Search: &SearchContent{
			Title:   title,
			Query:   query,
			Results: results,
		},
	}
}

func NewAdminContent(title string, systemInfo SystemInfo, themeList []string) TemplateContent {
	return TemplateContent{
		Admin: &AdminContent{
			Title:      title,
			SystemInfo: systemInfo,
			ThemeList:  themeList,
		},
	}
}

func NewPlaygroundContent(title string) TemplateContent {
	return TemplateContent{
		Playground: &PlaygroundContent{
			Title: title,
		},
	}
}

func NewHistoryContent(title string, commits []CommitInfo) TemplateContent {
	return TemplateContent{
		History: &HistoryContent{
			Title:   title,
			Commits: commits,
		},
	}
}

func NewLatestChangesContent(title string, changes []FileChange) TemplateContent {
	return TemplateContent{
		LatestChanges: &LatestChangesContent{
			Title:   title,
			Changes: changes,
		},
	}
}

func NewBrowseFilesContent(title, metadataType, value string, files []FileInfo) TemplateContent {
	return TemplateContent{
		BrowseFiles: &BrowseFilesContent{
			Title:        title,
			MetadataType: metadataType,
			Value:        value,
			Files:        files,
		},
	}
}

func NewFileEditContent(title, filePath, content string) TemplateContent {
	return TemplateContent{
		FileEdit: &FileEditContent{
			Title:    title,
			FilePath: filePath,
			Content:  content,
		},
	}
}

// GetThemeMetadata returns metadata for a specific theme
func (tm *ThemeManager) GetThemeMetadata(themeName string) *ThemeMetadata {
	if theme, exists := tm.themes[themeName]; exists {
		return &theme.Metadata
	}
	return nil
}

// LoadTheme loads a specific theme (public method for API compatibility)
func (tm *ThemeManager) LoadTheme(themeName string) error {
	exists := tm.themes[themeName] != nil

	if exists {
		return nil // already loaded
	}

	themePath := filepath.Join(tm.themesPath, themeName)
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return fmt.Errorf("theme '%s' not found at %s", themeName, themePath)
	}

	theme, err := tm.loadTheme(themeName, themePath)
	if err != nil {
		return fmt.Errorf("failed to load theme '%s': %w", themeName, err)
	}

	tm.themes[themeName] = theme
	logging.LogInfo("loaded theme: %s", themeName)
	return nil
}

// SetCurrentTheme sets the current theme (alias for SetTheme for API compatibility)
func (tm *ThemeManager) SetCurrentTheme(name string) error {
	return tm.SetTheme(name)
}
