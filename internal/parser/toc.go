package parser

import (
	"fmt"
	"html"
	"regexp"
	"strings"
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

// EditorHeading is a heading of a document being edited: its level, the visible text a TOC shows
// and its 0-based line.
type EditorHeading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Line  int    `json:"line"`
}

// EditorHeadings lists the headings of unsaved markdown content for the editor's live TOC, with
// the same rules and display text as the TOC of the rendered page (TOCFromMarkdown).
func EditorHeadings(content string) []EditorHeading {
	headings := Headings(strings.Split(content, "\n"))
	out := make([]EditorHeading, len(headings))
	for i, h := range headings {
		out[i] = EditorHeading{Level: h.Level, Text: HeadingDisplayText(h.Text), Line: h.Line}
	}
	return out
}
