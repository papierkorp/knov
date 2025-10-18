// Package plugins ..
package configmanager

import (
	"strings"
)

// GetCustomCSSEditor ..
func GetCustomCSSEditor(editorHTML string) string {
	customCSS := GetUserSettings().CustomCSS
	return strings.Replace(editorHTML, "{{CSS_CONTENT}}", customCSS, 1)
}
