// Package translation ..
package translation

import (
	"knov/internal/logging"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:generate sh -c "../../static/generate-translations.sh"

var globalPrinter *message.Printer

// Init ..
func Init() {
	globalPrinter = message.NewPrinter(language.English)
	logging.LogDebug("translation globalprinter: %v", globalPrinter)
}

// SetLanguage ..
func SetLanguage(lang string) {
	tag, _ := language.MatchStrings(language.NewMatcher([]language.Tag{
		language.English,
		language.German,
	}), lang)

	logging.LogDebug("translations setlanguage tag %s", tag)
	globalPrinter = message.NewPrinter(tag)
}

// Sprintf to be used for text that has to be translated
func Sprintf(key string, args ...any) string {
	if globalPrinter == nil {
		logging.LogError("translation not initialized, using fallback for key: %s", key)
		return key
	}

	// Try to translate, but if translation is missing, fallback to the key itself
	translated := globalPrinter.Sprintf(key, args...)

	// If translation failed (returns same as input), log it for debugging
	if translated == key && len(args) == 0 {
		logging.LogDebug("translation missing for key: %s", key)
	}

	return translated
}

// SprintfForRequest creates a language-specific printer and translates text
// Use this for HTMX responses to get proper per-user translations
func SprintfForRequest(lang string, key string, args ...any) string {
	// Create language-specific printer
	tag, _ := language.MatchStrings(language.NewMatcher([]language.Tag{
		language.English,
		language.German,
	}), lang)

	printer := message.NewPrinter(tag)
	translated := printer.Sprintf(key, args...)

	// If translation failed, fallback to key
	if translated == key && len(args) == 0 {
		logging.LogDebug("translation missing for key: %s (lang: %s)", key, lang)
	}

	return translated
}

// func getBrowserLocale(r *http.Request) string {
// 	acceptLanguage := r.Header.Get("Accept-Language")
// 	if acceptLanguage == "" {
// 		// default
// 		return "en"
// 	}
//
// 	languages := strings.Split(acceptLanguage, ",")
// 	if len(languages) > 0 {
// 		locale := strings.Split(strings.TrimSpace(languages[0]), "-")[0]
// 		return locale
// 	}
//
// 	return "en"
// }
