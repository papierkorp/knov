// Package translation is temporary created in the translation script and can be deleted
package translation

import "golang.org/x/text/message"

func init() {
    p := message.NewPrinter(message.MatchLanguage("en"))
    _ = p.Sprintf("Select Theme:")
}
