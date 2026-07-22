package configmanager

import "knov/internal/logging"

// ThemeSettings represents settings for a single theme
type ThemeSettings map[string]interface{}

// AllThemeSettings represents settings for all themes (theme name -> settings)
type AllThemeSettings map[string]ThemeSettings

// GetThemeSetting returns a specific setting value for a theme
func GetThemeSetting(themeName, settingKey string) interface{} {
	if settings, exists := ThemeSettingsStore.Get()[themeName]; exists {
		return settings[settingKey]
	}
	return nil
}

// SetThemeSetting updates a specific setting value for a theme and persists.
// Copy-on-write: a fresh AllThemeSettings is built so existing Get() snapshots
// remain immutable and safe to read without a lock.
func SetThemeSetting(themeName, settingKey string, value interface{}) {
	old := ThemeSettingsStore.Get()
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
	ThemeSettingsStore.Set(fresh)
	if err := SaveSettings(); err != nil {
		logging.LogError(logging.KeyApp, "failed to save theme setting: %v", err)
	}
}

// GetCurrentThemeSettings returns all settings for the current theme.
func GetCurrentThemeSettings() ThemeSettings {
	if settings, exists := ThemeSettingsStore.Get()[Theme.Get()]; exists {
		return settings
	}
	return make(ThemeSettings)
}
