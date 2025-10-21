// Package main ..
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
)

type IThemeManager interface {
	Initialize() error
	GetCurrentTheme() *Theme
	GetCurrentThemeName() string
	SetCurrentTheme(name string) error
	GetAvailableThemes() []string
	Render(w http.ResponseWriter, templateName string, data ThemeData) error
}

type ThemeData struct {
	TemplateData
	CurrentTheme    string
	CurrentMetadata ThemeMetadata
	AvailableThemes []string
}

type ThemeManager struct {
	themes       map[string]*Theme
	currentTheme *Theme
	funcMap      template.FuncMap
}

type Theme struct {
	Name      string
	Path      string
	Metadata  ThemeMetadata
	Templates ThemeTemplates
}

type ThemeMetadata struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Author      string        `json:"author"`
	Description string        `json:"description"`
	Features    ThemeFeatures `json:"features"`
}

type ThemeFeatures struct {
	DarkMode bool `json:"darkMode"`
}

type ThemeTemplates struct {
	Base *template.Template
}

func NewThemeManager() *ThemeManager {
	tm := &ThemeManager{
		themes: make(map[string]*Theme),
		funcMap: template.FuncMap{
			"formatDate": func(t any) string {
				if tv, ok := t.(interface{ Format(string) string }); ok {
					return tv.Format("January 2, 2006")
				}
				return ""
			},
		},
	}
	return tm
}

func (tm *ThemeManager) Initialize() error {
	themesDir := "./themes"

	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		return fmt.Errorf("themes directory does not exist: %s", themesDir)
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("error reading themes directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			themeName := entry.Name()
			themePath := filepath.Join(themesDir, themeName)

			theme, err := tm.loadTheme(themeName, themePath)
			if err != nil {
				fmt.Printf("error loading theme %s: %v\n", themeName, err)
				continue
			}

			tm.themes[themeName] = theme
			fmt.Printf("loaded theme: %s (v%s by %s)\n", theme.Metadata.Name, theme.Metadata.Version, theme.Metadata.Author)
		}
	}

	// default theme
	if len(tm.themes) > 0 {
		for _, theme := range tm.themes {
			tm.currentTheme = theme
			break
		}
	}

	return nil
}

func (tm *ThemeManager) loadTheme(name, path string) (*Theme, error) {
	theme := &Theme{
		Name: name,
		Path: path,
	}

	// Load metadata
	metadataPath := filepath.Join(path, "theme.json")
	if _, err := os.Stat(metadataPath); err == nil {
		metadataFile, err := os.ReadFile(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("error reading metadata file: %v", err)
		}

		if err := json.Unmarshal(metadataFile, &theme.Metadata); err != nil {
			return nil, fmt.Errorf("error parsing metadata: %v", err)
		}
	} else {
		// Default metadata if no theme.json exists
		theme.Metadata = ThemeMetadata{
			Name:        name,
			Version:     "1.0.0",
			Author:      "Unknown",
			Description: fmt.Sprintf("Theme %s", name),
		}
	}

	// Load templates
	templatePattern := filepath.Join(path, "*.gotmpl")
	tmpl := template.New(name).Funcs(tm.funcMap)

	tmpl, err := tmpl.ParseGlob(templatePattern)
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %v", err)
	}

	theme.Templates.Base = tmpl
	return theme, nil
}

func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

func (tm *ThemeManager) GetCurrentThemeName() string {
	if tm.currentTheme == nil {
		return ""
	}
	return tm.currentTheme.Name
}

func (tm *ThemeManager) SetCurrentTheme(name string) error {
	theme, exists := tm.themes[name]
	if !exists {
		return fmt.Errorf("theme %s not found", name)
	}
	tm.currentTheme = theme
	return nil
}

func (tm *ThemeManager) GetAvailableThemes() []string {
	themes := make([]string, 0, len(tm.themes))
	for name := range tm.themes {
		themes = append(themes, name)
	}
	return themes
}

func (tm *ThemeManager) Render(w http.ResponseWriter, templateName string, data ThemeData) error {
	if tm.currentTheme == nil {
		return fmt.Errorf("no theme is currently set")
	}

	w.Header().Set("Content-Type", "text/html")
	return tm.currentTheme.Templates.Base.ExecuteTemplate(w, templateName, data)
}
