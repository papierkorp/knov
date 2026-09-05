// Package markdown holds fence-aware scanning primitives for raw markdown text: a
// per-line "inside a fenced code block" mask (FenceMask), ATX heading extraction
// (ScanHeadings) and the canonical YAML front matter split (SplitFrontMatter).
//
// It stops at the syntactic level: ScanHeadings gives a heading's level, raw text
// and line but no id, since ids depend on the renderer - parser.Headings adds
// them. FenceMask and CodeBlocks share one scanner (scanFences).
package markdown

import (
	"bytes"
	"strings"
)

// FenceMask reports, per line, whether that line sits inside a ``` or ~~~ fenced
// code block (the opening and closing fence lines included). Opening and closing
// follow the same rules as CodeBlocks. An unterminated fence runs to the end of
// content (this is a choice, not a guaranteed match for every CommonMark renderer).
func FenceMask(lines []string) []bool {
	inside := make([]bool, len(lines))
	for _, cb := range scanFences(lines) {
		for i := cb.Start; i <= cb.End; i++ {
			inside[i] = true
		}
	}
	return inside
}

// RawHeading is one ATX heading ("# text") found outside any fenced code block.
// It carries no id: see parser.Headings for renderer-matching ids.
type RawHeading struct {
	Level int    // number of leading '#', always 1-6
	Text  string // heading text, leading '#'s and surrounding whitespace removed
	Line  int    // 0-based index of the heading's line in the original content
}

// ScanHeadings extracts every ATX heading from raw markdown (pre-split into
// lines): 1-6 leading '#' followed by a space (or end of line), outside fenced
// code blocks and outside a leading "---" front matter block. This approximates
// the CommonMark ATX rule - see ATXHeading for the corners it does not cover.
func ScanHeadings(lines []string) []RawHeading {
	mask := FenceMask(lines)
	bodyStart := frontMatterBodyLine(lines)

	var headings []RawHeading
	for i, line := range lines {
		if i < bodyStart || mask[i] {
			continue
		}
		level, text, ok := ATXHeading(line)
		if !ok {
			continue
		}
		headings = append(headings, RawHeading{Level: level, Text: text, Line: i})
	}
	return headings
}

// ATXHeading approximates CommonMark's ATX heading rule for one line: 1-6 leading '#',
// then a space/tab or end of line, returning the level (1-6) and trimmed text. Returns
// ok=false otherwise. Known simplifications: it trims any amount of leading indentation
// (CommonMark stops at 3 spaces) and it keeps a trailing "##" closing sequence in the
// text (CommonMark strips it).
func ATXHeading(line string) (level int, text string, ok bool) {
	t := strings.TrimSpace(line)
	level = len(t) - len(strings.TrimLeft(t, "#"))
	if level < 1 || level > 6 {
		return 0, "", false
	}
	rest := t[level:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return 0, "", false
	}
	return level, strings.TrimSpace(rest), true
}

// SplitFrontMatter splits content into (frontmatterYAML, body). frontmatter is
// nil when content does not open with a "---\n ... \n---\n" block. This is the
// single definition of "what is front matter"; parser.StripFrontMatter* and the
// heading scanner both route through here.
func SplitFrontMatter(content []byte) (frontmatter, body []byte) {
	delimiter := []byte("---\n")
	closing := []byte("\n---\n")

	if !bytes.HasPrefix(content, delimiter) {
		return nil, content
	}
	rest := content[len(delimiter):]
	idx := bytes.Index(rest, closing)
	if idx < 0 {
		return nil, content // malformed — leave untouched
	}
	return rest[:idx], rest[idx+len(closing):]
}

// frontMatterBodyLine returns the first body line index when content (as lines
// split on "\n") opens with a front matter block, or 0 when it does not. The
// block is "---" on line 0, the YAML lines, a closing "---" line, then the body.
func frontMatterBodyLine(lines []string) int {
	fm, _ := SplitFrontMatter([]byte(strings.Join(lines, "\n")))
	if fm == nil {
		return 0
	}
	// line 0 "---" + the YAML lines + the closing "---" line.
	return strings.Count(string(fm), "\n") + 3
}

// CodeBlock is one fenced code block located by CodeBlocks.
type CodeBlock struct {
	Indent       string // leading whitespace of the opening fence line
	Lang         string // info string after the fence marker, trimmed ("" when absent)
	Body         string // block content with the fence lines removed, lines joined by "\n"
	Start        int    // 0-based index of the opening fence line
	End          int    // 0-based index of the block's last line: the closing fence line,
	//                      or the final content line when Unterminated. Always in range.
	Unterminated bool // no closing fence was found; the block runs to the end of content
}

// CodeBlocks locates every ``` or ~~~ fenced code block in content (pre-split
// into lines). A fence opens on a line whose trimmed text starts with three or
// more backticks or tildes; it closes on a later line whose trimmed text is
// nothing but that same character repeated at least as many times as the opening
// run (CommonMark: a longer run closes a shorter one, a shorter run does not, and
// an info string on the line never closes). An unterminated fence runs to the end
// of content.
func CodeBlocks(lines []string) []CodeBlock {
	blocks := scanFences(lines)
	for i := range blocks {
		bodyEnd := blocks[i].End // closing fence line — not part of the body
		if blocks[i].Unterminated {
			bodyEnd = blocks[i].End + 1 // no closing fence — the last line is content
		}
		blocks[i].Body = strings.Join(lines[blocks[i].Start+1:bodyEnd], "\n")
	}
	return blocks
}

// scanFences is the shared fence scanner: it returns every fenced block's
// position (Start/End), indent and info string, without materializing Body.
func scanFences(lines []string) []CodeBlock {
	var blocks []CodeBlock
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "```") && !strings.HasPrefix(t, "~~~") {
			continue
		}
		ch := t[0]
		openLen := len(t) - len(strings.TrimLeft(t, string(ch)))

		cb := CodeBlock{
			Indent:       lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))],
			Lang:         strings.TrimSpace(strings.TrimLeft(t, string(ch))),
			Start:        i,
			End:          len(lines) - 1,
			Unterminated: true,
		}
		for j := i + 1; j < len(lines); j++ {
			if closesFence(lines[j], ch, openLen) {
				cb.End, cb.Unterminated = j, false
				break
			}
		}
		blocks = append(blocks, cb)
		i = cb.End // the loop's i++ then steps past the closing fence (or past EOF)
	}
	return blocks
}

// closesFence reports whether line closes a fence opened with openLen occurrences
// of ch ('`' or '~'): only that character, at least openLen of them (CommonMark
// requires the closing run to be no shorter than the opening run), nothing else.
func closesFence(line string, ch byte, openLen int) bool {
	t := strings.TrimSpace(line)
	if len(t) < openLen {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != ch {
			return false
		}
	}
	return true
}

// StripFencedBlocks drops every ``` or ~~~ fenced code block (fence lines
// included) from content (pre-split into lines) and returns the remaining lines
// joined by "\n". Used where code content must not be scanned for links or prose.
func StripFencedBlocks(lines []string) string {
	mask := FenceMask(lines)

	var kept []string
	for i, line := range lines {
		if !mask[i] {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
