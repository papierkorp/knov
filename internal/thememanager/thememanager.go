// Package thememanager provides theme management for knov using html/template
package thememanager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ----------------------------- globalThemeManager -----------------------------
// -----------------------------------------------------------------------------

var globalThemeManager *ThemeManager
var builtinThemeFS embed.FS

// SetBuiltinThemeFS sets the embedded filesystem for the builtin theme
func SetBuiltinThemeFS(fsys embed.FS) {
	builtinThemeFS = fsys
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
		themes:        make(map[string]*Theme),
		pageTemplates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"urlquery": template.URLQueryEscaper,
			"sub":      func(a, b int) int { return a - b },
			"mul":      func(a, b int) int { return a * b },
			"add":      func(a, b int) int { return a + b },
			"eq":       func(a, b interface{}) bool { return a == b },
			"ne":       func(a, b interface{}) bool { return a != b },
			"or":       func(a, b bool) bool { return a || b },
			"T":        translation.Sprintf,
		},
	}
}

// -----------------------------------------------------------------------------
// ------------------------------ Data Structures ------------------------------
// -----------------------------------------------------------------------------

// Theme represents a loaded theme
type Theme struct {
	Name      string
	Path      string // filesystem path for external themes
	Metadata  ThemeMetadata
	Templates *template.Template
	IsBuiltin bool
}

// ThemeManager manages all themes
type ThemeManager struct {
	themes        map[string]*Theme
	currentTheme  *Theme
	funcMap       template.FuncMap
	pageTemplates map[string]*template.Template // page-specific templates
	mutex         sync.RWMutex
}

// ThemeMetadata defines theme capabilities from theme.json
type ThemeMetadata struct {
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	Author      string              `json:"author"`
	Description string              `json:"description"`
	Views       map[string][]string `json:"views"`
	Features    ThemeFeatures       `json:"features"`
	Templates   map[string]string   `json:"templates"`
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
}

// Initialize loads all themes
func (tm *ThemeManager) Initialize() {
	logging.LogInfo("initialize thememanager ...")

	// Load builtin theme
	if err := tm.loadBuiltinTheme(); err != nil {
		logging.LogError("failed to load builtin theme: %v", err)
		panic(fmt.Sprintf("cannot continue without builtin theme: %v", err))
	}

	// Load external themes from themes/external directory
	if err := tm.loadExternalThemes(); err != nil {
		logging.LogWarning("failed to load external themes: %v", err)
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

// loadBuiltinTheme loads the embedded builtin theme
func (tm *ThemeManager) loadBuiltinTheme() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("loading builtin theme from embedded filesystem")

	// Read theme.json
	metadataBytes, err := builtinThemeFS.ReadFile("themes/builtin/theme.json")
	if err != nil {
		return fmt.Errorf("failed to read builtin theme.json: %w", err)
	}

	var metadata ThemeMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("failed to parse builtin theme.json: %w", err)
	}

	// Read base template content first
	baseContent, err := builtinThemeFS.ReadFile("themes/builtin/templates/base.html")
	if err != nil {
		return fmt.Errorf("failed to read base.html: %w", err)
	}

	// Create a dummy template set for the theme (for compatibility)
	tmpl := template.New("builtin").Funcs(tm.funcMap)

	// Parse all .html files and create individual template sets for each page
	err = fs.WalkDir(builtinThemeFS, "themes/builtin/templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		templateName := filepath.Base(path)

		// Skip base.html in the loop
		if templateName == "base.html" {
			return nil
		}

		content, err := builtinThemeFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Create a new template set for this page that includes base + page content
		pageTmpl := template.New(templateName).Funcs(tm.funcMap)

		// Parse base template first
		if _, err := pageTmpl.New("base.html").Parse(string(baseContent)); err != nil {
			return fmt.Errorf("failed to parse base.html for %s: %w", templateName, err)
		}

		// Parse the page template
		if _, err := pageTmpl.Parse(string(content)); err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}

		// Store this page-specific template set
		tm.pageTemplates[templateName] = pageTmpl

		logging.LogInfo("parsed builtin template: %s", path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse builtin templates: %w", err)
	}

	theme := &Theme{
		Name:      "builtin",
		Path:      "",
		Metadata:  metadata,
		Templates: tmpl,
		IsBuiltin: true,
	}

	tm.themes["builtin"] = theme
	logging.LogInfo("builtin theme loaded successfully")
	return nil
}

