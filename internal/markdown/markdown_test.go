package markdown

import (
	"fmt"
	"strings"
	"testing"
)

// maskString renders a FenceMask as 'x' (inside) / '.' (outside) for readable assertions.
func maskString(content string) string {
	var b strings.Builder
	for _, in := range FenceMask(strings.Split(content, "\n")) {
		if in {
			b.WriteByte('x')
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}

func TestFenceMask(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"backtick block", "a\n```\ncode\n```\nb", ".xxx."},
		{"tilde block recognized the same as backtick", "a\n~~~\ncode\n~~~\nb", ".xxx."},
		{"unterminated fence runs to end of content", "a\n```\nstill going", ".xx"},
		// a later ```lang line does not close an open block (only a bare run of the fence
		// char does), so a "#" line between them stays masked.
		{"info string does not close", "a\n```\ncode\n```go\n# not a heading\n```\nb", ".xxxxx."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maskString(c.in); got != c.want {
				t.Errorf("FenceMask(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func codeBlocksOf(s string) []CodeBlock {
	return CodeBlocks(strings.Split(s, "\n"))
}

func blockStr(cb CodeBlock) string {
	return fmt.Sprintf("indent=%q lang=%q body=%q start=%d end=%d unterminated=%t", cb.Indent, cb.Lang, cb.Body, cb.Start, cb.End, cb.Unterminated)
}

func TestCodeBlocks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"lang and indent captured, fence lines stripped from body", "intro\n  ```go\n  x := 1\n  ```\nend", `1|indent="  " lang="go" body="  x := 1" start=1 end=3 unterminated=false`},
		{"tilde fences located like backtick", "~~~python\ny = 2\n~~~", `1|indent="" lang="python" body="y = 2" start=0 end=2 unterminated=false`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bs := codeBlocksOf(c.in)
			got := fmt.Sprintf("%d|%s", len(bs), blockStr(bs[0]))
			if got != c.want {
				t.Errorf("CodeBlocks(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// a 4-backtick line closes a 3-backtick block instead of swallowing the rest of the document.
func TestCodeBlocksLongerRunCloses(t *testing.T) {
	bs := codeBlocksOf("```\ncode\n````\nafter")
	got := fmt.Sprintf("%d|end=%d|body=%q", len(bs), bs[0].End, bs[0].Body)
	if want := `1|end=2|body="code"`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// CommonMark's asymmetry: a 3-backtick line does not close a block opened with 4 backticks,
// so a nested 3-backtick example stays inside it.
func TestCodeBlocksShorterRunDoesNotClose(t *testing.T) {
	bs := codeBlocksOf("````\n```\nstill code\n````\nafter")
	got := fmt.Sprintf("%d|end=%d|body=%q", len(bs), bs[0].End, bs[0].Body)
	if want := "1|end=3|body=\"```\\nstill code\""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// End is the last line index (in range) and Unterminated is set.
func TestCodeBlocksUnterminated(t *testing.T) {
	bs := codeBlocksOf("a\n```\nb\nc")
	got := fmt.Sprintf("%d|end=%d|unterminated=%t|body=%q", len(bs), bs[0].End, bs[0].Unterminated, bs[0].Body)
	if want := `1|end=3|unterminated=true|body="b\nc"`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// code spans close only at a backtick run of the same length, unmatched runs are text.
func TestSplitCodeSpans(t *testing.T) {
	cases := map[string]string{
		"a `b` c":           "a |`b`| c",
		"a ``b`c`` d":       "a |``b`c``| d",
		"a ``b` c":          "a ``b` c",
		"`a` b `c`":         "|`a`| b |`c`|",
		"a ` b `` c ``` d ": "a ` b `` c ``` d ",
	}
	for in, want := range cases {
		if got := strings.Join(SplitCodeSpans(in), "|"); got != want {
			t.Errorf("SplitCodeSpans(%q) = %q, want %q", in, got, want)
		}
	}
}
