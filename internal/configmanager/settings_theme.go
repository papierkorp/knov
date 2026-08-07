package configmanager

import (
	"encoding/json"
	"strings"
	"sync/atomic"

	"knov/internal/configStorage"
	"knov/internal/logging"
)

// ThemeSettings represents settings for a single theme
type ThemeSettings map[string]interface{}

// AllThemeSettings represents settings for all themes (theme name -> settings)
type AllThemeSettings map[string]ThemeSettings

// themeSettingsPrefix is the configStorage key prefix each theme is stored under,
// one file per theme (e.g. "themes/builtin" -> config/themes/builtin.json).
const themeSettingsPrefix = "themes/"

var themeSettingsStore atomic.Pointer[AllThemeSettings]

// LoadThemeSettings loads every per-theme settings file from storage into memory.
func LoadThemeSettings() {
	keys, err := configStorage.List(themeSettingsPrefix)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list theme settings: %v", err)
		return
	}
	all := make(AllThemeSettings, len(keys))
	for _, key := range keys {
		data, err := configStorage.Get(key)
		if err != nil || data == nil {
			continue
		}
		var settings ThemeSettings
		if err := json.Unmarshal(data, &settings); err != nil {
			logging.LogError(logging.KeyApp, "failed to decode theme settings %q: %v", key, err)
			continue
		}
		all[strings.TrimPrefix(key, themeSettingsPrefix)] = settings
	}
	themeSettingsStore.Store(&all)
	logging.LogInfo(logging.KeyApp, "theme settings loaded")
}

func getAllThemeSettings() AllThemeSettings {
	if p := themeSettingsStore.Load(); p != nil {
		return *p
	}
	return make(AllThemeSettings)
}

// GetThemeSetting returns a specific setting value for a theme
func GetThemeSetting(themeName, settingKey string) interface{} {
	if settings, exists := getAllThemeSettings()[themeName]; exists {
		return settings[settingKey]
	}
	return nil
}

// SetThemeSetting updates a specific setting value for a theme and persists just that
// theme's file. Copy-on-write: a fresh AllThemeSettings is built so existing
// getAllThemeSettings() snapshots remain immutable and safe to read without a lock.
func SetThemeSetting(themeName, settingKey string, value interface{}) {
	old := getAllThemeSettings()
	fresh := make(AllThemeSettings, len(old))
	for k, v := range old {
		inner := make(ThemeSettings, len(v))
		for ik, iv := range v {
			inner[ik] = iv
		}
		fresh[k] = inner
	}
	if fresh[themeName] == nil {
		fresh[themeName] = make(ThemeSettings)
	}
	fresh[themeName][settingKey] = value
	themeSettingsStore.Store(&fresh)

	data, err := json.Marshal(fresh[themeName])
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to marshal theme settings for %q: %v", themeName, err)
		return
	}
	if err := configStorage.Set(themeSettingsPrefix+themeName, data); err != nil {
		logging.LogError(logging.KeyApp, "failed to save theme settings for %q: %v", themeName, err)
	}
}

// GetCurrentThemeSettings returns all settings for the current theme.
func GetCurrentThemeSettings() ThemeSettings {
	if settings, exists := getAllThemeSettings()[Theme.Get()]; exists {
		return settings
	}
	return make(ThemeSettings)
}
