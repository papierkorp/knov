// Package plugins ..
package configmanager

import (
	"os"
	"strings"
)

// GetCustomCSSEditor ..
func GetCustomCSSEditor(editorHTML string) string {
	css, _ := os.ReadFile("api/config/custom.css")
	return strings.Replace(editorHTML, "{{CSS_CONTENT}}", string(css), 1)

}
