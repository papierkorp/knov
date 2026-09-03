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
// each the id the rendered HTML gets (see HeadingID), with duplicates deduped in
// document order the same way InjectHeaderIDs dedupes them on the real HTML.
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
// rendered page, by running the text through the same steps the display pipeline
// applies to a heading line: resolve [[wikilinks]] and internal links to their
// final markdown (so "[[page|Alias]]" ids off "Alias", like the rendered anchor),
// render the inline markdown, strip the tags, slug (utils.GenerateID). This is
// the read side of InjectHeaderIDs; keep the two in sync. usedIDs carries the
// collision counts across a document (pass one shared map).
func HeadingID(text string, usedIDs map[string]int) string {
	resolved := ProcessMarkdownLinks(ResolveWikiLinks(text))
	return utils.GenerateID(stripHTMLTags(RenderInlineMarkdown(resolved)), usedIDs)
}
