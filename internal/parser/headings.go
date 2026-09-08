package parser

import (
	"knov/internal/markdown"
	"knov/internal/utils"
)

// Heading is a markdown.RawHeading plus the id the renderer assigns it.
type Heading struct {
	Level int
	Text  string
	ID    string
	Line  int
}

// Headings scans raw markdown (pre-split into lines) for ATX headings and gives
// each its id (see HeadingID), duplicates deduped in document order. This is the
// pre-render heading source (section editing, header autocomplete); the markdown
// renderer computes the same ids at render time through the shared SlugHeading,
// so a rendered anchor and its scan entry always agree.
func Headings(lines []string) []Heading {
	raw := markdown.ScanHeadings(lines)
	usedIDs := make(map[string]int)
	out := make([]Heading, len(raw))
	for i, h := range raw {
		out[i] = Heading{Level: h.Level, Text: h.Text, ID: HeadingID(h.Text, usedIDs), Line: h.Line}
	}
	return out
}

// HeadingID derives the id a heading with the given raw text ends up with in the
// rendered page: resolve [[wikilinks]] and internal links to their final markdown
// (so "[[page|Alias]]" ids off "Alias", like the rendered anchor), render the
// inline markdown, then SlugHeading. usedIDs carries the collision counts across
// a document (pass one shared map).
func HeadingID(text string, usedIDs map[string]int) string {
	resolved := ProcessMarkdownLinks(ResolveWikiLinks(text))
	return SlugHeading(RenderHeadingInline(resolved), usedIDs)
}

// SlugHeading turns a heading's rendered inline HTML into its anchor id: strip
// tags, then slug and dedupe via utils.GenerateID. Both HeadingID (the pre-render
// scan) and the render-time heading renderer route through here so the two id
// computations cannot drift.
func SlugHeading(inlineHTML string, usedIDs map[string]int) string {
	return utils.GenerateID(stripHTMLTags(inlineHTML), usedIDs)
}
