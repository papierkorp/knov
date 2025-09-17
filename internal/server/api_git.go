package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/git"
)

// @Summary Get recently changed files
// @Tags git
// @Produce json,html
// @Router /api/git/latestchanges [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 100 // default
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	files, err := git.GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, "failed to get recent files", http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range files {
		linkPath := strings.TrimPrefix(file.Path, configmanager.GetAppConfig().DataPath+"/")
		html.WriteString(fmt.Sprintf(`<li>%s - <a href="/files/%s"><strong>%s</strong></a> (%s)</li>`,
			file.Date,
			linkPath,
			file.Name,
			file.Message))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, files, html.String())
}
