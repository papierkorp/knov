// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/thememanager"
)

// RenderThemeOptions renders theme options for select dropdown
func RenderThemeOptions(availableThemes []thememanager.Theme, currentTheme thememanager.Theme) string {
	var html strings.Builder
	for _, theme := range availableThemes {
		selected := ""
		if theme.Name == currentTheme.Name {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, theme.Name, selected, theme.Name))
	}
	return html.String()
}

// RenderThemeSettings renders theme settings as HTML for display
func RenderThemeSettings(settings interface{}, themeName string) string {
	return fmt.Sprintf(`<div id="theme-settings-%s">
		<h4>Settings for %s</h4>
		<pre>%+v</pre>
	</div>`, themeName, themeName, settings)
}
