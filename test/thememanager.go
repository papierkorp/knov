package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

// -----------------------------------------------
// ------------- Define Thememanager -------------
// -----------------------------------------------

var themeManager *ThemeManager

type ThemeManager struct {
	themes       []Theme
	currentTheme Theme
}

type Theme struct {
	Name      ThemeName
	Metadata  ThemeMetadata
	Templates ThemeTemplates
}

type ThemeName string

type ThemeMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

type ThemeTemplates struct {
	base     *template.Template
	settings *template.Template
}

func InitThemeManager() {
	themeManager = &ThemeManager{}
	loadAllThemes()

	fmt.Printf("thememanager current: %v\n", themeManager.currentTheme.Name)
	fmt.Printf("thememanager themes: %v\n", themeManager.themes)

}

func loadAllThemes() error {
	// todo: use themespath config
	themesDir := "themes"

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			themeName := entry.Name()
			themeDir := filepath.Join(themesDir, themeName)
			themeJsonPath := filepath.Join(themeDir, "theme.json")

			_, err := os.Stat(themeJsonPath)
			if err == nil {
				// ---------------------- Load Metadata ----------------------
				data, err := os.ReadFile(themeJsonPath)
				if err != nil {
					fmt.Printf("arning could not read theme.json for theme '%s': %v\n", themeName, err)
					continue
				}

				var metadata ThemeMetadata
				err = json.Unmarshal(data, &metadata)
				if err != nil {
					fmt.Printf("warning invalid json in theme.json for theme: %s: %v", themeName, err)
				}

				// ---------------------- Load Templates ----------------------
				// Load base.gotmpl
				baseTemplatePath := filepath.Join(themeDir, "base.gotmpl")
				baseTemplate, err := template.ParseFiles(baseTemplatePath)
				if err != nil {
					fmt.Printf("warning could not load base.gotmpl for theme '%s': %v\n", themeName, err)
					continue
				}

				// Load settings.gotmpl
				settingsTemplatePath := filepath.Join(themeDir, "settings.gotmpl")
				settingsTemplate, err := template.ParseFiles(settingsTemplatePath)
				if err != nil {
					fmt.Printf("warning could not load settings.gotmpl for theme '%s': %v\n", themeName, err)
					continue
				}

				// ---------------------- Generate Theme ----------------------

				templates := ThemeTemplates{
					base:     baseTemplate,
					settings: settingsTemplate,
				}

				theme := Theme{
					Name:      ThemeName(themeName),
					Metadata:  metadata,
					Templates: templates,
				}

				err = themeManager.addTheme(theme)
				if err != nil {
					fmt.Printf("warning could not add theme: %s: %v", themeName, err)
					continue
				}

				// todo: check if another theme is set in config
				if themeName == "builtin" {
					themeManager.setCurrentTheme(theme)
				}

				fmt.Printf("added theme: %s\n", metadata.Name)
			}
		}
	}

	return nil
}

func (tm *ThemeManager) Render(w http.ResponseWriter, templateName string) error {
	var template *template.Template

	data := map[string]any{
		"Title":        templateName,
		"Themes":       tm.themes,
		"CurrentTheme": tm.currentTheme.Name,
	}

	switch templateName {
	case "base":
		template = tm.currentTheme.Templates.base
	case "settings":
		template = tm.currentTheme.Templates.settings
	default:
		return fmt.Errorf("unknown template type: %s", templateName)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return template.Execute(w, data)
}

// -----------------------------------------------
// ---------------- Getter/Setter ----------------
// -----------------------------------------------

func (tm *ThemeManager) addTheme(theme Theme) error {
	// todo: validate theme before adding it
	tm.themes = append(tm.themes, theme)

	return nil
}

func (tm *ThemeManager) setCurrentTheme(theme Theme) error {
	// todo: validate theme before adding it
	tm.currentTheme = theme

	return nil
}
