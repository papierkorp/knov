// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"knov/internal/configmanager"
	"strings"
)

// RenderConfigDisplay renders the main configuration display with theme, language and data path
func RenderConfigDisplay(userSettings configmanager.UserSettings, appConfig configmanager.AppConfig) string {
	var html strings.Builder
	html.WriteString("<div class='config'>")
	html.WriteString(fmt.Sprintf("<p>theme: %s</p>", userSettings.Theme))
	html.WriteString(fmt.Sprintf("<p>language: %s</p>", userSettings.Language))
	html.WriteString(fmt.Sprintf("<p>data path: %s</p>", appConfig.DataPath))
	html.WriteString("</div>")
	return html.String()
}

// GetLanguageOptions returns language options for select dropdown
func GetLanguageOptions() []SelectOption {
	languages := configmanager.GetAvailableLanguages()
	options := make([]SelectOption, len(languages))
	for i, lang := range languages {
		options[i] = SelectOption{
			Value: lang.Code,
			Label: lang.Name,
		}
	}
	return options
}

// RenderCustomCSSTextarea renders the custom CSS editor textarea
func RenderCustomCSSTextarea(content string) string {
	extraAttrs := `style="width: 100%; font-family: monospace;" hx-post="/api/config/customcss" hx-trigger="blur" hx-swap="none"`
	return RenderTextarea("css", content, 20, extraAttrs)
}
