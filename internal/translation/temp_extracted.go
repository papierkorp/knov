// Package translation is temporary created in the translation script and can be deleted
package translation

import "golang.org/x/text/message"

func init() {
    p := message.NewPrinter(message.MatchLanguage("en"))
    _ = p.Sprintf("Custom CSS")
    _ = p.Sprintf("Dark Mode")
    _ = p.Sprintf("Data Path:")
    _ = p.Sprintf("Documentation")
    _ = p.Sprintf("English")
    _ = p.Sprintf("General Settings")
    _ = p.Sprintf("German")
    _ = p.Sprintf("Git Settings")
    _ = p.Sprintf("Hello World from templ at home in the dark!")
    _ = p.Sprintf("Help")
    _ = p.Sprintf("Home")
    _ = p.Sprintf("Playground")
    _ = p.Sprintf("Repository URL:")
    _ = p.Sprintf("Saves when you click outside...")
    _ = p.Sprintf("Search")
    _ = p.Sprintf("Select Language:")
    _ = p.Sprintf("Select Theme:")
    _ = p.Sprintf("Settings")
    _ = p.Sprintf("Theme Settings")
}
