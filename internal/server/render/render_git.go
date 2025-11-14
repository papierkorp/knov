// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"knov/internal/git"
	"knov/internal/utils"
	"strings"
)

// RenderGitHistoryFileList renders a list of git history files as HTML
func RenderGitHistoryFileList(files []git.GitHistoryFile) string {
	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range files {
		linkPath := utils.ToRelativePath(file.Path)
		html.WriteString(fmt.Sprintf(`<li>%s - <a href="/files/%s"><strong>%s</strong></a> (%s)</li>`,
			file.Date,
			linkPath,
			file.Name,
			file.Message))
	}
	html.WriteString("</ul>")
	return html.String()
}
