package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Required template files for each theme
var RequiredTemplates = []string{
	"base.gotmpl",
	"history.gotmpl", 
	"fileview.gotmpl",
}

type ThemeManager struct {
	themes       map[string]*Theme
	currentTheme string
	logger       *log.Logger
}

type Theme struct {
	Name     string
	Metadata ThemeMetadata
	Template *template.Template
}

type ThemeMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

type ThemeData struct {
	TemplateData
	CurrentTheme    string
	CurrentMetadata ThemeMetadata
	AvailableThemes []string
}

type ThemeValidationError struct {
	ThemeName string
	Errors    []string
}

func (e ThemeValidationError) Error() string {
	return fmt.Sprintf("theme '%s' validation failed: %s", e.ThemeName, strings.Join(e.Errors, ", "))
}

func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		themes: make(map[string]*Theme),
		logger: log.New(os.Stdout, "[ThemeManager] ", log.LstdFlags),
	}
}

func (tm *ThemeManager) LoadThemes(themesDir string) error {
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	loadedCount := 0
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()
		themePath := filepath.Join(themesDir, themeName)
		
		theme, err := tm.loadTheme(themeName, themePath)
		if err != nil {
			tm.logger.Printf("Failed to load theme '%s': %v", themeName, err)
			continue
		}

		tm.themes[themeName] = theme
		loadedCount++
		tm.logger.Printf("Loaded theme: %s (v%s by %s)", 
			theme.Metadata.Name, theme.Metadata.Version, theme.Metadata.Author)

		// Set first successfully loaded theme as default
		if tm.currentTheme == "" {
			tm.currentTheme = themeName
		}
	}

	if loadedCount == 0 {
		return fmt.Errorf("no valid themes found in %s", themesDir)
	}

	tm.logger.Printf("Successfully loaded %d themes", loadedCount)
	return nil
}

func (tm *ThemeManager) loadTheme(name, path string) (*Theme, error) {
	// Validate required files exist
	if err := tm.validateThemeFiles(name, path); err != nil {
		return nil, err
	}

	theme := &Theme{Name: name}

	// Load metadata
	metadata := ThemeMetadata{
		Name:    name,
		Version: "1.0.0",
		Author:  "Unknown",
	}
	
	metadataPath := filepath.Join(path, "theme.json")
	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(data, &metadata); err != nil {
			tm.logger.Printf("Warning: invalid theme.json for '%s': %v", name, err)
		}
	}
	theme.Metadata = metadata

	// Load templates
	templatePath := filepath.Join(path, "*.gotmpl")
	tmpl, err := template.ParseGlob(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Verify all required templates are present in parsed template
	if err := tm.validateParsedTemplates(name, tmpl); err != nil {
		return nil, err
	}

	theme.Template = tmpl
	return theme, nil
}

func (tm *ThemeManager) validateThemeFiles(themeName, themePath string) error {
	var errors []string

	for _, requiredFile := range RequiredTemplates {
		filePath := filepath.Join(themePath, requiredFile)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("missing required file: %s", requiredFile))
		}
	}

	if len(errors) > 0 {
		return ThemeValidationError{ThemeName: themeName, Errors: errors}
	}

	return nil
}

func (tm *ThemeManager) validateParsedTemplates(themeName string, tmpl *template.Template) error {
	var errors []string

	for _, requiredTemplate := range RequiredTemplates {
		if tmpl.Lookup(requiredTemplate) == nil {
			errors = append(errors, fmt.Sprintf("template not parsed: %s", requiredTemplate))
		}
	}

	if len(errors) > 0 {
		return ThemeValidationError{ThemeName: themeName, Errors: errors}
	}

	return nil
}

func (tm *ThemeManager) SetTheme(name string) error {
	if _, exists := tm.themes[name]; !exists {
		return fmt.Errorf("theme '%s' not found", name)
	}
	tm.currentTheme = name
	tm.logger.Printf("Switched to theme: %s", name)
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

func (tm *ThemeManager) HasTemplate(templateName string) bool {
	theme := tm.GetCurrentTheme()
	if theme == nil {
		return false
	}
	return theme.Template.Lookup(templateName) != nil
}

func (tm *ThemeManager) Render(w http.ResponseWriter, templateName string, data TemplateData) error {
	theme := tm.GetCurrentTheme()
	if theme == nil {
		return fmt.Errorf("no theme is currently set")
	}

	// Check if the requested template exists
	if !tm.HasTemplate(templateName) {
		return fmt.Errorf("template '%s' not found in theme '%s'", templateName, tm.currentTheme)
	}

	themeData := ThemeData{
		TemplateData:    data,
		CurrentTheme:    tm.currentTheme,
		CurrentMetadata: theme.Metadata,
		AvailableThemes: tm.GetThemeNames(),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := theme.Template.ExecuteTemplate(w, templateName, themeData); err != nil {
		return fmt.Errorf("failed to execute template '%s': %w", templateName, err)
	}

	return nil
}
