package parser

import (
	"fmt"
	"html"
	"regexp"
)

type TOCItem struct {
	Level int
	Text  string
	ID    string
	Class string
	Link  string
}

// TOCFromMarkdown builds a document's table of contents straight from its raw markdown
// headings (see Headings), rather than re-parsing the renderer's HTML output - so it
// can't drift from goldmark's rendered heading shape.
func TOCFromMarkdown(lines []string) []TOCItem {
	return HeadingsToTOC(Headings(lines))
}

// HeadingsToTOC turns already-scanned headings into TOC entries. Callers that
// already have a Heading slice from MarkdownHandler.RenderWithUsedIDs should
// use this directly instead of TOCFromMarkdown, which would re-scan the
// document and risk a second, independently-numbered id sequence.
func HeadingsToTOC(headings []Heading) []TOCItem {
	toc := make([]TOCItem, 0, len(headings))
	for _, h := range headings {
		toc = append(toc, TOCItem{
			Level: h.Level,
			Text:  HeadingDisplayText(h.Text),
			ID:    h.ID,
			Class: fmt.Sprintf("toc-level-%d", h.Level),
			Link:  "#" + h.ID,
		})
	}
	return toc
}

func stripHTMLTags(s string) string {
	stripped := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
	return html.UnescapeString(stripped)
}
