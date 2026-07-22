// Package configmanager ..
package configmanager

import (
	"knov/internal/logging"
)

// -----------------------------------------------------------------------------
// ---------------------------------- Language ---------------------------------
// -----------------------------------------------------------------------------

// LanguageOption represents an available language choice.
type LanguageOption struct {
	Code string
	Name string
}

// GetAvailableLanguages ..
func GetAvailableLanguages() []LanguageOption {
	return []LanguageOption{
		{Code: "en", Name: "English"},
		{Code: "de", Name: "Deutsch"},
	}
}

// CheckLanguage validates a language code, falling back to "en" if unknown.
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

	logging.LogWarning(logging.KeyApp, "language '%s' not supported, falling back to 'en', availableLanguages: %v", lang, availableLanguages)
	return "en"
}

// GetLanguage returns current language from user settings
func GetLanguage() string {
	return CheckLanguage(Language.Get())
}

// SetLanguage updates user settings with new language
func SetLanguage(lang string) {
	validLang := CheckLanguage(lang)
	Language.SetFromString(validLang) //nolint:errcheck // pre-validated by CheckLanguage
	SaveSettings()                    //nolint:errcheck
}

// GetTheme returns the current theme name.
func GetTheme() string { return Theme.Get() }

// SetTheme updates the theme and persists.
func SetTheme(theme string) {
	Theme.SetFromString(theme) //nolint:errcheck // theme is a dynamic-select, no Options to validate against
	SaveSettings()             //nolint:errcheck
}
