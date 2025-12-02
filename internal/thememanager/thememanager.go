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
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	ThemeSettings map[string]ThemeSetting `json:"themeSettings,omitempty"`
}

type ThemeSetting struct {
	Type        string      `json:"type"`
	Default     interface{} `json:"default"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Options     []string    `json:"options,omitempty"`
	Min         *int        `json:"min,omitempty"`
	Max         *int        `json:"max,omitempty"`
}

type ThemeTemplates struct {
	admin          *template.Template
	base           *template.Template
	browse         *template.Template
	browsemetadata *template.Template
	browsefiles    *template.Template
	dashboardview  *template.Template
	dashboardedit  *template.Template
	dashboardnew   *template.Template
	fileedit       *template.Template
	filenew        *template.Template
	fileview       *template.Template
	help           *template.Template
	history        *template.Template
	home           *template.Template
	latestchanges  *template.Template
	overview       *template.Template
	playground     *template.Template
	search         *template.Template
	settings       *template.Template
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
	SetTheme()
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
		case "browse":
			templates.browse = tmpl
		case "browsemetadata":
			templates.browsemetadata = tmpl
		case "browsefiles":
			templates.browsefiles = tmpl
		case "dashboardview":
			templates.dashboardview = tmpl
		case "dashboardedit":
			templates.dashboardedit = tmpl
		case "dashboardnew":
			templates.dashboardnew = tmpl
		case "fileedit":
			templates.fileedit = tmpl
		case "filenew":
			templates.filenew = tmpl
		case "fileview":
			templates.fileview = tmpl
		case "help":
			templates.help = tmpl
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
			fmt.Printf("warning: unknown template file '%s' -> ignoring\n", filePath)
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
		overwriteTemplate, parseErr := template.ParseFiles(overwritePath)

		if parseErr != nil {
			fmt.Printf("warning: failed to parse overwrite template '%s': %v, using theme template\n", templateName, parseErr)
		} else {
			template = overwriteTemplate
			fmt.Printf("using overwrite template for '%s'\n", templateName)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// render template to buffer first to inject default CSS
	var buf strings.Builder
	err = template.Execute(&buf, data)
	if err != nil {
		return err
	}

	// inject default CSS link into <head> section
	html := injectDefaultCSS(buf.String())

	// write final HTML to response
	_, err = w.Write([]byte(html))
	return err
}

// injectDefaultCSS injects the default codehighlight.css link into the HTML <head>
func injectDefaultCSS(html string) string {
	// find the closing </head> tag
	headCloseIndex := strings.Index(html, "</head>")
	if headCloseIndex == -1 {
		// no </head> found, return as is
		return html
	}

	// inject default CSS link before </head>
	defaultCSSLink := `    <link href="/static/css/codehighlight.css" rel="stylesheet" />
`

	return html[:headCloseIndex] + defaultCSSLink + html[headCloseIndex:]
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
// ---------------- Getter/Setter ----------------
// -----------------------------------------------

// SetTheme loads the saved theme from user configuration
func SetTheme() {
	savedThemeName := configmanager.GetTheme()

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

	fmt.Printf("warning: saved theme '%s' not found, falling back to builtin\n", savedThemeName)
	setBuiltinAsDefault()
}

func GetThemeManager() ThemeManager {
	return *themeManager
}

func (tm *ThemeManager) GetAvailableThemes() []Theme {
	return tm.themes
}

func (tm *ThemeManager) GetCurrentTheme() Theme {
	currentThemeName := configmanager.GetTheme()

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

	return Theme{}
}

func (tm *ThemeManager) GetCurrentThemeName() string {
	return configmanager.GetTheme()
}

func (tm *ThemeManager) GetCurrentThemeMetadata() ThemeMetadata {
	return tm.currentTheme.Metadata
}

// GetCurrentThemeSettingsSchema returns the settings schema for the current theme
func (tm *ThemeManager) GetCurrentThemeSettingsSchema() map[string]ThemeSetting {
	currentTheme := tm.GetCurrentTheme()
	if currentTheme.Metadata.ThemeSettings == nil {
		return make(map[string]ThemeSetting)
	}
	return currentTheme.Metadata.ThemeSettings
}

func (tm *ThemeManager) addTheme(theme Theme) error {
	tm.themes = append(tm.themes, theme)

	return nil
}

func (tm *ThemeManager) SetCurrentTheme(theme Theme) error {
	configmanager.SetTheme(theme.Name)

	tm.currentTheme = theme

	return nil
}

func (t *Theme) TemplateMap() map[string]*template.Template {
	return map[string]*template.Template{
		"admin":          t.Templates.admin,
		"base":           t.Templates.base,
		"browse":         t.Templates.browse,
		"browsemetadata": t.Templates.browsemetadata,
		"browsefiles":    t.Templates.browsefiles,
		"dashboardview":  t.Templates.dashboardview,
		"dashboardedit":  t.Templates.dashboardedit,
		"dashboardnew":   t.Templates.dashboardnew,
		"fileedit":       t.Templates.fileedit,
		"filenew":        t.Templates.filenew,
		"fileview":       t.Templates.fileview,
		"help":           t.Templates.help,
		"history":        t.Templates.history,
		"home":           t.Templates.home,
		"latestchanges":  t.Templates.latestchanges,
		"overview":       t.Templates.overview,
		"playground":     t.Templates.playground,
		"search":         t.Templates.search,
		"settings":       t.Templates.settings,
	}
}

func (tm *ThemeManager) GetTemplate(name string) (*template.Template, error) {
	currentTheme := tm.GetCurrentTheme()

	// if current theme is empty, try to set builtin as default
	if currentTheme.Name == "" {
		setBuiltinAsDefault()
		currentTheme = tm.GetCurrentTheme()
	}

	tmpl, ok := currentTheme.TemplateMap()[name]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", name)
	}
	return tmpl, nil
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

	// validate that all templates are loaded
	for name, tmpl := range theme.TemplateMap() {
		if tmpl == nil {
			return fmt.Errorf("theme '%s' is missing template: %s", theme.Name, name)
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
