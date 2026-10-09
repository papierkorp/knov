package parser

import (
	"slices"
	"testing"
)

// the editor's live TOC lists the headings of the rendered page: front matter and fenced code are
// skipped, closing # dropped, inline markdown and links shown as the text they render
func TestEditorHeadings(t *testing.T) {
	content := "---\ntitle: x\n# not a heading\n---\n# One **bold** [[page|Alias]] ##\n```\n# in code\n```\ntext\n## Two [x](a.md) `c`\n#nospace\n"
	got := EditorHeadings(content)
	want := []EditorHeading{{1, "One bold Alias", 4}, {2, "Two x c", 9}}
	if !slices.Equal(got, want) {
		t.Errorf("EditorHeadings = %+v, want %+v", got, want)
	}
	if got := EditorHeadings(""); len(got) != 0 {
		t.Errorf("EditorHeadings(\"\") = %+v, want none", got)
	}
}
