// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"strings"
)

// RenderFileViewOptions renders file view options for select dropdown
func RenderFileViewOptions(views []string, selectedView string) string {
	options := make([]SelectOption, len(views))
	for i, view := range views {
		options[i] = SelectOption{
			Value: view,
			Label: strings.Title(view),
		}
	}
	return RenderSelectOptions(options, selectedView)
}