// loadExternalThemes loads all themes from themes/external directory
func (tm *ThemeManager) loadExternalThemes() error {
	externalDir := filepath.Join(configmanager.GetThemesPath(), "external")

	// Create external directory if it doesn't exist
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		return fmt.Errorf("failed to create external themes directory: %w", err)
	}

	entries, err := os.ReadDir(externalDir)
	if err != nil {
		return fmt.Errorf("failed to read external themes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()
		themePath := filepath.Join(externalDir, themeName)

		if err := tm.loadExternalTheme(themeName, themePath); err != nil {
			logging.LogWarning("failed to load external theme %s: %v", themeName, err)
			continue
		}
	}

	return nil
}

// loadExternalTheme loads a single external theme from a directory
func (tm *ThemeManager) loadExternalTheme(name string, path string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("loading external theme: %s from %s", name, path)

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

	// Read base template content first
	baseTemplatePath := filepath.Join(path, "templates", "base.html")
	baseContent, err := os.ReadFile(baseTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read base.html: %w", err)
	}

	// Create a dummy template set for the theme (for compatibility)
	tmpl := template.New(name).Funcs(tm.funcMap)

	templatesDir := filepath.Join(path, "templates")
	err = filepath.WalkDir(templatesDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(filePath, ".html") {
			return nil
		}

		templateName := filepath.Base(filePath)

		// Skip base.html in the loop
		if templateName == "base.html" {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// Create a new template set for this page that includes base + page content
		pageTmpl := template.New(templateName).Funcs(tm.funcMap)

		// Parse base template first
		if _, err := pageTmpl.New("base.html").Parse(string(baseContent)); err != nil {
			return fmt.Errorf("failed to parse base.html for %s: %w", templateName, err)
		}

		// Parse the page template
		if _, err := pageTmpl.Parse(string(content)); err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}

		// Store this page-specific template set
		tm.pageTemplates[templateName] = pageTmpl

		logging.LogInfo("parsed template: %s", filePath)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	theme := &Theme{
		Name:      metadata.Name,
		Path:      path,
		Metadata:  metadata,
		Templates: tmpl,
		IsBuiltin: false,
	}

	tm.themes[metadata.Name] = theme
	logging.LogInfo("external theme %s loaded successfully", name)
	return nil
}

// LoadThemeFromArchive extracts and loads a theme from a .tgz archive
func (tm *ThemeManager) LoadThemeFromArchive(name string, reader io.Reader) error {
	logging.LogInfo("loading theme from archive: %s", name)

	// Create temporary directory for extraction
	externalDir := filepath.Join(configmanager.GetThemesPath(), "external")
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		return fmt.Errorf("failed to create external themes directory: %w", err)
	}

	themePath := filepath.Join(externalDir, name)

	// Remove existing theme directory if it exists
	if err := os.RemoveAll(themePath); err != nil {
		return fmt.Errorf("failed to remove existing theme directory: %w", err)
	}

	// Create theme directory
	if err := os.MkdirAll(themePath, 0755); err != nil {
		return fmt.Errorf("failed to create theme directory: %w", err)
	}

	// Extract .tgz archive
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Security: prevent path traversal
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		target := filepath.Join(themePath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	// Load the extracted theme
	if err := tm.loadExternalTheme(name, themePath); err != nil {
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

	// Get the page-specific template set
	pageTmpl, ok := tm.pageTemplates[page]
	if !ok {
		return fmt.Errorf("page template %s not found", page)
	}

	// Execute base template which includes the page-specific content definitions
	var buf bytes.Buffer
	if err := pageTmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
