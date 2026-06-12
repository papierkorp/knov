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
	"knov/internal/logging"
	"knov/internal/server/notify"
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
	filedittable   *template.Template
	filenew        *template.Template
	fileview       *template.Template
	filesoverview  *template.Template
	filteredit     *template.Template
	filterview     *template.Template
	help           *template.Template
	history        *template.Template
	home           *template.Template
	playground     *template.Template
	search         *template.Template
	settings       *template.Template
	mediaview      *template.Template
	mediaoverview  *template.Template
	chat           *template.Template
	kanban         *template.Template
}

func InitThemeManager() {
	themeManager = &ThemeManager{}

	// dont run for devs / go run command
	exe, err := os.Executable()
	if err == nil && !strings.Contains(exe, "go-build") {
		initBuiltInTheme(builtinThemeFiles)
		initRailTheme(railThemeFiles)
	}

	loadAllThemes()

	// set the theme from user configuration after all themes are loaded
	SetTheme()
}

// -----------------------------------------------
// ----------------- load Themes -----------------
// -----------------------------------------------

func loadAllThemes() error {
	themesDir := configmanager.GetThemesPath()

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
			logging.LogWarning("failed to load theme '%s': %v", themeName, err)
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
		case "filedittable":
			templates.filedittable = tmpl
		case "filenew":
			templates.filenew = tmpl
		case "fileview":
			templates.fileview = tmpl
		case "filteredit":
			templates.filteredit = tmpl
		case "filterview":
			templates.filterview = tmpl
		case "help":
			templates.help = tmpl
		case "history":
			templates.history = tmpl
		case "home":
			templates.home = tmpl
		case "playground":
			templates.playground = tmpl
		case "search":
			templates.search = tmpl
		case "settings":
			templates.settings = tmpl
		case "mediaoverview":
			templates.mediaoverview = tmpl
		case "mediaview":
			templates.mediaview = tmpl
		case "filesoverview":
			templates.filesoverview = tmpl
		case "chat":
			templates.chat = tmpl
		case "kanban":
			templates.kanban = tmpl
		default:
			logging.LogWarning("unknown template file '%s' -> ignoring", filePath)
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

	logging.LogInfo("added theme: %s", metadata.Name)

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
			logging.LogWarning("failed to parse overwrite template '%s': %v, using theme template", templateName, parseErr)
		} else {
			template = overwriteTemplate
			logging.LogDebug("using overwrite template for '%s'", templateName)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// render template to buffer first to inject default CSS and JS
	var buf strings.Builder
	err = template.Execute(&buf, data)
	if err != nil {
		return err
	}

	// inject default CSS link into <head> section
	html := injectDefaultCSS(buf.String())

	// inject default JS before </body>
	html = injectDefaultJS(html)

	// write final HTML to response
	_, err = w.Write([]byte(html))
	return err
}

// injectDefaultCSS injects static CSS links that every theme requires into </head>.
func injectDefaultCSS(html string) string {
	headCloseIndex := strings.Index(html, "</head>")
	if headCloseIndex == -1 {
		return html
	}

	defaultCSSLinks := `    <link href="/static/css/notify.css" rel="stylesheet" />
    <link href="/static/css/codehighlight.css" rel="stylesheet" />
    <link href="/static/css/indexeditor.css" rel="stylesheet" />
    <link href="/static/css/listeditor.css" rel="stylesheet" />
	  <link href="/static/css/kanban.css" rel="stylesheet" />
    <link href="/static/css/todoeditor.css" rel="stylesheet" />
    <link href="/static/css/tableeditor.css" rel="stylesheet" />
    <link href="/static/css/filtereditor.css" rel="stylesheet" />
    <link href="/static/css/markdowneditor.css" rel="stylesheet" />
`

	return html[:headCloseIndex] + defaultCSSLinks + html[headCloseIndex:]
}

// injectDefaultJS injects scripts that every theme requires before </body>.
// Theme creators do not need to add these manually.
func injectDefaultJS(html string) string {
	bodyCloseIndex := strings.Index(html, "</body>")
	if bodyCloseIndex == -1 {
		return html
	}
	scripts := notify.RenderJS(configmanager.GetNotifyDuration())
	scripts += `<script src="/static/wiki-autocomplete.js"></script>`
	scripts += `<script>
function toggleConflictDiff(btn, id, url) {
	var c = document.getElementById(id);
	if (c.innerHTML !== '') { c.innerHTML = ''; btn.textContent = btn.dataset.show; return; }
	htmx.ajax('GET', url, {target: '#' + id, swap: 'innerHTML'});
	btn.textContent = btn.dataset.hide;
}
</script>`
	return html[:bodyCloseIndex] + scripts + html[bodyCloseIndex:]
}

// -----------------------------------------------
// ---------------- Handle Rail ------------------
// -----------------------------------------------

var railThemeFiles embed.FS

func SetRailFiles(files embed.FS) {
	railThemeFiles = files
}

func initRailTheme(railTheme embed.FS) error {
	railDir := "themes/rail"

	if err := os.MkdirAll(railDir, 0755); err != nil {
		return fmt.Errorf("failed to create rail theme directory: %w", err)
	}

	err := fs.WalkDir(railTheme, "themes/rail", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "themes/rail" {
			return nil
		}

		relPath := strings.TrimPrefix(path, "themes/rail")
		targetPath := filepath.Join(railDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, err := railTheme.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		return os.WriteFile(targetPath, data, 0644)
	})

	if err != nil {
		return fmt.Errorf("failed to extract rail theme: %w", err)
	}

	logging.LogInfo("extracted rail theme")
	return nil
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

	logging.LogInfo("extracted builtin theme")
	return nil
}

// setBuiltinAsDefault sets builtin theme as the current theme
func setBuiltinAsDefault() {
	for _, theme := range themeManager.themes {
		if theme.Name == "builtin" {
			err := themeManager.SetCurrentTheme(theme)
			if err != nil {
				logging.LogError("failed to set builtin theme: %v", err)
			} else {
				logging.LogInfo("current theme set to: builtin")
			}
			return
		}
	}
	logging.LogError("builtin theme not found")
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
				logging.LogWarning("failed to set saved theme '%s': %v, falling back to builtin", savedThemeName, err)
				setBuiltinAsDefault()
			} else {
				logging.LogInfo("current theme set to: %s", savedThemeName)
			}
			return
		}
	}

	logging.LogWarning("saved theme '%s' not found, falling back to builtin", savedThemeName)
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
		"filedittable":   t.Templates.filedittable,
		"filenew":        t.Templates.filenew,
		"fileview":       t.Templates.fileview,
		"filteredit":     t.Templates.filteredit,
		"filterview":     t.Templates.filterview,
		"help":           t.Templates.help,
		"history":        t.Templates.history,
		"home":           t.Templates.home,
		"playground":     t.Templates.playground,
		"search":         t.Templates.search,
		"settings":       t.Templates.settings,
		"mediaoverview":  t.Templates.mediaoverview,
		"mediaview":      t.Templates.mediaview,
		"filesoverview":  t.Templates.filesoverview,
		"chat":           t.Templates.chat,
		"kanban":         t.Templates.kanban,
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
