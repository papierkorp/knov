// Package thememanager provides theme management for knov using html/template
package thememanager

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sync"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/translation"
	"knov/internal/utils"
)

// -----------------------------------------------------------------------------
// ----------------------------- Global Variables ------------------------------
// -----------------------------------------------------------------------------

var globalThemeManager *ThemeManager
var builtinThemeArchive embed.FS
var themesPath string // Themes directory path (config/themes)

// SetBuiltinThemeArchive sets the embedded builtin theme archive
func SetBuiltinThemeArchive(fsys embed.FS) {
	builtinThemeArchive = fsys
}

// getThemesPath returns the themes directory path
func getThemesPath() string {
	if themesPath == "" {
		themesPath = filepath.Join(configmanager.GetConfigPath(), "themes")
	}
	return themesPath
}

// Init initializes the global theme manager
func Init() {
	tm := NewThemeManager()
	tm.Initialize()
	globalThemeManager = tm
}

// GetThemeManager returns the global theme manager
func GetThemeManager() IThemeManager {
	return globalThemeManager
}

// NewThemeManager creates a new theme manager instance
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		themes: make(map[string]*Theme),
		funcMap: template.FuncMap{
			"T":   translation.Sprintf, // Translation function
			"add": func(a, b int) int { return a + b },
			"sub": func(a, b int) int { return a - b },
			"mul": func(a, b int) int { return a * b },
		},
	}
}

// -----------------------------------------------------------------------------
// ------------------------------ Data Structures ------------------------------
// -----------------------------------------------------------------------------

// Theme represents a loaded theme
type Theme struct {
	Name      string
	Path      string // empty for builtin, filesystem path for external
	Metadata  ThemeMetadata
	Templates ThemeTemplates
}

// ThemeTemplates contains all parsed templates for a theme
type ThemeTemplates struct {
	Base          *template.Template
	Home          *template.Template
	FileView      *template.Template
	FileEdit      *template.Template
	Search        *template.Template
	Overview      *template.Template
	Dashboard     *template.Template
	Settings      *template.Template
	Admin         *template.Template
	Playground    *template.Template
	History       *template.Template
	LatestChanges *template.Template
	BrowseFiles   *template.Template
}

// ThemeManager manages all themes
type ThemeManager struct {
	themes       map[string]*Theme
	currentTheme *Theme
	funcMap      template.FuncMap // needed for template functions like T, urlquery, eq, etc.
	mutex        sync.RWMutex
}

// ThemeMetadata defines theme capabilities from theme.json
type ThemeMetadata struct {
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	Author      string              `json:"author"`
	Description string              `json:"description"`
	Views       map[string][]string `json:"views"`
	Features    ThemeFeatures       `json:"features"`
}

// ThemeFeatures defines theme feature support
type ThemeFeatures struct {
	DarkMode     bool          `json:"darkMode"`
	ColorSchemes []ColorScheme `json:"colorSchemes"`
}

// ColorScheme defines a color scheme option
type ColorScheme struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// -----------------------------------------------------------------------------
// -------------------------- IThemeManager Interface --------------------------
// -----------------------------------------------------------------------------

// IThemeManager defines the theme manager interface
type IThemeManager interface {
	Initialize()
	GetCurrentTheme() *Theme
	GetCurrentThemeName() string
	SetCurrentTheme(name string) error
	GetAvailableThemes() []string
	LoadThemeFromArchive(name string, reader io.Reader) error
	GetAvailableViews(viewType string) []string
	GetThemeMetadata(themeName string) *ThemeMetadata
	RenderPage(w io.Writer, page string, data interface{}) error
	RenderContent(w io.Writer, page string, data interface{}) error
}

// Initialize loads all themes
func (tm *ThemeManager) Initialize() {
	logging.LogInfo("initialize thememanager ...")

	themesDir := getThemesPath()
	builtinPath := filepath.Join(themesDir, "builtin")

	// Check if builtin theme needs to be extracted
	if _, err := os.Stat(filepath.Join(builtinPath, "theme.json")); os.IsNotExist(err) {
		logging.LogInfo("extracting builtin theme from embedded archive")

		// Read embedded archive
		archiveData, err := builtinThemeArchive.ReadFile("themes/builtin.tar.gz")
		if err != nil {
			logging.LogError("failed to read embedded builtin theme archive: %v", err)
			panic(fmt.Sprintf("cannot continue without builtin theme: %v", err))
		}

		// Use LoadThemeFromArchive to extract and load builtin
		reader := bytes.NewReader(archiveData)
		if err := tm.LoadThemeFromArchive("builtin", reader); err != nil {
			logging.LogError("failed to load builtin theme: %v", err)
			panic(fmt.Sprintf("cannot continue without builtin theme: %v", err))
		}
	}

	// Load all themes from themes/ directory (including builtin if already extracted)
	if err := tm.loadAllThemes(); err != nil {
		logging.LogWarning("failed to load themes: %v", err)
	}

	// Ensure builtin theme exists
	if _, ok := tm.themes["builtin"]; !ok {
		logging.LogError("builtin theme not found after loading")
		panic("builtin theme is required but not found")
	}

	// Set current theme from config
	currentTheme := configmanager.GetTheme()
	if currentTheme == "" {
		currentTheme = "builtin"
	}

	if err := tm.SetCurrentTheme(currentTheme); err != nil {
		logging.LogWarning("failed to set theme %s, using builtin: %v", currentTheme, err)
		configmanager.SetTheme("builtin")
		tm.SetCurrentTheme("builtin")
	}

	logging.LogInfo("theme manager initialized successfully")
}

