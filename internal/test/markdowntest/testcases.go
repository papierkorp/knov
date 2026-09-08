package markdowntest

import (
	"fmt"
	"strings"

	"knov/internal/markdown"
	"knov/internal/parser"
	"knov/internal/test"
)

// headingsOf and codeBlocksOf adapt the []string-input scanner API to the
// string literals the cases are written with. headingsOf goes through
// parser.Headings so the ids are the renderer-matching ones.
func headingsOf(s string) []parser.Heading {
	return parser.Headings(strings.Split(s, "\n"))
}

func codeBlocksOf(s string) []markdown.CodeBlock {
	return markdown.CodeBlocks(strings.Split(s, "\n"))
}

func result(name, expected, actual string) test.CaseResult {
	cr := test.CaseResult{Name: name, Expected: expected, Actual: actual, Success: expected == actual}
	if !cr.Success {
		cr.Error = fmt.Sprintf("%s: got %q, want %q", name, actual, expected)
	}
	return cr
}

// maskString renders a FenceMask as 'x' (inside) / '.' (outside) for readable assertions.
func maskString(content string) string {
	var b strings.Builder
	for _, in := range markdown.FenceMask(strings.Split(content, "\n")) {
		if in {
			b.WriteByte('x')
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}

// caseFenceMaskBacktick covers a ``` block: the fence lines and body are all marked inside.
func caseFenceMaskBacktick() test.CaseResult {
	return result("fence-mask-backtick", ".xxx.", maskString("a\n```\ncode\n```\nb"))
}

// caseFenceMaskTilde covers a ~~~ block being recognized the same as ```.
func caseFenceMaskTilde() test.CaseResult {
	return result("fence-mask-tilde", ".xxx.", maskString("a\n~~~\ncode\n~~~\nb"))
}

// caseFenceMaskUnterminated covers an unterminated fence running to the end of content.
func caseFenceMaskUnterminated() test.CaseResult {
	return result("fence-mask-unterminated", ".xx", maskString("a\n```\nstill going"))
}

// caseFenceMaskInfoStringDoesNotClose covers the shared close rule: a later ```lang line does
// not close an open block (only a bare run of the fence char does), so a "#" line between them
// stays masked.
func caseFenceMaskInfoStringDoesNotClose() test.CaseResult {
	return result("fence-mask-info-string-no-close", ".xxxxx.", maskString("a\n```\ncode\n```go\n# not a heading\n```\nb"))
}

// caseHeadingsRequireSpaceAndLevel covers the ATX guardrails we do enforce: no space after
// the '#'s, or more than six '#'s, means it is not a heading.
func caseHeadingsRequireSpaceAndLevel() test.CaseResult {
	hs := headingsOf("#nospace\n####### toomany\n## Real One")
	got := fmt.Sprintf("%d:%s", len(hs), idOf(hs, 0))
	return result("headings-require-space-and-level", "1:real-one", got)
}

// caseHeadingsSkipFrontMatter covers a "#" line inside a leading "---" front matter block not
// being treated as a heading (the renderer strips front matter before parsing).
func caseHeadingsSkipFrontMatter() test.CaseResult {
	hs := headingsOf("---\ntitle: x\n# yaml comment\n---\n# Real Heading")
	got := fmt.Sprintf("%d:%s", len(hs), idOf(hs, 0))
	return result("headings-skip-front-matter", "1:real-heading", got)
}

// caseHeadingsSkipFenced covers a "#" line inside a fence not being treated as a heading.
func caseHeadingsSkipFenced() test.CaseResult {
	hs := headingsOf("# Real\n```\n# Fake\n```\n## Also Real")
	got := fmt.Sprintf("%d:%s,%s", len(hs), idOf(hs, 0), idOf(hs, 1))
	return result("headings-skip-fenced", "2:real,also-real", got)
}

// caseHeadingsDedupIDs covers repeated heading text getting the renderer's "-1" suffix from
// the shared usedIDs map.
func caseHeadingsDedupIDs() test.CaseResult {
	hs := headingsOf("## Notes\n## Notes\n## Notes")
	got := fmt.Sprintf("%s,%s,%s", idOf(hs, 0), idOf(hs, 1), idOf(hs, 2))
	return result("headings-dedup-ids", "notes,notes-1,notes-2", got)
}

// caseHeadingsUnicodeID covers unicode letters surviving in the id (not dropped as ASCII-only
// slugging would), matching the rendered anchor ids (parser.SlugHeading).
func caseHeadingsUnicodeID() test.CaseResult {
	hs := headingsOf("# Persönliche Übersicht")
	return result("headings-unicode-id", "persönliche-übersicht", idOf(hs, 0))
}

// caseStripFencedBlocksBothMarkers covers both fence styles being removed while surrounding
// lines are kept verbatim.
func caseStripFencedBlocksBothMarkers() test.CaseResult {
	in := "keep 1\n```\ndrop\n```\nkeep 2\n~~~\ndrop\n~~~\nkeep 3"
	return result("strip-fenced-blocks-both-markers", "keep 1\nkeep 2\nkeep 3", markdown.StripFencedBlocks(strings.Split(in, "\n")))
}

// caseHeadingsLinkID covers a heading whose text is a markdown link: the id is slugged from
// the link's visible text ("see-the-docs"), not its url, matching the rendered HTML.
func caseHeadingsLinkID() test.CaseResult {
	hs := headingsOf("## See [the docs](http://example.com/x)")
	return result("headings-link-id", "see-the-docs", idOf(hs, 0))
}

// caseHeadingsWikiLinkAliasID covers a heading that is a [[path|Alias]] wikilink: the id is
// slugged from the alias ("overview"), like the rendered anchor, not from the path.
func caseHeadingsWikiLinkAliasID() test.CaseResult {
	hs := headingsOf("## [[some/page|Overview]]")
	return result("headings-wikilink-alias-id", "overview", idOf(hs, 0))
}

// caseHeadingScanMatchesRenderIDs is the end-to-end guard for the refactor's core
// invariant: the ids parser.Headings computes from raw source (used by section
// editing) must equal the ids the markdown renderer actually puts on the <hN>
// tags (read back here via GenerateTOC, as the autocomplete endpoint does). The
// two sides share SlugHeading but are fed differently resolved text, so this pins
// that they still agree across links, wikilink aliases, unicode, dedupe order,
// numbered headings, inline formatting and a trailing "##".
func caseHeadingScanMatchesRenderIDs() test.CaseResult {
	name := "heading-scan-matches-render-ids"
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
	for _, h := range parser.Headings(strings.Split(src, "\n")) {
		scanIDs = append(scanIDs, h.ID)
	}

	h := parser.NewMarkdownHandler()
	parsed, err := h.Parse([]byte(src))
	if err != nil {
		return test.CaseResult{Name: name, Success: false, Error: err.Error()}
	}
	rendered, err := h.Render(parsed, "note.md")
	if err != nil {
		return test.CaseResult{Name: name, Success: false, Error: err.Error()}
	}
	var renderIDs []string
	for _, item := range parser.GenerateTOC(string(rendered)) {
		renderIDs = append(renderIDs, item.ID)
	}

	scan, render := strings.Join(scanIDs, ","), strings.Join(renderIDs, ",")
	got := fmt.Sprintf("scan=%s render=%s", scan, render)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("scan=%s render=%s", want, want),
		Actual:   got,
		Success:  scan == want && render == want,
	}
	if !cr.Success {
		cr.Error = "pre-render scan ids and rendered anchor ids diverged"
	}
	return cr
}

func idOf(hs []parser.Heading, i int) string {
	if i >= len(hs) {
		return "<none>"
	}
	return hs[i].ID
}

func blockStr(cb markdown.CodeBlock) string {
	return fmt.Sprintf("indent=%q lang=%q body=%q start=%d end=%d unterminated=%t", cb.Indent, cb.Lang, cb.Body, cb.Start, cb.End, cb.Unterminated)
}

// caseCodeBlocksLangAndIndent covers capturing the opening fence indent + info string and the
// body with fence lines stripped.
func caseCodeBlocksLangAndIndent() test.CaseResult {
	bs := codeBlocksOf("intro\n  ```go\n  x := 1\n  ```\nend")
	got := fmt.Sprintf("%d|%s", len(bs), blockStr(bs[0]))
	return result("codeblocks-lang-and-indent", `1|indent="  " lang="go" body="  x := 1" start=1 end=3 unterminated=false`, got)
}

// caseCodeBlocksTilde covers ~~~ fences being located just like ```.
func caseCodeBlocksTilde() test.CaseResult {
	bs := codeBlocksOf("~~~python\ny = 2\n~~~")
	got := fmt.Sprintf("%d|%s", len(bs), blockStr(bs[0]))
	return result("codeblocks-tilde", `1|indent="" lang="python" body="y = 2" start=0 end=2 unterminated=false`, got)
}

// caseCodeBlocksLongerRunCloses covers a 4-backtick line closing a 3-backtick block instead of
// swallowing the rest of the document.
func caseCodeBlocksLongerRunCloses() test.CaseResult {
	bs := codeBlocksOf("```\ncode\n````\nafter")
	got := fmt.Sprintf("%d|end=%d|body=%q", len(bs), bs[0].End, bs[0].Body)
	return result("codeblocks-longer-run-closes", `1|end=2|body="code"`, got)
}

// caseCodeBlocksShorterRunDoesNotClose covers CommonMark's asymmetry: a 3-backtick line does
// not close a block opened with 4 backticks, so a nested 3-backtick example stays inside it.
func caseCodeBlocksShorterRunDoesNotClose() test.CaseResult {
	bs := codeBlocksOf("````\n```\nstill code\n````\nafter")
	got := fmt.Sprintf("%d|end=%d|body=%q", len(bs), bs[0].End, bs[0].Body)
	return result("codeblocks-shorter-run-does-not-close", "1|end=3|body=\"```\\nstill code\"", got)
}

// caseCodeBlocksUnterminated covers an unterminated fence running to the end of content: End
// is the last line index (in range) and Unterminated is set.
func caseCodeBlocksUnterminated() test.CaseResult {
	bs := codeBlocksOf("a\n```\nb\nc")
	got := fmt.Sprintf("%d|end=%d|unterminated=%t|body=%q", len(bs), bs[0].End, bs[0].Unterminated, bs[0].Body)
	return result("codeblocks-unterminated", `1|end=3|unterminated=true|body="b\nc"`, got)
}

// caseRenderLongerRunFenceDoesNotSwallow is an end-to-end check that the shared CodeBlocks
// scanner lets a 4-backtick line close a 3-backtick block, so a heading right after it still
// renders instead of being eaten into the code block to the end of the file.
func caseRenderLongerRunFenceDoesNotSwallow() test.CaseResult {
	name := "render-longer-run-fence-does-not-swallow"
	in := "```\ncode\n````\n\n# Real Heading After\n"
	out, err := parser.NewMarkdownHandler().Render([]byte(in), "note.md")
	if err != nil {
		cr := test.CaseResult{Name: name, Success: false, Error: err.Error()}
		return cr
	}
	got := string(out)
	success := strings.Contains(got, "Real Heading After") && strings.Contains(got, "<h1")
	cr := test.CaseResult{
		Name:     name,
		Expected: `rendered HTML has an <h1> "Real Heading After"`,
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "heading after a 4-backtick close was swallowed into the code block"
	}
	return cr
}
