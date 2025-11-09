// Package translation ..
package translation

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"knov/internal/logging"
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
	return globalPrinter.Sprintf(key, args...)
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