// loadAllThemes loads all themes from themes/ directory
func (tm *ThemeManager) loadAllThemes() error {
	themesDir := getThemesPath()

	// Create themes directory if it doesn't exist
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes directory: %w", err)
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()
		themePath := filepath.Join(themesDir, themeName)

		if err := tm.loadTheme(themeName, themePath); err != nil {
			logging.LogWarning("failed to load theme %s: %v", themeName, err)
			continue
		}
	}

	return nil
}

// loadTheme loads a single theme from a directory
func (tm *ThemeManager) loadTheme(name string, path string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("loading theme: %s from %s", name, path)

	// Read theme.json
	metadataPath := filepath.Join(path, "theme.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read theme.json: %w", err)
	}

	var metadata ThemeMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("failed to parse theme.json: %w", err)
	}

	// Parse templates
	templatesDir := filepath.Join(path, "templates")
	templates, err := tm.parseTemplates(templatesDir)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	theme := &Theme{
		Name:      metadata.Name,
		Path:      path,
		Metadata:  metadata,
		Templates: templates,
	}

	tm.themes[metadata.Name] = theme
	logging.LogInfo("theme %s loaded successfully", name)
	return nil
}

// parseTemplates parses all templates from a directory
func (tm *ThemeManager) parseTemplates(templatesDir string) (ThemeTemplates, error) {
	var templates ThemeTemplates

	// Read base template first
	baseContent, err := os.ReadFile(filepath.Join(templatesDir, "base.html"))
	if err != nil {
		return templates, fmt.Errorf("failed to read base.html: %w", err)
	}

	// Helper function to parse a template with base
	parseTemplate := func(filename string) (*template.Template, error) {
		content, err := os.ReadFile(filepath.Join(templatesDir, filename))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filename, err)
		}

		tmpl := template.New(filename).Funcs(tm.funcMap)
		if _, err := tmpl.New("base.html").Parse(string(baseContent)); err != nil {
			return nil, fmt.Errorf("failed to parse base.html: %w", err)
		}
		if _, err := tmpl.Parse(string(content)); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
		}

		return tmpl, nil
	}

	// Parse all required templates
	templates.Base = template.Must(template.New("base.html").Funcs(tm.funcMap).Parse(string(baseContent)))
	templates.Home = template.Must(parseTemplate("home.html"))
	templates.FileView = template.Must(parseTemplate("fileview.html"))
	templates.FileEdit = template.Must(parseTemplate("fileedit.html"))
	templates.Search = template.Must(parseTemplate("search.html"))
	templates.Overview = template.Must(parseTemplate("overview.html"))
	templates.Dashboard = template.Must(parseTemplate("dashboard.html"))
	templates.Settings = template.Must(parseTemplate("settings.html"))
	templates.Admin = template.Must(parseTemplate("admin.html"))
	templates.Playground = template.Must(parseTemplate("playground.html"))
	templates.History = template.Must(parseTemplate("history.html"))
	templates.LatestChanges = template.Must(parseTemplate("latestchanges.html"))
	templates.BrowseFiles = template.Must(parseTemplate("browsefiles.html"))

	return templates, nil
}

// LoadThemeFromArchive extracts and loads a theme from a .tgz archive
func (tm *ThemeManager) LoadThemeFromArchive(name string, reader io.Reader) error {
	logging.LogInfo("loading theme from archive: %s", name)

	themesDir := getThemesPath()
	themePath := filepath.Join(themesDir, name)

	// Extract archive using utils
	if err := utils.ExtractTarGzFromReader(reader, themePath); err != nil {
		return fmt.Errorf("failed to extract theme archive: %w", err)
	}

	// Load the extracted theme
	if err := tm.loadTheme(name, themePath); err != nil {
		// Clean up on failure
		os.RemoveAll(themePath)
		return fmt.Errorf("failed to load extracted theme: %w", err)
	}

	logging.LogInfo("theme %s loaded from archive successfully", name)
	return nil
}

