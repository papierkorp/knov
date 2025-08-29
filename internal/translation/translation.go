// Package translation ..
package translation

import (
	"log"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:generate sh -c "../../static/generate-translations.sh"

var globalPrinter *message.Printer

// Init ..
func Init() {
	globalPrinter = message.NewPrinter(language.English)
	log.Printf("DEBUG translation globalprinter: %v", globalPrinter)
}

// SetLanguage ..
func SetLanguage(lang string) {
	tag, _ := language.MatchStrings(language.NewMatcher([]language.Tag{
		language.English,
		language.German,
	}), lang)

	log.Printf("DEBUG translations setlanguage tag %s", tag)
	globalPrinter = message.NewPrinter(tag)
}

// Sprintf to be used for text that has to be translated
func Sprintf(key string, args ...interface{}) string {
	log.Printf("DEBUG translation sprintf")
	return globalPrinter.Sprintf(key, args...)
}
