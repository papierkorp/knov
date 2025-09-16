// Package configmanager ..
package configmanager

import (
	"knov/internal/logging"
	"knov/internal/translation"
)

// Init initializes both app config and user settings
func Init() {
	InitAppConfig()
	InitUserSettings("default") // default user for now
}

// -----------------------------------------------------------------------------
// ---------------------------------- Language ---------------------------------
// -----------------------------------------------------------------------------

type Language struct {
	Code string
	Name string
}

func GetAvailableLanguages() []Language {
	return []Language{
		{Code: "en", Name: "English"},
		{Code: "de", Name: "Deutsch"},
	}
}

func CheckLanguage(lang string) string {
	if lang == "" {
		return "en"
	}

	availableLanguages := GetAvailableLanguages()
	for _, availableLang := range availableLanguages {
		if availableLang.Code == lang {
			return lang
		}
	}

	logging.LogWarning("language '%s' not supported, falling back to 'en'", lang)
	return "en"
}

// GetLanguage returns current language from user settings
func GetLanguage() string {
	return CheckLanguage(userSettings.Language)
}

// SetLanguage updates user settings with new language
func SetLanguage(lang string) {
	validLang := CheckLanguage(lang)
	userSettings.Language = validLang
	saveUserSettings()
	translation.SetLanguage(validLang)
}

// GetTheme returns current theme from user settings
func GetTheme() string {
	if userSettings.Theme == "" {
		return "builtin"
	}
	return userSettings.Theme
}

// SetTheme updates user settings with new theme
func SetTheme(theme string) {
	userSettings.Theme = theme
	saveUserSettings()
}
