// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/git"
	"knov/internal/parser"
	"knov/internal/translation"
)

// RenderGitHistoryFileList renders a list of git history files as HTML
func RenderGitHistoryFileList(files []git.GitHistoryFile) string {
	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range files {
		linkPath := contentStorage.ToRelativePath(file.Path)
		html.WriteString(fmt.Sprintf(`<li>%s - <a href="/files/%s"><strong>%s</strong></a> (%s)</li>`,
			file.Date,
			linkPath,
			file.Name,
			file.Message))
	}
	html.WriteString("</ul>")
	return html.String()
}

// RenderFileVersionsList renders list of file versions as HTML
// output can be "full", "sidebar", or "compact"
func RenderFileVersionsList(versions []git.FileVersion, filePath string, output string) string {
	if len(versions) == 0 {
		return `<div class="no-versions">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no version history available") + `</div>`
	}

	var html strings.Builder

	switch output {
	case "sidebar":
		html.WriteString(`<div class="version-sidebar">`)
		html.WriteString(`<ul class="version-list">`)

		maxVersions := 5
		if len(versions) < maxVersions {
			maxVersions = len(versions)
		}

		for i := 0; i < maxVersions; i++ {
			version := versions[i]
			cssClass := "sidebar-version"
			if version.IsCurrent {
				cssClass += " current"
			}

			html.WriteString(fmt.Sprintf(`
				<li class="%s">
					<a href="/files/history/%s?commit=%s">
						<span class="version-date">%s:</span>
						<span class="version-message">%s</span>
					</a>
				</li>`,
				cssClass,
				contentStorage.ToRelativePath(filePath),
				version.Commit,
				version.Date,
				version.Message,
			))
		}

		html.WriteString(`</ul>`)

		if len(versions) > 5 {
			html.WriteString(fmt.Sprintf(`
				<a href="/files/history/%s" class="view-all-versions">
					%s
				</a>`,
				contentStorage.ToRelativePath(filePath),
				translation.SprintfForRequest(configmanager.GetLanguage(), "view all %d versions", len(versions)),
			))
		}

		html.WriteString(`</div>`)

	case "compact":
		html.WriteString(`<div class="file-versions-compact">`)
		html.WriteString(`<ul class="version-list">`)

		for _, version := range versions {
			cssClass := "version-item"
			if version.IsCurrent {
				cssClass += " current-version"
			}

			html.WriteString(fmt.Sprintf(`
				<li class="%s">
					<span class="version-date">%s</span> - <span class="version-message">%s</span>
				</li>`,
				cssClass,
				version.Date,
				version.Message,
			))
		}

		html.WriteString(`</ul>`)
		html.WriteString(`</div>`)

	default: // "full"
		html.WriteString(`<div class="file-versions-list">`)
		html.WriteString(`<ul class="version-list">`)

		for _, version := range versions {
			cssClass := "version-item"
			if version.IsCurrent {
				cssClass += " current-version"
			}

			html.WriteString(fmt.Sprintf(`
				<li class="%s">
					<div class="version-header">
						<span class="version-date">%s:</span>
						<span class="version-message">%s</span>
						<span class="version-author">%s %s</span>
					</div>
					<div class="version-actions">
						<a href="/files/history/%s?commit=%s" class="action-link">%s</a>
					</div>
				</li>`,
				cssClass,
				version.Date,
				version.Message,
				translation.SprintfForRequest(configmanager.GetLanguage(), "by"),
				version.Author,
				contentStorage.ToRelativePath(filePath),
				version.Commit,
				translation.SprintfForRequest(configmanager.GetLanguage(), "view"),
			))
		}

		html.WriteString(`</ul>`)
		html.WriteString(`</div>`)
	}

	return html.String()
}

// RenderFileAtVersion renders file content at a specific version
// output can be "full" (with title) or "content" (without title)
func RenderFileAtVersion(content, filePath, commit, date, message string, output string) string {
	var html strings.Builder
	html.WriteString(`<div class="file-version-content">`)

	if output != "content" { // default to showing title unless explicitly set to "content"
		html.WriteString(fmt.Sprintf(`<div class="version-header">
			<h3>%s %s %s - %s (%s)</h3>
		</div>`,
			filePath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "at"),
			date,
			message,
			commit))
	}

	html.WriteString(`<pre class="file-content">`)
	html.WriteString(content)
	html.WriteString(`</pre></div>`)
	return html.String()
}

// RenderFileDiff renders diff between two file versions with syntax highlighting
func RenderFileDiff(diff, filePath, fromCommit, toCommit string) string {
	var html strings.Builder
	html.WriteString(`<div class="file-diff-content">`)
	html.WriteString(fmt.Sprintf(`<div class="diff-header">
		<h3>%s: %s</h3>
		<p>%s | %s</p>
	</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "diff"),
		filePath,
		fromCommit,
		toCommit))

	// apply syntax highlighting to diff output
	highlightedDiff := parser.HighlightCodeBlock(diff, "diff")
	html.WriteString(highlightedDiff)

	html.WriteString(`</div>`)
	return html.String()
}