// GetCurrentTheme returns the currently active theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.currentTheme
}

// GetCurrentThemeName returns the name of the current theme
func (tm *ThemeManager) GetCurrentThemeName() string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tm.currentTheme != nil {
		return tm.currentTheme.Name
	}
	return ""
}

// SetCurrentTheme sets the active theme
func (tm *ThemeManager) SetCurrentTheme(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	theme, ok := tm.themes[name]
	if !ok {
		return fmt.Errorf("theme %s not found", name)
	}

	tm.currentTheme = theme
	logging.LogInfo("switched to theme: %s", name)
	return nil
}

// GetAvailableThemes returns a list of all available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var names []string
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

// GetAvailableViews returns available view variants for a specific view type
func (tm *ThemeManager) GetAvailableViews(viewType string) []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tm.currentTheme == nil {
		return []string{"default"}
	}

	if views, ok := tm.currentTheme.Metadata.Views[viewType]; ok {
		return views
	}

	return []string{"default"}
}

// GetThemeMetadata returns metadata for a specific theme
func (tm *ThemeManager) GetThemeMetadata(themeName string) *ThemeMetadata {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if theme, ok := tm.themes[themeName]; ok {
		return &theme.Metadata
	}
	return nil
}

// RenderPage renders a page template with the given data
func (tm *ThemeManager) RenderPage(w io.Writer, page string, data interface{}) error {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tm.currentTheme == nil {
		return fmt.Errorf("no theme is currently set")
	}

	// Get the appropriate template
	var tmpl *template.Template
	switch page {
	case "home.html":
		tmpl = tm.currentTheme.Templates.Home
	case "fileview.html":
		tmpl = tm.currentTheme.Templates.FileView
	case "fileedit.html":
		tmpl = tm.currentTheme.Templates.FileEdit
	case "search.html":
		tmpl = tm.currentTheme.Templates.Search
	case "overview.html":
		tmpl = tm.currentTheme.Templates.Overview
	case "dashboard.html":
		tmpl = tm.currentTheme.Templates.Dashboard
	case "settings.html":
		tmpl = tm.currentTheme.Templates.Settings
	case "admin.html":
		tmpl = tm.currentTheme.Templates.Admin
	case "playground.html":
		tmpl = tm.currentTheme.Templates.Playground
	case "history.html":
		tmpl = tm.currentTheme.Templates.History
	case "latestchanges.html":
		tmpl = tm.currentTheme.Templates.LatestChanges
	case "browsefiles.html":
		tmpl = tm.currentTheme.Templates.BrowseFiles
	default:
		return fmt.Errorf("unknown page template: %s", page)
	}

	if tmpl == nil {
		return fmt.Errorf("template %s not found in current theme", page)
	}

	// Execute base template which includes the page-specific content
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}

// RenderContent renders only the content portion of a page (for HTMX requests)
func (tm *ThemeManager) RenderContent(w io.Writer, page string, data interface{}) error {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tm.currentTheme == nil {
		return fmt.Errorf("no theme is currently set")
	}

	// Get the appropriate template
	var tmpl *template.Template
	switch page {
	case "home.html":
		tmpl = tm.currentTheme.Templates.Home
	case "fileview.html":
		tmpl = tm.currentTheme.Templates.FileView
	case "fileedit.html":
		tmpl = tm.currentTheme.Templates.FileEdit
	case "search.html":
		tmpl = tm.currentTheme.Templates.Search
	case "overview.html":
		tmpl = tm.currentTheme.Templates.Overview
	case "dashboard.html":
		tmpl = tm.currentTheme.Templates.Dashboard
	case "settings.html":
		tmpl = tm.currentTheme.Templates.Settings
	case "admin.html":
		tmpl = tm.currentTheme.Templates.Admin
	case "playground.html":
		tmpl = tm.currentTheme.Templates.Playground
	case "history.html":
		tmpl = tm.currentTheme.Templates.History
	case "latestchanges.html":
		tmpl = tm.currentTheme.Templates.LatestChanges
	case "browsefiles.html":
		tmpl = tm.currentTheme.Templates.BrowseFiles
	default:
		return fmt.Errorf("unknown page template: %s", page)
	}

	if tmpl == nil {
		return fmt.Errorf("template %s not found in current theme", page)
	}

	// Execute only the "content" template (not base.html)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "content", data); err != nil {
		return fmt.Errorf("failed to render content template: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
