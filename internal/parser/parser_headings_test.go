package parser

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func headingsOf(s string) []Heading {
	return Headings(strings.Split(s, "\n"))
}

func idOf(hs []Heading, i int) string {
	if i >= len(hs) {
		return "<none>"
	}
	return hs[i].ID
}

// no space after the '#'s, or more than six '#'s, means it is not a heading.
func TestHeadingsRequireSpaceAndLevel(t *testing.T) {
	hs := headingsOf("#nospace\n####### toomany\n## Real One")
	got := fmt.Sprintf("%d:%s", len(hs), idOf(hs, 0))
	if want := "1:real-one"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// a "#" line inside a leading "---" front matter block is not treated as a heading (the
// renderer strips front matter before parsing).
func TestHeadingsSkipFrontMatter(t *testing.T) {
	hs := headingsOf("---\ntitle: x\n# yaml comment\n---\n# Real Heading")
	got := fmt.Sprintf("%d:%s", len(hs), idOf(hs, 0))
	if want := "1:real-heading"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// a "#" line inside a fence is not treated as a heading.
func TestHeadingsSkipFenced(t *testing.T) {
	hs := headingsOf("# Real\n```\n# Fake\n```\n## Also Real")
	got := fmt.Sprintf("%d:%s,%s", len(hs), idOf(hs, 0), idOf(hs, 1))
	if want := "2:real,also-real"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// repeated heading text gets the renderer's "-1" suffix from the shared usedIDs map.
func TestHeadingsDedupIDs(t *testing.T) {
	hs := headingsOf("## Notes\n## Notes\n## Notes")
	got := fmt.Sprintf("%s,%s,%s", idOf(hs, 0), idOf(hs, 1), idOf(hs, 2))
	if want := "notes,notes-1,notes-2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// unicode letters survive in the id (not dropped as ASCII-only slugging would), matching the
// rendered anchor ids (SlugHeading).
func TestHeadingsUnicodeID(t *testing.T) {
	hs := headingsOf("# Persönliche Übersicht")
	if got, want := idOf(hs, 0), "persönliche-übersicht"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// a heading whose text is a markdown link: the id is slugged from the link's visible text
// ("see-the-docs"), not its url, matching the rendered HTML.
func TestHeadingsLinkID(t *testing.T) {
	hs := headingsOf("## See [the docs](http://example.com/x)")
	if got, want := idOf(hs, 0), "see-the-docs"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// a heading that is a [[path|Alias]] wikilink: the id is slugged from the alias ("overview"),
// like the rendered anchor, not from the path.
func TestHeadingsWikiLinkAliasID(t *testing.T) {
	hs := headingsOf("## [[some/page|Overview]]")
	if got, want := idOf(hs, 0), "overview"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// renderedHeadingIDRe reads the id straight off each rendered <hN id="..."> tag, independent
// of Headings, so TestHeadingScanMatchesRenderIDs can catch the pre-render scan and the
// renderer drifting apart. Mirrors the id format knovNodeRenderer.renderHeading emits (see
// its "Emitted format contract" comment).
var renderedHeadingIDRe = regexp.MustCompile(`<h[1-6] id="([^"]*)"`)

func renderedHeadingIDs(htmlStr string) []string {
	var ids []string
	for _, m := range renderedHeadingIDRe.FindAllStringSubmatch(htmlStr, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// TestHeadingScanMatchesRenderIDs is the end-to-end guard for the refactor's core invariant:
// the ids Headings computes from raw source (used by section editing and TOCFromMarkdown)
// must equal the ids the markdown renderer actually puts on the <hN> tags (read back here via
// renderedHeadingIDs, straight off the rendered HTML, independent of Headings). The two sides
// share SlugHeading but are fed differently resolved text, so this pins that they still agree
// across links, wikilink aliases, unicode, dedupe order, numbered headings, inline formatting
// and a trailing "##".
func TestHeadingScanMatchesRenderIDs(t *testing.T) {
	src := "# Intro\n" +
		"## See [the docs](http://example.com/x)\n" +
		"## [[some/page|Overview]]\n" +
		"## Persönliche Übersicht\n" +
		"## Notes\n" +
		"## Notes\n" +
		"## 1. Introduction\n" +
		"## *Bold* and `code`\n" +
		"## Trailing ##\n"
	want := "intro,see-the-docs,overview,persönliche-übersicht,notes,notes-1,1-introduction,bold-and-code,trailing"

	var scanIDs []string
	for _, h := range headingsOf(src) {
		scanIDs = append(scanIDs, h.ID)
	}

	h := NewMarkdownHandler()
	parsed, err := h.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := h.Render(parsed, "note.md", true)
	if err != nil {
		t.Fatal(err)
	}
	renderIDs := renderedHeadingIDs(string(rendered))

	if scan := strings.Join(scanIDs, ","); scan != want {
		t.Errorf("pre-render scan ids = %q, want %q", scan, want)
	}
	if render := strings.Join(renderIDs, ","); render != want {
		t.Errorf("rendered anchor ids = %q, want %q", render, want)
	}
}

// sectionEditIDRe reads the section id straight off each rendered section-edit-btn's href.
var sectionEditIDRe = regexp.MustCompile(`\?section=([^"]*)"[^>]*class="section-edit-btn"`)

// TestWrapHeaderSectionsMatchesHeadingIDs guards sectionWriter (see its doc comment on
// parser_markdown.go): it must produce exactly one content-section per heading, each with a
// section-edit-btn keyed to that heading's id.
func TestWrapHeaderSectionsMatchesHeadingIDs(t *testing.T) {
	src := "# Intro\ntext\n## Notes\nmore\n## Notes\neven more\n"
	want := "intro,notes,notes-1"

	rendered, err := NewMarkdownHandler().Render([]byte(src), "note.md", true)
	if err != nil {
		t.Fatal(err)
	}
	got := string(rendered)

	if sections := strings.Count(got, `class="content-section"`); sections != 3 {
		t.Errorf("content-section count = %d, want 3", sections)
	}
	var sectionIDs []string
	for _, m := range sectionEditIDRe.FindAllStringSubmatch(got, -1) {
		sectionIDs = append(sectionIDs, m[1])
	}
	if gotIDs := strings.Join(sectionIDs, ","); gotIDs != want {
		t.Errorf("section-edit-btn ids = %q, want %q", gotIDs, want)
	}
}

// Render(..., editableSections: false) must omit both the per-heading header-edit-btn and the
// per-section section-edit-btn, since editors with no inline section editing (list, todo,
// tracker, filter, index, book) get no edit affordances anywhere in the rendered HTML.
func TestEditableSectionsFalseSuppressesEditButtons(t *testing.T) {
	src := "# Intro\ntext\n## Notes\nmore\n"

	rendered, err := NewMarkdownHandler().Render([]byte(src), "note.md", false)
	if err != nil {
		t.Fatal(err)
	}
	got := string(rendered)

	if strings.Contains(got, "header-edit-btn") || strings.Contains(got, "section-edit-btn") {
		t.Errorf("an edit button survived with editableSections=false: %s", got)
	}
}

// a 4-backtick line closes a 3-backtick block (markdown.CodeBlocks) instead of swallowing the
// rest of the document, so a heading right after it still renders.
func TestRenderLongerRunFenceDoesNotSwallow(t *testing.T) {
	in := "```\ncode\n````\n\n# Real Heading After\n"
	out, err := NewMarkdownHandler().Render([]byte(in), "note.md", true)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "Real Heading After") || !strings.Contains(got, "<h1") {
		t.Errorf("heading after a 4-backtick close was swallowed into the code block: %s", got)
	}
}
