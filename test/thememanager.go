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
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Author      string        `json:"author"`
	Description string        `json:"description"`
	Views       TemplateViews `json:"views"`
}

type ThemeTemplates struct {
	base     *template.Template
	settings *template.Template
}

type TemplateViews struct {
	BaseViews     []string `json:"base"`
	SettingsViews []string `json:"settings"`
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
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()

		// todo: make it a config
		if themeName == "overwrite" {
			continue
		}

		err := loadSingleTheme(themeName, themesDir)
		if err != nil {
			fmt.Printf("warning: failed to load theme '%s': %v\n", themeName, err)
			continue
		}
	}

	return nil
}

func loadSingleTheme(themeName, themesDir string) error {
	themeDir := filepath.Join(themesDir, themeName)

	err := validateThemeFiles(themeName, themesDir)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	themeJsonPath := filepath.Join(themeDir, "theme.json")

	_, err = os.Stat(themeJsonPath)
	if err != nil {
		return fmt.Errorf("theme.json not found: %w", err)
	}

	// ---------------------- Load Metadata ----------------------
	data, err := os.ReadFile(themeJsonPath)
	if err != nil {
		return fmt.Errorf("could not read theme.json: %w", err)
	}

	var metadata ThemeMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return fmt.Errorf("invalid json in theme.json: %w", err)
	}

	if len(metadata.Views.BaseViews) == 0 {
		metadata.Views.BaseViews = []string{}
	}
	if len(metadata.Views.SettingsViews) == 0 {
		metadata.Views.SettingsViews = []string{}
	}

	// ---------------------- Load Templates ----------------------
	// Load base.gotmpl
	baseTemplatePath := filepath.Join(themeDir, "base.gotmpl")
	baseTemplate, err := template.ParseFiles(baseTemplatePath)
	if err != nil {
		return fmt.Errorf("could not load base.gotmpl: %w", err)
	}

	// Load settings.gotmpl
	settingsTemplatePath := filepath.Join(themeDir, "settings.gotmpl")
	settingsTemplate, err := template.ParseFiles(settingsTemplatePath)
	if err != nil {
		return fmt.Errorf("could not load settings.gotmpl: %w", err)
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

	err = validateTheme(theme)
	if err != nil {
		return fmt.Errorf("theme validation failed: %w", err)
	}

	err = themeManager.addTheme(theme)
	if err != nil {
		return fmt.Errorf("could not add theme: %w", err)
	}

	// todo: check if another theme is set in config
	if themeName == "builtin" {
		themeManager.setCurrentTheme(theme)
	}

	fmt.Printf("added theme: %s\n", metadata.Name)

	return nil
}

// -----------------------------------------------
// -------------------- Render --------------------
// -----------------------------------------------

func (tm *ThemeManager) Render(w http.ResponseWriter, templateName string, viewName string) error {
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
	err := validateTemplateFile(overwritePath)
	if err == nil {
		requiredViews := themeManager.getRequiredViews(templateName)

		overwriteTemplate, parseErr := template.ParseFiles(overwritePath)

		if parseErr != nil {
			fmt.Printf("warning: failed to parse overwrite template '%s': %v, using theme template\n", templateName, parseErr)
		} else {
			fmt.Printf("requiredViews: %v", requiredViews)
			validateErr := validateTemplate(templateName, overwriteTemplate, requiredViews, "overwrite")
			fmt.Printf("validateTemplate overwrite: %v", validateErr)
			if validateErr != nil {
				fmt.Printf("warning: overwrite template validation failed for '%s': %v, using theme template\n", templateName, validateErr)
			} else {
				template = overwriteTemplate
				fmt.Printf("using overwrite template for '%s'\n", templateName)
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if viewName == "" || viewName == "default" || viewName == templateName {

		views := themeManager.getRequiredViews(templateName)

		if len(views) > 0 && views[0] != "" && views[0] != templateName {
			viewName = views[0]
		} else {
			return template.Execute(w, data)
		}

		return template.ExecuteTemplate(w, viewName, data)

		// err = template.Execute(w, data)
		// fmt.Println("err: ", err)
		//
		// validateErr := validateTemplate(templateName, template, views, "overwrite")
		//
		// fmt.Println("verr: ", validateErr)
		// if err == nil && validateErr == nil {
		// 	fmt.Println("xxx")
		// 	return err
		// }
		//
		// viewName = views[0]
	}

	fmt.Println("viewname: ", viewName)

	return template.ExecuteTemplate(w, viewName, data)

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
	tm.themes = append(tm.themes, theme)

	return nil
}

func (tm *ThemeManager) setCurrentTheme(theme Theme) error {
	tm.currentTheme = theme

	return nil
}

func (tm *ThemeManager) getRequiredViews(templateName string) []string {
	switch templateName {
	case "base":
		return tm.currentTheme.Metadata.Views.BaseViews
	case "settings":
		return tm.currentTheme.Metadata.Views.SettingsViews
	default:
		return []string{}
	}
}

// -----------------------------------------------
// ---------------- Error Handling ----------------
// -----------------------------------------------

func validateThemeFiles(themeName, themeDir string) error {
	requiredFiles := []string{"theme.json", "base.gotmpl", "settings.gotmpl"}

	themeDir = filepath.Join(themeDir, themeName)

	themeJsonPath := filepath.Join(themeDir, "theme.json")
	_, err := os.ReadFile(themeJsonPath)
	if err != nil {
		return fmt.Errorf("theme '%s' failed to read theme.json: %v", themeName, err)
	}

	for _, file := range requiredFiles {
		path := filepath.Join(themeDir, file)

		err := validateTemplateFile(path)
		if err != nil {
			return fmt.Errorf("theme '%s': %w", themeName, err)
		}
	}
	return nil
}

func validateTheme(theme Theme) error {
	if theme.Metadata.Name == "" {
		return fmt.Errorf("theme '%s' is missing 'name' in theme.json", theme.Name)
	}
	if theme.Metadata.Version == "" {
		return fmt.Errorf("theme '%s' is missing 'version' in theme.json", theme.Name)
	}
	if theme.Metadata.Author == "" {
		return fmt.Errorf("theme '%s' is missing 'author' in theme.json", theme.Name)
	}
	if theme.Metadata.Description == "" {
		return fmt.Errorf("theme '%s' is missing 'description' in theme.json", theme.Name)
	}

	templateChecks := []struct {
		name     string
		template *template.Template
		views    []string
	}{
		{"base", theme.Templates.base, theme.Metadata.Views.BaseViews},
		{"settings", theme.Templates.settings, theme.Metadata.Views.SettingsViews},
	}

	for _, tc := range templateChecks {
		if len(tc.views) == 0 {
			return fmt.Errorf("theme '%s' is missing 'views.%s' in theme.json", theme.Name, tc.name)
		}
		err := validateTemplate(tc.name, tc.template, tc.views, theme.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateTemplateFile(templatePath string) error {
	if !strings.HasSuffix(templatePath, ".gotmpl") {
		return nil
	}

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

func validateTemplate(templateName string, tmpl *template.Template, views []string, themeName ThemeName) error {
	fmt.Println("validateTemplate ", themeName, templateName, views)
	for _, view := range views {
		if view == "" || view == "default" || view == templateName {
			fmt.Println("continue")
			continue
		}
		fmt.Println("view: ", view)
		fmt.Println("lookup: ", tmpl.Lookup(view))
		if tmpl.Lookup(view) == nil {
			fmt.Println("notfound")
			return fmt.Errorf("theme '%s': view '%s' not found in %s.gotmpl", themeName, view, templateName)
		}
	}
	fmt.Println("return nil")

	return nil
}
