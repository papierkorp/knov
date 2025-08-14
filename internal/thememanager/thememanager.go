// Package thememanager ...
// Package thememanager ...
package thememanager

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/a-h/templ"
)

// ---------------------------------------------------------
// ------------------ GLOBAL THEMEMANAGER ------------------
// ---------------------------------------------------------

var globalThemeManager *ThemeManager

// ThemeManager ...
type ThemeManager struct {
	themes       map[string]ITheme
	themeInfo    map[string]ThemeInfo
	currentTheme ITheme
	mutex        sync.RWMutex
}

func GetThemeManager() IThemeManager {
	if globalThemeManager == nil {
		globalThemeManager = NewThemeManager()
		if err := globalThemeManager.Initialize(); err != nil {
			log.Fatalf("Failed to initialize theme manager: %v", err)
		}
	}
	return globalThemeManager
}

// ---------------------------------------------------------
// --------------------- GENERAL TYPES ---------------------
// ---------------------------------------------------------

// ITheme interface defines required methods for themes
type ITheme interface {
	Home() (templ.Component, error)
	Help() (templ.Component, error)
	Settings() (templ.Component, error)
	Search() (templ.Component, error)
	DocsRoot() (templ.Component, error)
	Docs(content string) (templ.Component, error)
	Playground() (templ.Component, error)
	Plugins() (templ.Component, error)
}

// ThemeInfo structure for theme metadata
type ThemeInfo struct {
	Name              string   `json:"name"`
	DisplayName       string   `json:"displayName"`
	Description       string   `json:"description"`
	Version           string   `json:"version"`
	Tags              []string `json:"tags"`
	Author            string   `json:"author"`
	SupportedFeatures []string `json:"supportedFeatures"`
	ThemeColor        string   `json:"themeColor"`
}

// ---------------------------------------------------------
// ------------------- ThemeManager Logic ------------------
// ---------------------------------------------------------

// IThemeManager ..
type IThemeManager interface {
	GetCurrentTheme() ITheme
	SetCurrentTheme(name string) error
	GetCurrentThemeName() string
	GetAvailableThemes() []string
	GetThemeInfo(name string) (ThemeInfo, bool)
	ListThemeInfo() map[string]ThemeInfo
	Initialize() error
}

// NewThemeManager ..
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		themes:    make(map[string]ITheme),
		themeInfo: make(map[string]ThemeInfo),
	}
}

// Initialize loads all themes from the themes directory
func (tm *ThemeManager) Initialize() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Println("Loading themes...")
	themes, themeInfos, err := loadThemes()
	if err != nil {
		return fmt.Errorf("failed to load themes: %v", err)
	}

	// Store loaded themes and their info
	for name, theme := range themes {
		tm.themes[name] = theme
		if info, exists := themeInfos[name]; exists {
			tm.themeInfo[name] = info
		} else {
			// Fallback info if theme.json is missing
			tm.themeInfo[name] = ThemeInfo{
				Name:        name,
				DisplayName: name,
				Description: "Theme loaded without metadata",
				Tags:        []string{"unknown"},
			}
		}
		log.Printf("✅ Loaded theme: %s (%s)", name, tm.themeInfo[name].DisplayName)
	}

	if len(tm.themes) == 0 {
		return fmt.Errorf("❌ No themes were loaded")
	}

	// Set default theme
	if theme, ok := tm.themes["default"]; ok {
		tm.currentTheme = theme
		log.Println("✅ Set default theme: default")
	} else {
		// Set first available theme as default
		for name, theme := range tm.themes {
			tm.currentTheme = theme
			log.Printf("⚠️  Default theme not found, using: %s", name)
			break
		}
	}

	log.Printf("✅ Theme Manager initialized with %d themes", len(tm.themes))
	return nil
}

// GetCurrentTheme ..
func (tm *ThemeManager) GetCurrentTheme() ITheme {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.currentTheme
}

// SetCurrentTheme ..
func (tm *ThemeManager) SetCurrentTheme(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	theme, ok := tm.themes[name]
	if !ok {
		return fmt.Errorf("theme %s not found", name)
	}

	tm.currentTheme = theme
	log.Printf("✅ Switched to theme: %s", name)
	return nil
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

// GetThemeInfo returns theme metadata for a specific theme
func (tm *ThemeManager) GetThemeInfo(name string) (ThemeInfo, bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	info, exists := tm.themeInfo[name]
	return info, exists
}

// ListThemeInfo returns all theme metadata
func (tm *ThemeManager) ListThemeInfo() map[string]ThemeInfo {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// Return a copy to prevent external modification
	infoCopy := make(map[string]ThemeInfo)
	for name, info := range tm.themeInfo {
		infoCopy[name] = info
	}
	return infoCopy
}

// loadThemeMetadata loads theme.json file for a theme
func loadThemeMetadata(themePath string) (ThemeInfo, error) {
	metadataPath := fmt.Sprintf("%s/theme.json", themePath)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return ThemeInfo{}, fmt.Errorf("failed to read theme metadata: %v", err)
	}

	var info ThemeInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return ThemeInfo{}, fmt.Errorf("failed to parse theme metadata: %v", err)
	}

	return info, nil
}
