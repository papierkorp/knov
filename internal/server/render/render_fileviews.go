// Package render - file views: the alternative ways a file can be shown on its /files/ page
package render

import (
	"fmt"
	htmlpkg "html"
	"net/url"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// fileView is one way to show a file. render returns ok=false when the view can't
// render the file, so the file's regular content is shown instead.
type fileView struct {
	id, label, icon string
	render          func(relPath string) (string, bool)
}

var (
	// a saved filter shows its live results, probed by path so a paired file
	// with missing/stale metadata still renders live
	viewRendered = fileView{"rendered", "rendered", "fa-eye", RenderFilterFileView}
	viewRaw      = fileView{"raw", "raw", "fa-code", renderRawFileView}
)

// fileViewsFor lists the views of relPath's file type; the first one is the default.
func fileViewsFor(relPath string) []fileView {
	if files.ResolveEditor(pathutils.ToWithPrefix(relPath)) == files.EditorTypeTracker {
		return []fileView{
			{"stats", "statistics", "fa-chart-line", RenderTrackerFileView},
			{"counters", "counters", "fa-plus-minus", RenderTrackerClickView},
			viewRaw,
		}
	}
	return []fileView{viewRendered, viewRaw}
}

// activeFileView picks the view named viewID, or the first (default) one when empty/unknown.
func activeFileView(views []fileView, viewID string) fileView {
	for _, v := range views {
		if v.id == viewID {
			return v
		}
	}
	return views[0]
}

// RenderFileView renders relPath's file page body as the view named viewID (the file
// type's default when empty/unknown). html is the file's regular rendered content,
// shown when the view can't render.
func RenderFileView(relPath, viewID, html string) string {
	if body, ok := activeFileView(fileViewsFor(relPath), viewID).render(relPath); ok {
		return body
	}
	return html
}

// FileViewLink is one view of a file as a link, for a theme's view menu.
type FileViewLink struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Icon   string `json:"icon"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// FileViewLinks lists a link per view of relPath, marking the active one.
func FileViewLinks(relPath, viewID string) []FileViewLink {
	lang := configmanager.GetLanguage()
	views := fileViewsFor(relPath)
	active := activeFileView(views, viewID)
	links := make([]FileViewLink, len(views))
	for i, v := range views {
		links[i] = FileViewLink{v.id, translation.SprintfForRequest(lang, v.label), v.icon,
			pathutils.ToFileURL(pathutils.DocsPath(relPath)) + "?view=" + v.id, v.id == active.id}
	}
	return links
}

// RenderFileViewLinks renders a menu link per view.
func RenderFileViewLinks(links []FileViewLink) string {
	var h strings.Builder
	for _, l := range links {
		cls := "menu-item"
		if l.Active {
			cls += " active"
		}
		fmt.Fprintf(&h, `<a href="%s" class="%s"><i class="fa %s"></i> %s</a>`,
			htmlpkg.EscapeString(l.URL), cls, l.Icon, htmlpkg.EscapeString(l.Label))
	}
	return h.String()
}

// renderRawFileView shows the file's source as stored, loaded from the raw content api.
func renderRawFileView(relPath string) (string, bool) {
	return fmt.Sprintf(`<pre id="component-file-view-raw" hx-get="/api/files/raw?filepath=%s" hx-trigger="load" hx-swap="innerHTML"></pre>`,
		htmlpkg.EscapeString(url.QueryEscape(relPath))), true
}

// RenderRawContent renders a file's source as escaped text.
func RenderRawContent(content []byte) string {
	return htmlpkg.EscapeString(string(content))
}
