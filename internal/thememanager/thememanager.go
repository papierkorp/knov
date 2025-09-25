// Package thememanager ...
package thememanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"slices"
	"sync"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/logging"

	"github.com/a-h/templ"
)

// -----------------------------------------------------------------------------
// ----------------------------- globalThemeManager -----------------------------
// -----------------------------------------------------------------------------

var globalThemeManager *ThemeManager

// Init ..
func Init() {
	tm := NewThemeManager()
	tm.Initialize()
	globalThemeManager = tm
}

// GetThemeManager ...
func GetThemeManager() IThemeManager {
	return globalThemeManager
}

// NewThemeManager ..
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		themes: make(map[string]ITheme),
	}
}

// ThemeManager ...
type ThemeManager struct {
	themes       map[string]ITheme
	currentTheme ITheme
	mutex        sync.RWMutex
}

// ITheme ...
type ITheme interface {
	Home() (templ.Component, error)
	Settings() (templ.Component, error)
	Admin() (templ.Component, error)
	Playground() (templ.Component, error)
	LatestChanges() (templ.Component, error)
	History() (templ.Component, error)
	Search(query string) (templ.Component, error)
	Overview() (templ.Component, error)
	GetAvailableFileViews() []string
	RenderFileView(viewName string, content string, filePath string) (templ.Component, error)
	Dashboard(id string) (templ.Component, error)
}

// -----------------------------------------------------------------------------
// -------------------------- IThemeManager Interface --------------------------
// -----------------------------------------------------------------------------

// TemplateData to be passed onto Templates for dynamic data access
type TemplateData struct {
	ThemeToUse      string
	AvailableThemes []string
	Dashboard       *dashboard.Dashboard
	ShowCreateForm  bool
}

// -----------------------------------------------------------------------------
// -------------------------- IThemeManager Interface --------------------------
// -----------------------------------------------------------------------------

// IThemeManager ..
type IThemeManager interface {
	Initialize()
	GetCurrentTheme() ITheme
	GetCurrentThemeName() string
	SetCurrentTheme(name string) error
	GetAvailableThemes() []string
	LoadTheme(themeName string) error
	LoadAllThemes() error
}

// Initialize loads all themes from the themes directory
func (tm *ThemeManager) Initialize() {
	logging.LogInfo("initialize thememanager ...")

	err := tm.CompileThemes()
	if err != nil {
		logging.LogError("failed compiling the themes: %s", err)
	}

	err = tm.LoadAllThemes()
	if err != nil {
		logging.LogError("failed to load all themes: %v", err)
	}

	availableThemes := tm.GetAvailableThemes()
	currentTheme := configmanager.GetTheme()
	if currentTheme == "" || !slices.Contains(availableThemes, currentTheme) {
		logging.LogError("couldn't find theme: %s, using builtin instead", currentTheme)
		configmanager.SetTheme("builtin")
		currentTheme = "builtin"
	}
	err = tm.SetCurrentTheme(currentTheme)
	if err != nil {
		logging.LogError("failed to set current theme - %s: %v", currentTheme, err)
	}

	logging.LogInfo("theme loaded successfully")
}

// GetCurrentTheme ..
func (tm *ThemeManager) GetCurrentTheme() ITheme {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.currentTheme
}

// GetCurrentThemeName ..
func (tm *ThemeManager) GetCurrentThemeName() string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for name, theme := range tm.themes {
		if theme == tm.currentTheme {
			return name
		}
	}
	return ""
}

// SetCurrentTheme ..
func (tm *ThemeManager) SetCurrentTheme(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("start to set current theme")
	theme, ok := tm.themes[name]
	if !ok {
		logging.LogError("theme %s not found", name)
		return fmt.Errorf("theme %s not found", name)
	}

	tm.currentTheme = theme
	logging.LogInfo("switched to theme: %s", name)
	return nil
}

// GetAvailableThemes ..
func (tm *ThemeManager) GetAvailableThemes() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var names []string
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

// -----------------------------------------------------------------------------
// ----------------------------- Theme Compilation -----------------------------
// -----------------------------------------------------------------------------

// CompileThemes searches and compiles all themes in the themes directory
func (tm *ThemeManager) CompileThemes() error {
	themesDir := "themes/"

	logging.LogInfo("start compiling themes")

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()
		themeDir := filepath.Join(themesDir, themeName)
		absOutPath, err := filepath.Abs(filepath.Join(themesDir, themeName+".so"))
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		cmd := exec.Command("go", "build", "-buildvcs=false", "-buildmode=plugin", "-o", absOutPath, ".")
		cmd.Dir = themeDir
		output, err := cmd.CombinedOutput()

		if err != nil {
			logging.LogError("failed to compile theme %s in %s: %v, %s", themeName, themeDir, err, output)
			return err
		}

		logging.LogInfo("compiled theme: %s", absOutPath)
	}

	return nil
}

// LoadTheme loads a specific theme plugin by name
func (tm *ThemeManager) LoadTheme(themeName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("start to load theme")

	if _, exists := tm.themes[themeName]; exists {
		return nil
	}

	themesDir := filepath.Join("themes/", themeName+".so")
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		return fmt.Errorf("theme plugin file not found: %s", themesDir)
	}

	plugin, err := plugin.Open(themesDir)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", themesDir, err)
	}

	themeSymbol, err := plugin.Lookup("Theme")
	if err != nil {
		return fmt.Errorf("failed to find 'Theme' symbol in plugin %s: %w", themeName, err)
	}
	theme, ok := themeSymbol.(ITheme)
	if !ok {
		return fmt.Errorf("theme %s does not implement ITheme interface", themeName)
	}

	tm.themes[themeName] = theme
	logging.LogInfo("successfully loaded theme: %s", themeName)

	return nil
}

// LoadAllThemes loads all available theme plugins from the themes directory
func (tm *ThemeManager) LoadAllThemes() error {
	themesDir := "themes/"

	logging.LogInfo("loading all themes from %s", themesDir)

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("failed to read themes directory: %w", err)
	}

	var loadErrors []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeName := entry.Name()

		tm.mutex.RLock()
		_, exists := tm.themes[themeName]
		tm.mutex.RUnlock()

		if exists {
			logging.LogError("theme %s is already loaded, skipping", themeName)
		} else {
			err := tm.LoadTheme(themeName)
			if err != nil {
				errMsg := fmt.Sprintf("failed to load theme %s: %v", themeName, err)
				logging.LogError(errMsg)
				loadErrors = append(loadErrors, errMsg)
				continue
			}
		}

	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load some themes: %v", loadErrors)
	}

	logging.LogInfo("successfully loaded all available themes")
	return nil
}
