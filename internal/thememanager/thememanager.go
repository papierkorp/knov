// Package thememanager ...
package thememanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"reflect"
	"slices"
	"strings"
	"sync"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
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
		themes:        make(map[string]ITheme),
		thememetadata: make(map[string]*ThemeMetadata),
	}
}

// ThemeManager ...
type ThemeManager struct {
	themes        map[string]ITheme
	currentTheme  ITheme
	thememetadata map[string]*ThemeMetadata
	mutex         sync.RWMutex
}

// ITheme ...
type ITheme interface {
	Home(viewName string) (templ.Component, error)
	Settings(viewName string) (templ.Component, error)
	Admin(viewName string) (templ.Component, error)
	Playground(viewName string) (templ.Component, error)
	LatestChanges(viewName string) (templ.Component, error)
	History(viewName string) (templ.Component, error)
	Search(viewName string, query string) (templ.Component, error)
	Overview(viewName string) (templ.Component, error)
	RenderFileView(viewName string, fileContent *files.FileContent, filePath string) (templ.Component, error)
	FileEdit(viewName string, content string, filePath string) (templ.Component, error)
	Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error)
	BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error)
}

// ColorScheme defines a pre-defined color scheme
type ColorScheme struct {
	Name  string
	Label string
}

// ThemeMetadata defines theme capabilities and available options
type ThemeMetadata struct {
	AvailableFileViews          []string
	AvailableHomeViews          []string
	AvailableSearchViews        []string
	AvailableOverviewViews      []string
	AvailableDashboardViews     []string
	AvailableSettingsViews      []string
	AvailableAdminViews         []string
	AvailablePlaygroundViews    []string
	AvailableHistoryViews       []string
	AvailableLatestChangesViews []string
	AvailableBrowseFilesViews   []string
	SupportsDarkMode            bool
	AvailableColorSchemes       []ColorScheme
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
	GetAvailableViews(viewType string) []string
	GetThemeMetadata(themeName string) *ThemeMetadata
}

// Initialize loads all themes from the themes directory
func (tm *ThemeManager) Initialize() {
	logging.LogInfo("initialize thememanager ...")

	// register builtin theme directly
	tm.registerBuiltinTheme()

	err := tm.CompileThemes()
	if err != nil {
		logging.LogWarning("failed compiling the themes: %s", err)
	}

	err = tm.LoadAllThemes()
	if err != nil {
		logging.LogWarning("failed to load all themes: %v", err)
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

// registerBuiltinTheme registers the builtin theme directly without plugin loading
func (tm *ThemeManager) registerBuiltinTheme() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	logging.LogInfo("registering builtin theme")
	tm.themes["builtin"] = &builtinTheme
	tm.thememetadata["builtin"] = &builtinMetadata
	logging.LogInfo("builtin theme registered successfully")
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

// GetAvailableViews returns available view variants for a specific view type
func (tm *ThemeManager) GetAvailableViews(viewType string) []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for name, theme := range tm.themes {
		if theme == tm.currentTheme {
			if meta, ok := tm.thememetadata[name]; ok {
				switch viewType {
				case "file":
					return meta.AvailableFileViews
				case "home":
					return meta.AvailableHomeViews
				case "search":
					return meta.AvailableSearchViews
				case "overview":
					return meta.AvailableOverviewViews
				case "dashboard":
					return meta.AvailableDashboardViews
				case "settings":
					return meta.AvailableSettingsViews
				case "admin":
					return meta.AvailableAdminViews
				case "playground":
					return meta.AvailablePlaygroundViews
				case "history":
					return meta.AvailableHistoryViews
				case "latestchanges":
					return meta.AvailableLatestChangesViews
				case "browsefiles":
					return meta.AvailableBrowseFilesViews
				}
			}
			break
		}
	}
	return []string{"default"}
}

// GetThemeMetadata returns metadata for a specific theme
func (tm *ThemeManager) GetThemeMetadata(themeName string) *ThemeMetadata {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if meta, ok := tm.thememetadata[themeName]; ok {
		return meta
	}
	return nil
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
		if !entry.IsDir() || entry.Name() == "builtin" {
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

	// skip builtin theme since it's already registered
	if themeName == "builtin" {
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
		themeType := reflect.TypeOf(themeSymbol)
		interfaceType := reflect.TypeOf((*ITheme)(nil)).Elem()

		var issues []string
		for i := 0; i < interfaceType.NumMethod(); i++ {
			ifaceMethod := interfaceType.Method(i)
			themeMethod, found := themeType.MethodByName(ifaceMethod.Name)

			if !found {
				issues = append(issues, fmt.Sprintf("%s: missing", ifaceMethod.Name))
			} else {
				if ifaceMethod.Type.NumIn() != themeMethod.Type.NumIn()-1 ||
					ifaceMethod.Type.NumOut() != themeMethod.Type.NumOut() {
					issues = append(issues, fmt.Sprintf("%s: wrong signature", ifaceMethod.Name))
				} else {
					for j := 0; j < ifaceMethod.Type.NumIn(); j++ {
						if ifaceMethod.Type.In(j) != themeMethod.Type.In(j+1) {
							issues = append(issues, fmt.Sprintf("%s: param %d should be %v, got %v",
								ifaceMethod.Name, j+1, ifaceMethod.Type.In(j), themeMethod.Type.In(j+1)))
							break
						}
					}
				}
			}
		}

		if len(issues) > 0 {
			return fmt.Errorf("theme %s does not implement ITheme interface:\n%v", themeName, strings.Join(issues, "\n"))
		}
		return fmt.Errorf("theme %s does not implement ITheme interface", themeName)
	}
	tm.themes[themeName] = theme

	metaSymbol, err := plugin.Lookup("Metadata")
	if err == nil {
		if meta, ok := metaSymbol.(*ThemeMetadata); ok {
			tm.thememetadata[themeName] = meta
			logging.LogInfo("loaded metadata for theme: %s", themeName)
		}
	}

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
		if !entry.IsDir() || entry.Name() == "builtin" {
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
