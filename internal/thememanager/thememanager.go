// Package thememanager ...
package thememanager

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"sync"

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
	log.Println("initialize thememanager ...")

	err := tm.CompileThemes()
	if err != nil {
		log.Printf("failed compiling the themes: %s", err)
	}

	err = tm.LoadAllThemes()
	if err != nil {
		log.Printf("failed to load all themes: %v", err)
	}

	// todo: set to load from config
	// currentTheme := "defaulttheme"
	currentTheme := "dark"

	err = tm.SetCurrentTheme(currentTheme)
	if err != nil {
		log.Printf("failed to set current theme - %s: %v", currentTheme, err)
	}

	log.Println("theme loaded successfully")
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

	log.Println("start to set current theme")
	theme, ok := tm.themes[name]
	if !ok {
		log.Printf("theme %s not found", name)
		return fmt.Errorf("theme %s not found", name)
	}

	tm.currentTheme = theme
	log.Printf("switched to theme: %s", name)
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
	themesDir := "data/themes"

	log.Printf("start compiling themes")

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

		cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", absOutPath, ".")
		cmd.Dir = themeDir
		output, err := cmd.CombinedOutput()

		if err != nil {
			log.Printf("failed to compile theme %s: %v, %s", themeName, err, output)
			return err
		}

		log.Printf("compiled theme: %s", absOutPath)
	}

	return nil
}

// LoadTheme loads a specific theme plugin by name
func (tm *ThemeManager) LoadTheme(themeName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Println("start to load theme")

	if _, exists := tm.themes[themeName]; exists {
		return nil
	}

	pluginPath := filepath.Join("data/themes", themeName+".so")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("theme plugin file not found: %s", pluginPath)
	}

	plugin, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", pluginPath, err)
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
	log.Printf("successfully loaded theme: %s", themeName)

	return nil
}

// LoadAllThemes loads all available theme plugins from the themes directory
func (tm *ThemeManager) LoadAllThemes() error {
	themesDir := "data/themes"

	log.Printf("loading all themes from %s", themesDir)

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
			log.Printf("theme %s is already loaded, skipping", themeName)
		} else {
			err := tm.LoadTheme(themeName)
			if err != nil {
				errMsg := fmt.Sprintf("failed to load theme %s: %v", themeName, err)
				log.Printf(errMsg)
				loadErrors = append(loadErrors, errMsg)
				continue
			}
		}

	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load some themes: %v", loadErrors)
	}

	log.Printf("successfully loaded all available themes")
	return nil
}
