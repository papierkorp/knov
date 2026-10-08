package parser

import (
	"strings"
	"testing"
	"unicode/utf8"

	"knov/internal/test/specialchars"
)

// fuzzPath reports whether p can be a file path the codec has to write: a "//" reads as a host
// (and is no path segment). not encoded yet, reported (see docs/temp_todo.md): ascii control
// characters besides tab and line breaks (a markdown destination can't hold them, "\f" ends it)
// and "`" (two of them are a code span, so the walker skips the link).
func fuzzPath(p string) bool {
	return p != "" && utf8.ValidString(p) && !strings.Contains(p, "//") && !strings.ContainsFunc(p, func(r rune) bool {
		return (r < 0x20 || r == 0x7f) && r != '\t' && r != '\n' && r != '\r' || r == '`'
	})
}

// for any file path and link kind, the codec reads back what it wrote, also through the link
// walker: ParseLink(Dest) and RewriteLinks see the same path, and writing it back unchanged keeps
// the content as it was
func FuzzLinkCodec(f *testing.F) {
	for _, p := range specialchars.Names {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if !fuzzPath(p) {
			t.Skip()
		}
		for _, kind := range []LinkKind{LinkMarkdown, LinkWiki, LinkHTML} {
			dest := Link{Kind: kind, Path: p}.Dest()
			if got := ParseLink(dest, kind); got.Path != p || got.External {
				t.Errorf("kind %d: ParseLink(%q) = %+v, want path %q", kind, dest, got, p)
			}
		}
		for _, content := range []string{
			Link{Kind: LinkMarkdown, Text: "x", Path: p}.String(),
			Link{Kind: LinkMarkdown, Image: true, Text: "x", Path: p}.String(),
			Link{Kind: LinkWiki, Path: p}.String(),
			"[id]: " + Link{Kind: LinkMarkdown, Path: p}.Dest(),
			`<img src="` + Link{Kind: LinkHTML, Path: p}.Dest() + `">`,
		} {
			var got []string
			out, changed := RewriteLinks(content, func(l Link) (string, bool) {
				got = append(got, l.Path)
				return l.Path, true
			})
			if changed || out != content || len(got) != 1 || got[0] != p {
				t.Errorf("RewriteLinks(%q) = %q, %v, read %q, want %q", content, out, changed, got, p)
			}
		}
	})
}

// RewriteLinks writing every path back unchanged never changes any content
func FuzzRewriteLinksIdentity(f *testing.F) {
	for _, p := range specialchars.Names {
		f.Add("[x](" + p + ") [[" + p + "]] ![y](<" + p + ">)\n[id]: " + p + "\n<a href=\"" + p + "\">`[c](" + p + ")`")
	}
	f.Add("[a\nb](a.md) `x\n[b](b.md)` [c](c.md)\n\n```\n[d](d.md)\n```\r\n[id]: e.md\r\n")
	f.Fuzz(func(t *testing.T, content string) {
		if out, changed := RewriteLinks(content, func(l Link) (string, bool) { return l.Path, true }); changed || out != content {
			t.Errorf("RewriteLinks(%q) = %q, %v", content, out, changed)
		}
	})
}
