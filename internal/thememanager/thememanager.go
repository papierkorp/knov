package thememanager

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

	"knov/internal/configmanager"
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
	Name      string
	Metadata  ThemeMetadata
	Templates ThemeTemplates
}

type ThemeMetadata struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Author       string        `json:"author"`
	Description  string        `json:"description"`
	Views        TemplateViews `json:"views"`
	ColorSchemes []ColorScheme `json:"colorSchemes,omitempty"`
}

type ColorScheme struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ThemeTemplates struct {
	admin         *template.Template
	base          *template.Template
	browsefiles   *template.Template
	dashboard     *template.Template
	dashboardedit *template.Template
	dashboardnew  *template.Template
	fileedit      *template.Template
	fileview      *template.Template
	history       *template.Template
	home          *template.Template
	latestchanges *template.Template
	overview      *template.Template
	playground    *template.Template
	search        *template.Template
	settings      *template.Template
}

type TemplateViews struct {
	AdminViews         []string `json:"admin"`
	BaseViews          []string `json:"base"`
	BrowseFilesViews   []string `json:"browsefiles"`
	DashboardViews     []string `json:"dashboard"`
	DashboardEditViews []string `json:"dashboardedit"`
	DashboardNewViews  []string `json:"dashboardnew"`
	FileEditViews      []string `json:"fileedit"`
	FileViewViews      []string `json:"fileview"`
	HistoryViews       []string `json:"history"`
	HomeViews          []string `json:"home"`
	LatestChangesViews []string `json:"latestchanges"`
	OverviewViews      []string `json:"overview"`
	PlaygroundViews    []string `json:"playground"`
	SearchViews        []string `json:"search"`
	SettingsViews      []string `json:"settings"`
}

type TemplateEntry struct {
	Tmpl  *template.Template
	Views []string
}

func InitThemeManager() {
	themeManager = &ThemeManager{}

	// dont run for devs / go run command
	exe, err := os.Executable()
	if err == nil && !strings.Contains(exe, "go-build") {
		initBuiltInTheme(builtinThemeFiles)
	}

	loadAllThemes()

	// set the theme from user configuration after all themes are loaded
	setThemeFromConfig()
}

// setThemeFromConfig loads the saved theme from user configuration
func setThemeFromConfig() {
	savedThemeName := configmanager.GetTheme()

	// find the theme in loaded themes
	for _, theme := range themeManager.themes {
		if theme.Name == savedThemeName {
			err := themeManager.SetCurrentTheme(theme)
			if err != nil {
				fmt.Printf("warning: failed to set saved theme '%s': %v, falling back to builtin\n", savedThemeName, err)
				setBuiltinAsDefault()
			} else {
				fmt.Printf("current theme set to: %s\n", savedThemeName)
			}
			return
		}
	}

	// if saved theme not found, fall back to builtin
	fmt.Printf("warning: saved theme '%s' not found, falling back to builtin\n", savedThemeName)
	setBuiltinAsDefault()
}

// setBuiltinAsDefault sets builtin theme as the current theme
func setBuiltinAsDefault() {
	for _, theme := range themeManager.themes {
		if theme.Name == "builtin" {
			err := themeManager.SetCurrentTheme(theme)
			if err != nil {
				fmt.Printf("error: failed to set builtin theme: %v\n", err)
			} else {
				fmt.Printf("current theme set to: builtin\n")
			}
			return
		}
	}
	fmt.Printf("error: builtin theme not found\n")
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

		err := LoadSingleTheme(themeName, themesDir)
		if err != nil {
			fmt.Printf("warning: failed to load theme '%s': %v\n", themeName, err)
			continue
		}
	}

	return nil
}

