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