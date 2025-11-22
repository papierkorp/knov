package configmanager

import "knov/internal/logging"

// ThemeSettings represents settings for a single theme
type ThemeSettings map[string]interface{}

// AllThemeSettings represents settings for all themes (theme name -> settings)
type AllThemeSettings map[string]ThemeSettings

// GetThemeSetting returns a specific setting value for a theme
func GetThemeSetting(themeName, settingKey string) interface{} {
	if userSettings.ThemeSettings == nil {
		return nil
	}
	if themeSettings, exists := userSettings.ThemeSettings[themeName]; exists {
		return themeSettings[settingKey]
	}
	return nil
}

// SetThemeSetting updates a specific setting value for a theme
func SetThemeSetting(themeName, settingKey string, value interface{}) {
	if userSettings.ThemeSettings == nil {
		userSettings.ThemeSettings = make(AllThemeSettings)
	}

	if userSettings.ThemeSettings[themeName] == nil {
		userSettings.ThemeSettings[themeName] = make(ThemeSettings)
	}

	userSettings.ThemeSettings[themeName][settingKey] = value

	err := saveUserSettings()
	if err != nil {
		logging.LogError("failed to save theme setting: %v", err)
	}
}

// GetCurrentThemeSettings returns all settings for the current theme
func GetCurrentThemeSettings() ThemeSettings {
	if userSettings.ThemeSettings == nil {
		return make(ThemeSettings)
	}

	if settings, exists := userSettings.ThemeSettings[userSettings.Theme]; exists {
		return settings
	}

	return make(ThemeSettings)
}
