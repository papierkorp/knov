// Package translation ..
package translation

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:generate gotext -srclang=en update -out=catalog.go -lang=en,de knov/themes/builtin/templates

var globalPrinter *message.Printer

// Init ..
func Init() {
	globalPrinter = message.NewPrinter(language.English)
}

// SetLanguage ..
func SetLanguage(lang string) {
	tag, _ := language.MatchStrings(language.NewMatcher([]language.Tag{

		language.English,

		language.German,
	}), lang)

	globalPrinter = message.NewPrinter(tag)
}

// Sprintf to be used for text that has to be translated
func Sprintf(key string, args ...interface{}) string {
	return globalPrinter.Sprintf(key, args...)
}