func LoadSingleTheme(themeName, themesDir string) error {
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

	// ---------------------- Generate Theme ----------------------
	templates := ThemeTemplates{}
	files, err := filepath.Glob(filepath.Join(themeDir, "*.gohtml"))
	if err != nil {
		return fmt.Errorf("failed to list templates in %s: %w", themeDir, err)
	}

	baseFilePath := filepath.Join(themeDir, "base.gohtml")

	for _, filePath := range files {
		name := strings.TrimSuffix(filepath.Base(filePath), ".gohtml")

		var tmpl *template.Template
		var err error

		funcMap := CreateFuncMap()

		if name == "base" {
			tmpl, err = template.New("base.gohtml").Funcs(funcMap).ParseFiles(filePath)
		} else {
			tmpl, err = template.New("base.gohtml").Funcs(funcMap).ParseFiles(baseFilePath, filePath)
		}

		if err != nil {
			return fmt.Errorf("could not parse %s: %w", filepath.Base(filePath), err)
		}

		switch name {
		case "admin":
			templates.admin = tmpl
		case "base":
			templates.base = tmpl
		case "browsefiles":
			templates.browsefiles = tmpl
		case "dashboard":
			templates.dashboard = tmpl
		case "dashboardedit":
			templates.dashboardedit = tmpl
		case "dashboardnew":
			templates.dashboardnew = tmpl
		case "fileedit":
			templates.fileedit = tmpl
		case "fileview":
			templates.fileview = tmpl
		case "history":
			templates.history = tmpl
		case "home":
			templates.home = tmpl
		case "latestchanges":
			templates.latestchanges = tmpl
		case "overview":
			templates.overview = tmpl
		case "playground":
			templates.playground = tmpl
		case "search":
			templates.search = tmpl
		case "settings":
			templates.settings = tmpl
		default:
			fmt.Printf("warning: unknown template file '%s' â€” ignoring\n", filePath)
		}
	}

	theme := Theme{
		Name:      string(themeName),
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

	fmt.Printf("added theme: %s\n", metadata.Name)

	return nil
}

// -----------------------------------------------
// -------------------- Render --------------------
// -----------------------------------------------

func (tm *ThemeManager) Render(w http.ResponseWriter, templateName string, data any) error {
	var template *template.Template

	template, err := tm.GetTemplate(templateName)
	if err != nil {
		return err
	}

	if template == nil {
		return fmt.Errorf("template '%s' is not loaded", templateName)
	}

	// todo: make config
	overwritePath := filepath.Join("themes", "overwrite", templateName+".gohtml")
	err = validateTemplateFile(overwritePath)
	if err == nil {
		requiredViews := tm.GetAvailableViews(templateName)

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

	return template.Execute(w, data)
}

// -----------------------------------------------
// ---------------- Handle Builtin ----------------
// -----------------------------------------------

var builtinThemeFiles embed.FS

func SetBuiltinFiles(files embed.FS) {
	builtinThemeFiles = files
}

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

func GetThemeManager() ThemeManager {
	return *themeManager
}

func (tm *ThemeManager) GetAvailableViews(templateName string) []string {
	return tm.GetViews(templateName)
}

func (tm *ThemeManager) GetAvailableThemes() []Theme {
	return tm.themes
}

func (tm *ThemeManager) GetCurrentTheme() Theme {
	currentThemeName := configmanager.GetTheme()

	// find the theme in available themes
	for _, theme := range tm.themes {
		if theme.Name == currentThemeName {
			return theme
		}
	}

	// fallback to builtin if not found
	for _, theme := range tm.themes {
		if theme.Name == "builtin" {
			return theme
		}
	}

	// return empty theme if nothing found (shouldn't happen)
	return Theme{}
}

func (tm *ThemeManager) GetCurrentThemeName() string {
	return configmanager.GetTheme()
}

func (tm *ThemeManager) GetCurrentThemeMetadata() ThemeMetadata {
	return tm.currentTheme.Metadata
}

func (tm *ThemeManager) addTheme(theme Theme) error {
	tm.themes = append(tm.themes, theme)

	return nil
}

func (tm *ThemeManager) SetCurrentTheme(theme Theme) error {
	// save to config (source of truth)
	configmanager.SetTheme(theme.Name)

	// also update memory for consistency
	tm.currentTheme = theme

	return nil
}

func (t *Theme) TemplateMap() map[string]TemplateEntry {
	return map[string]TemplateEntry{
		"admin":         {t.Templates.admin, t.Metadata.Views.AdminViews},
		"base":          {t.Templates.base, t.Metadata.Views.BaseViews},
		"browsefiles":   {t.Templates.browsefiles, t.Metadata.Views.BrowseFilesViews},
		"dashboard":     {t.Templates.dashboard, t.Metadata.Views.DashboardViews},
		"dashboardedit": {t.Templates.dashboardedit, t.Metadata.Views.DashboardEditViews},
		"dashboardnew":  {t.Templates.dashboardnew, t.Metadata.Views.DashboardNewViews},
		"fileedit":      {t.Templates.fileedit, t.Metadata.Views.FileEditViews},
		"fileview":      {t.Templates.fileview, t.Metadata.Views.FileViewViews},
		"history":       {t.Templates.history, t.Metadata.Views.HistoryViews},
		"home":          {t.Templates.home, t.Metadata.Views.HomeViews},
		"latestchanges": {t.Templates.latestchanges, t.Metadata.Views.LatestChangesViews},
		"overview":      {t.Templates.overview, t.Metadata.Views.OverviewViews},
		"playground":    {t.Templates.playground, t.Metadata.Views.PlaygroundViews},
		"search":        {t.Templates.search, t.Metadata.Views.SearchViews},
		"settings":      {t.Templates.settings, t.Metadata.Views.SettingsViews},
	}
}

func (tm *ThemeManager) GetTemplate(name string) (*template.Template, error) {
	currentTheme := tm.GetCurrentTheme()

	// if current theme is empty, try to set builtin as default
	if currentTheme.Name == "" {
		setBuiltinAsDefault()
		currentTheme = tm.GetCurrentTheme()
	}

	entry, ok := currentTheme.TemplateMap()[name]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", name)
	}
	return entry.Tmpl, nil
}

func (tm *ThemeManager) GetViews(name string) []string {
	currentTheme := tm.GetCurrentTheme()

	// if current theme is empty, try to set builtin as default
	if currentTheme.Name == "" {
		setBuiltinAsDefault()
		currentTheme = tm.GetCurrentTheme()
	}

	entry, ok := currentTheme.TemplateMap()[name]
	if !ok {
		return []string{}
	}
	return entry.Views
}

// -----------------------------------------------
// ---------------- Error Handling ----------------
// -----------------------------------------------

func validateThemeFiles(themeName, themeDir string) error {
	requiredFiles := []string{"theme.json"}
	themeDir = filepath.Join(themeDir, themeName)

	for name := range (&Theme{}).TemplateMap() {
		requiredFiles = append(requiredFiles, fmt.Sprintf("%s.gohtml", name))
	}

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
	if len(theme.Metadata.ColorSchemes) == 0 {
		return fmt.Errorf("theme '%s' is missing 'colorSchemes' in theme.json", theme.Name)
	}

	// validate each color scheme has required fields
	for i, scheme := range theme.Metadata.ColorSchemes {
		if scheme.Value == "" {
			return fmt.Errorf("theme '%s' colorSchemes[%d] is missing 'value' field", theme.Name, i)
		}
		if scheme.Label == "" {
			return fmt.Errorf("theme '%s' colorSchemes[%d] is missing 'label' field", theme.Name, i)
		}
	}

	for name, entry := range theme.TemplateMap() {
		if entry.Tmpl == nil {
			continue // or return an error if required
		}
		if len(entry.Views) == 0 {
			return fmt.Errorf("theme '%s' is missing 'views.%s' in theme.json", theme.Name, name)
		}

		if err := validateTemplate(name, entry.Tmpl, entry.Views, theme.Name); err != nil {
			return err
		}
	}

	return nil
}

func validateTemplateFile(templatePath string) error {
	if !strings.HasSuffix(templatePath, ".gohtml") {
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

func validateTemplate(templateName string, tmpl *template.Template, views []string, themeName string) error {
	for _, view := range views {
		if view == "" || view == "default" || view == templateName {
			continue
		}
		if tmpl.Lookup(view) == nil {
			return fmt.Errorf("theme '%s': view '%s' not found in %s.gohtml", themeName, view, templateName)
		}
	}

	return nil
}
