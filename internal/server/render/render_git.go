// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	htmlpkg "html"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// RenderGitHistoryFileList renders a list of git history files as HTML.
// nextOffset is the offset to use for the load more button; hasMore controls whether to show it.
func RenderGitHistoryFileList(files []git.GitHistoryFile, collection, folder string, nextOffset int, hasMore bool) string {
	var b strings.Builder
	b.WriteString("<ul>")
	for _, file := range files {
		linkPath := pathutils.ToRelative(file.Path)
		fmt.Fprintf(&b, `<li>%s - <a href="/files/%s"><strong>%s</strong></a> (%s)</li>`,
			configmanager.FormatDateTime(file.Date),
			linkPath,
			file.Name,
			strings.TrimSpace(file.Message))
	}
	b.WriteString("</ul>")
	if hasMore {
		url := fmt.Sprintf("/api/git/latestchanges?count=50&offset=%d", nextOffset)
		if collection != "" {
			url += "&collection=" + collection
		}
		if folder != "" {
			url += "&folder=" + folder
		}
		fmt.Fprintf(&b, `<button class="load-more-btn" hx-get="%s" hx-target="this" hx-swap="outerHTML" hx-headers='{"Accept":"text/html"}'>%s</button>`,
			url, translation.SprintfForRequest(configmanager.GetLanguage(), "load more"))
	}
	return b.String()
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
				pathutils.ToRelative(filePath),
				version.Commit,
				configmanager.FormatDateTime(version.Date),
				version.Message,
			))
		}

		html.WriteString(`</ul>`)

		if len(versions) > 5 {
			html.WriteString(fmt.Sprintf(`
				<a href="/files/history/%s" class="view-all-versions">
					%s
				</a>`,
				pathutils.ToRelative(filePath),
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
				configmanager.FormatDateTime(version.Date),
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
				configmanager.FormatDateTime(version.Date),
				version.Message,
				translation.SprintfForRequest(configmanager.GetLanguage(), "by"),
				version.Author,
				pathutils.ToRelative(filePath),
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

// FileDiffVersion describes one side of a file diff for display.
type FileDiffVersion struct {
	Commit    string
	Date      string
	Message   string
	Content   string
	IsCurrent bool
}

// RenderFileDiff renders the diff between two file versions with syntax highlighting,
// plus the full before/after file content so the change is unambiguous even
// without reading the unified diff.
func RenderFileDiff(diff, filePath string, before, after FileDiffVersion) string {
	var html strings.Builder
	html.WriteString(`<div class="file-diff-content">`)
	html.WriteString(fmt.Sprintf(`<div class="diff-header">
		<h3>%s: %s</h3>
		<p>%s: %s (%s) &rarr; %s: %s (%s)</p>
	</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "diff"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "before"),
		before.Date, before.Commit,
		translation.SprintfForRequest(configmanager.GetLanguage(), "after"),
		after.Date, after.Commit))

	// apply syntax highlighting to diff output
	highlightedDiff := parser.HighlightCodeBlock(diff, "diff")
	html.WriteString(highlightedDiff)

	html.WriteString(`<div id="component-diff-fullfiles">`)
	html.WriteString(renderDiffFullFile(translation.SprintfForRequest(configmanager.GetLanguage(), "before"), filePath, before))
	html.WriteString(renderDiffFullFile(translation.SprintfForRequest(configmanager.GetLanguage(), "after"), filePath, after))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}

func renderDiffFullFile(label, filePath string, v FileDiffVersion) string {
	restoreBtn := ""
	if !v.IsCurrent {
		restoreBtn = fmt.Sprintf(`<form hx-post="/api/files/versions/restore/%s" hx-swap="none" class="diff-fullfile-restore">
			<input type="hidden" name="commit" value="%s">
			<button type="submit" class="action-button action-restore">%s</button>
		</form>`, filePath, v.Commit, translation.SprintfForRequest(configmanager.GetLanguage(), "restore this version"))
	}
	return fmt.Sprintf(`<details class="diff-fullfile">
		<summary>%s: %s (%s)</summary>
		%s
		<pre class="file-content">%s</pre>
	</details>`, label, v.Date, v.Commit, restoreBtn, htmlpkg.EscapeString(v.Content))
}

// RenderConflictDiff renders a live text diff between the current file and a conflict copy.
// Uses go-diff to produce a unified-style diff, then reuses the existing diff CSS.
func RenderConflictDiff(originalPath, conflictPath string) string {
	originalContent, err := os.ReadFile(originalPath)
	if err != nil {
		return `<div class="diff-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read original file") + `</div>`
	}
	conflictContent, err := os.ReadFile(conflictPath)
	if err != nil {
		return `<div class="diff-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read conflict file") + `</div>`
	}

	dmp := diffmatchpatch.New()
	// use line-level diff for readable output
	aChars, bChars, lines := dmp.DiffLinesToChars(string(originalContent), string(conflictContent))
	diffs := dmp.DiffMain(aChars, bChars, false)
	dmp.DiffCleanupSemantic(diffs)
	diffs = dmp.DiffCharsToLines(diffs, lines)

	// build unified-style diff text for syntax highlighting
	var sb strings.Builder
	for _, d := range diffs {
		for _, line := range strings.Split(d.Text, "\n") {
			if line == "" {
				continue
			}
			switch d.Type {
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(&sb, "-%s\n", line)
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(&sb, "+%s\n", line)
			case diffmatchpatch.DiffEqual:
				fmt.Fprintf(&sb, " %s\n", line)
			}
		}
	}

	origName := filepath.Base(originalPath)
	conflictName := filepath.Base(conflictPath)
	highlighted := parser.HighlightCodeBlock(sb.String(), "diff")

	var html strings.Builder
	html.WriteString(`<div class="file-diff-content">`)
	fmt.Fprintf(&html, `<div class="diff-header">
		<h3>%s</h3>
		<p>%s: %s &rarr; %s</p>
	</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "conflict diff"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "comparing"),
		origName, conflictName)
	html.WriteString(highlighted)
	html.WriteString(`</div>`)
	return html.String()
}
