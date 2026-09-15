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
	return HeadingsWithIDs(lines, make(map[string]int))
}

// HeadingsWithIDs is Headings, but collision counts are read from and written
// back to the caller's usedIDs map instead of a fresh one - see
// MarkdownHandler.RenderWithUsedIDs for why a caller would share one.
func HeadingsWithIDs(lines []string, usedIDs map[string]int) []Heading {
	raw := markdown.ScanHeadings(lines)
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
	return SlugHeading(headingInlineHTML(text), usedIDs)
}

// HeadingDisplayText renders a heading's raw markdown text into its visible plain-text
// form (links resolved, inline markdown rendered, tags stripped) - the TOC text for a
// heading, computed from the same resolution HeadingID slugs so the two never drift.
func HeadingDisplayText(text string) string {
	return stripHTMLTags(headingInlineHTML(text))
}

// headingInlineHTML resolves [[wikilinks]]/internal links and renders the inline
// markdown, shared by HeadingID (slugged) and HeadingDisplayText (tags stripped).
func headingInlineHTML(text string) string {
	return RenderHeadingInline(ProcessMarkdownLinks(ResolveWikiLinks(text)))
}

// SlugHeading turns a heading's rendered inline HTML into its anchor id: strip
// tags, then slug and dedupe via utils.GenerateID. Both HeadingID (the pre-render
// scan) and the render-time heading renderer route through here so the two id
// computations cannot drift.
func SlugHeading(inlineHTML string, usedIDs map[string]int) string {
	return utils.GenerateID(stripHTMLTags(inlineHTML), usedIDs)
}
