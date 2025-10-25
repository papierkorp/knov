package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// dont run for devs / go run command
	exe, err := os.Executable()
	if err == nil && !strings.Contains(exe, "go-build") {
		initBuiltInTheme(builtinTheme)
	}

	loadAllThemes()
}

// -----------------------------------------------
// ----------------- load Themes -----------------
// -----------------------------------------------

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
			// todo: make it a config
			if themeName == "overwrite" {
				continue
			}

			themeDir := filepath.Join(themesDir, themeName)

			err := validateTheme(themeName, themesDir)
			if err != nil {
				fmt.Printf("warning: %v\n", err)
				continue
			}

			themeJsonPath := filepath.Join(themeDir, "theme.json")

			_, err = os.Stat(themeJsonPath)
			if err == nil {
				// ---------------------- Load Metadata ----------------------
				data, err := os.ReadFile(themeJsonPath)
				if err != nil {
					fmt.Printf("warning could not read theme.json for theme '%s': %v\n", themeName, err)
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

// -----------------------------------------------
// -------------------- Render --------------------
// -----------------------------------------------

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

	if template == nil {
		return fmt.Errorf("template '%s' is not loaded", templateName)
	}

	// todo: make config
	overwritePath := filepath.Join("themes", "overwrite", templateName+".gotmpl")
	err := validateTemplate(overwritePath)
	if err == nil {
		overwriteTemplate, err := template.ParseFiles(overwritePath)
		if err != nil {
			return fmt.Errorf("failed to parse overwrite template '%s': %v", templateName, err)
		}
		template = overwriteTemplate
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return template.Execute(w, data)
}

// -----------------------------------------------
// ---------------- Handle Builtin ----------------
// -----------------------------------------------

func initBuiltInTheme(builtinTheme embed.FS) error {
	builtinDir := "themes/builtin"

	err := os.MkdirAll(builtinDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create builtin theme directory: %w", err)
	}

	// extract all files
	err = fs.WalkDir(builtinTheme, "themes/builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "themes/builtin" {
			return nil
		}

		relPath := filepath.Join(strings.TrimPrefix(path, "themes/builtin"))
		targetPath := filepath.Join(builtinDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, err := builtinTheme.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		return os.WriteFile(targetPath, data, 0644)
	})

	if err != nil {
		return fmt.Errorf("failed to extract builtin theme: %w", err)
	}

	fmt.Println("extracted builtin theme")
	return nil
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

// -----------------------------------------------
// ---------------- Error Handling ----------------
// -----------------------------------------------

func validateTheme(themeName, themeDir string) error {
	requiredFiles := []string{"theme.json", "base.gotmpl", "settings.gotmpl"}

	themeDir = filepath.Join(themeDir, themeName)

	for _, file := range requiredFiles {
		path := filepath.Join(themeDir, file)
		info, err := os.Stat(path)

		if os.IsNotExist(err) {
			return fmt.Errorf("theme '%s' is missing required file: %s", themeName, file)
		}
		if err != nil {
			return fmt.Errorf("theme '%s' failed to access file %s: %v", themeName, file, err)
		}
		if info.Size() == 0 {
			return fmt.Errorf("theme '%s' has empty file: %s", themeName, file)
		}
	}

	themeJsonPath := filepath.Join(themeDir, "theme.json")
	data, err := os.ReadFile(themeJsonPath)
	if err != nil {
		return fmt.Errorf("theme '%s' failed to read theme.json: %v", themeName, err)
	}

	var metadata ThemeMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("theme '%s' has invalid theme.json: %v", themeName, err)
	}

	if metadata.Name == "" {
		return fmt.Errorf("theme '%s' is missing 'name' in theme.json", themeName)
	}
	if metadata.Version == "" {
		return fmt.Errorf("theme '%s' is missing 'version' in theme.json", themeName)
	}
	if metadata.Author == "" {
		return fmt.Errorf("theme '%s' is missing 'author' in theme.json", themeName)
	}
	if metadata.Description == "" {
		return fmt.Errorf("theme '%s' is missing 'description' in theme.json", themeName)
	}

	return nil
}

func validateTemplate(templatePath string) error {
	info, err := os.Stat(templatePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("template file does not exist: %s", templatePath)
	}
	if err != nil {
		return fmt.Errorf("failed to access template file %s: %v", templatePath, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("template file is empty: %s", templatePath)
	}
	return nil
}
