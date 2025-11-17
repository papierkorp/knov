package configmanager

import "knov/internal/logging"

// ThemeSettings contains theme-specific configuration
type ThemeSettings struct {
	DarkMode     bool                   `json:"darkMode,omitempty"`
	ColorScheme  string                 `json:"colorScheme,omitempty"`
	CustomCSS    string                 `json:"customCSS,omitempty"`
	FontFamily   string                 `json:"fontFamily,omitempty"`
	SidebarWidth int                    `json:"sidebarWidth,omitempty"`
	CustomValues map[string]interface{} `json:"customValues,omitempty"`
}

func GetThemeSetting(themeName, settingKey string) interface{} {
	if userSettings.ThemeSettings == nil {
		return nil
	}
	if themeSettings, exists := userSettings.ThemeSettings[themeName]; exists {
		switch settingKey {
		case "darkMode":
			return themeSettings.DarkMode
		case "colorScheme":
			return themeSettings.ColorScheme
		case "customCSS":
			return themeSettings.CustomCSS
		case "sidebarWidth":
			return themeSettings.SidebarWidth
		case "fontFamily":
			return themeSettings.FontFamily
		default:
			if themeSettings.CustomValues != nil {
				return themeSettings.CustomValues[settingKey]
			}
		}
	}
	return nil
}

// SetThemeSetting would become:
func SetThemeSetting(themeName, settingKey string, value interface{}) {
	if userSettings.ThemeSettings == nil {
		userSettings.ThemeSettings = make(map[string]ThemeSettings)
	}

	themeSettings := userSettings.ThemeSettings[themeName]
	if themeSettings.CustomValues == nil {
		themeSettings.CustomValues = make(map[string]interface{})
	}

	switch settingKey {
	case "darkMode":
		if v, ok := value.(bool); ok {
			themeSettings.DarkMode = v
		}
	case "colorScheme":
		if v, ok := value.(string); ok {
			themeSettings.ColorScheme = v
		}
	case "customCSS":
		if v, ok := value.(string); ok {
			themeSettings.CustomCSS = v
		}
	case "sidebarWidth":
		if v, ok := value.(int); ok {
			themeSettings.SidebarWidth = v
		} else if v, ok := value.(float64); ok {
			themeSettings.SidebarWidth = int(v)
		}
	case "fontFamily":
		if v, ok := value.(string); ok {
			themeSettings.FontFamily = v
		}
	default:
		themeSettings.CustomValues[settingKey] = value
	}

	userSettings.ThemeSettings[themeName] = themeSettings
	err := saveUserSettings()
	if err != nil {
		logging.LogError("failed to save theme setting: %v", err)
	}
}

func GetCurrentThemeSettings() map[string]interface{} {
	if userSettings.ThemeSettings == nil {
		return make(map[string]interface{})
	}

	themeSettings := userSettings.ThemeSettings[userSettings.Theme]
	result := make(map[string]interface{})

	result["darkMode"] = themeSettings.DarkMode
	result["colorScheme"] = themeSettings.ColorScheme
	result["customCSS"] = themeSettings.CustomCSS
	result["sidebarWidth"] = themeSettings.SidebarWidth
	result["fontFamily"] = themeSettings.FontFamily

	for k, v := range themeSettings.CustomValues {
		result[k] = v
	}

	return result
}

// GetDarkMode returns current dark mode setting from theme settings
func GetDarkMode() bool {
	if value := GetThemeSetting(userSettings.Theme, "darkMode"); value != nil {
		if darkMode, ok := value.(bool); ok {
			return darkMode
		}
	}
	return false // default
}

// SetDarkMode updates dark mode setting
func SetDarkMode(enabled bool) {
	SetThemeSetting(userSettings.Theme, "darkMode", enabled)
}

// GetColorScheme returns current color scheme from theme settings
func GetColorScheme() string {
	if value := GetThemeSetting(userSettings.Theme, "colorScheme"); value != nil {
		if colorScheme, ok := value.(string); ok && colorScheme != "" {
			return colorScheme
		}
	}
	return "default" // default
}

// SetColorScheme updates color scheme setting
func SetColorScheme(scheme string) {
	SetThemeSetting(userSettings.Theme, "colorScheme", scheme)
}

// GetCustomCSS returns current custom CSS from theme settings
func GetCustomCSS() string {
	if value := GetThemeSetting(userSettings.Theme, "customCSS"); value != nil {
		if customCSS, ok := value.(string); ok {
			return customCSS
		}
	}
	return "" // default
}

// SetCustomCSS updates custom CSS setting
func SetCustomCSS(css string) {
	SetThemeSetting(userSettings.Theme, "customCSS", css)
}

// GetFontFamily returns current font family from theme settings
func GetFontFamily() string {
	if userSettings.ThemeSettings == nil {
		return "system"
	}

	themeSettings := userSettings.ThemeSettings[userSettings.Theme]
	if themeSettings.FontFamily != "" {
		return themeSettings.FontFamily
	}
	return "system" // default
}
