// Package plugins ..
package plugins

import (
	"net/http"
	"os"
	"strings"
)

// GetCustomCSSEditor ..
func GetCustomCSSEditor(editorHTML string) string {
	css, _ := os.ReadFile("config/custom.css")
	return strings.Replace(editorHTML, "{{CSS_CONTENT}}", string(css), 1)

}

// HandleCustomCSS ..
func HandleCustomCSS(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()

		css := r.FormValue("css")
		err := os.WriteFile("config/custom.css", []byte(css), 0644)

		if err != nil {
			http.Error(w, "Failed to save CSS", http.StatusInternalServerError)
			return
		}

		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
	}
}
