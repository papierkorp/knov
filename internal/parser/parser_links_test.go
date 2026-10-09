package parser

import (
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"knov/internal/pathutils"
	"knov/internal/test/specialchars"
	"knov/internal/utils"
)

// "./", "../" and bare markdown links are relative to the rendered doc's folder, wikilinks and "/"
// ones to the docs root
func TestRenderRelativeLinks(t *testing.T) {
	in := "[y](../a.md) [[./b]] ![i](./c.png) [z](sub/d.md) [w](/d.md) [[d]] ![j](g.png)\n\n[r]: ./e.md\n[s]: ./f.png\n[t]: h.md"
	want := "[y](" + fileURL("x/a.md") + ") [b](" + fileURL("x/sub/b.md") + ") ![i](" + pathutils.ToMediaURL("x/sub/c.png") + ") [z](" +
		fileURL("x/sub/sub/d.md") + ") [w](" + fileURL("d.md") + ") [d](" + fileURL("d.md") +
		") ![j](" + pathutils.ToMediaURL("x/sub/g.png") + ")\n\n[r]: " + fileURL("x/sub/e.md") + "\n[s]: " + pathutils.ToMediaURL("x/sub/f.png") + "\n[t]: " + fileURL("x/sub/h.md")
	if got := RenderLinks(in, "docs/x/sub/n.md"); got != want {
		t.Errorf("RenderLinks(%q) = %q, want %q", in, got, want)
	}
}

func TestRenderLinks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		// empty link text falls back to the filename (no anchor).
		{"fallback label plain path", "[](note.md)", "[note](" + fileURL("note.md") + ")"},
		// empty link text falls back to "filename - Header Text".
		{"fallback label path plus anchor", "[](note.md#todo-vorlage)", "[note - Todo Vorlage](" + fileURL("note.md") + "#todo-vorlage)"},
		// media links are decoded once and re-encoded as a /media/ url
		{"files media url is the docs file docs/media/", "[x](/files/media/a%20b.png)", "[x](" + fileURL("media/a b.png") + ")"},
		{"media path", "[x](<media/a b.png>)", "[x](" + pathutils.ToMediaURL("a b.png") + "?mode=detail)"},
		// query, anchor and title are kept for media links too
		{"files media url query", "[x](/files/media/a.png?raw=1#p)", "[x](" + fileURL("media/a.png") + "?raw=1#p)"},
		{"media path query", `[x](media/a.png?raw=1#p "t")`, "[x](" + pathutils.ToMediaURL("a.png") + `?raw=1&mode=detail#p "t")`},
		// any scheme is external, like for link metadata - a one-letter one is a windows drive
		{"mailto external", "[x](mailto:a@b.c)", "[x](mailto:a@b.c)"},
		{"protocol-relative external", "[x](//cdn.example.com/a)", "[x](//cdn.example.com/a)"},
		{"external bad escape", "[x](https://example.com/100%)", "[x](https://example.com/100%)"},
		{"unc external", `[x](\\server\share)`, `[x](\\server\share)`},
		// query and title are split off like RewriteLinks does, one level of (...) is part of the path
		{"query", "[x](a.md?view=raw#sec)", "[x](" + fileURL("a.md") + "?view=raw#sec)"},
		{"title", `[x](a.md "t")`, "[x](" + fileURL("a.md") + ` "t")`},
		{"parens", "[x](a(1).md)", "[x](" + fileURL("a(1).md") + ")"},
		{"angled title", `[x](<a b.md#sec> "t")`, "[x](" + fileURL("a b.md") + `#sec "t")`},
		{"angled external", "[x](<https://example.com/a b>)", "[x](<https://example.com/a b>)"},
		{"media no ext", "[x](media/a)", "[x](" + pathutils.ToMediaURL("a") + "?mode=detail)"},
		// a same-page link: empty link text falls back to just the humanized header text, no
		// filename prefix.
		{"fallback label pure anchor", "[](#todo-vorlage)", "[Todo Vorlage](#todo-vorlage)"},
		// a visible heading text as anchor becomes the heading id, so the link stays whole
		{"anchor text slugged", "[x](<a.md#Phase 1: Plan>)", "[x](" + fileURL("a.md") + "#phase-1-plan)"},
		{"wiki anchor text slugged", "[x](/files/a.md#Phase 1)", "[x](" + fileURL("a.md") + "#phase-1)"},
		// a percent-encoded path segment (a space in the folder name) is decoded before the
		// fallback label is built from it.
		{"percent-encoded path decoded before label", "[](mein%20ordner/notiz.md#eintrag-eins)", "[notiz - Eintrag Eins](" + fileURL("mein ordner/notiz.md") + "#eintrag-eins)"},
		// a percent-encoded anchor segment is decoded before the fallback label is built, while
		// the href's anchor fragment itself is left exactly as written.
		{"percent-encoded anchor decoded before label", "[](notiz.md#einf%C3%BChrung-teil-1)", "[notiz - Einführung Teil 1](" + fileURL("notiz.md") + "#einf%C3%BChrung-teil-1)"},
		// external (non-relative) links are left exactly as written - no fallback label, no
		// /files/ routing.
		{"external link untouched", "[Example](https://example.com/some/path)", "[Example](https://example.com/some/path)"},
		// an image is written as the app url of its target (renderImage previews it).
		{"image embed untouched", "![Diagram](media/diagram.png)", "![Diagram](/media/diagram.png)"},
		// a Windows-style backslash path is normalized to forward slashes - the first
		// transformation the image path gets before it is written as a url.
		{"image embed backslash path normalized", `![Diagram](sub\diagram.png)`, "![Diagram](/media/sub/diagram.png)"},
		{"doc link backslash path normalized", `[](sub\note.md)`, "[note](" + fileURL("sub/note.md") + ")"},
		// a markdown escape (\_) is no separator, the anchor becomes its heading id
		{"doc link markdown escape resolved", `[x](a\_b.md#c\d)`, "[x](" + fileURL("a_b.md") + "#c-d)"},
		{"image embed markdown escape resolved", `![D](a\_b.png)`, `![D](/media/a_b.png)`},
		// in a windows path every "\" is a separator, also before punctuation (_resources)
		{"doc link windows punctuation folder", `[x](sub\_resources\a.md)`, "[x](" + fileURL("sub/_resources/a.md") + ")"},
		{"image embed title and anchor kept", `![D](a\_b.png#c\d "t\x")`, `![D](/media/a_b.png#c\d "t\x")`},
		{"image embed windows punctuation folder", `![D](sub\_resources\a.png)`, "![D](/media/sub/_resources/a.png)"},
		{"doc link backslash dot segments", `[x](a\..\b\.c.md)`, "[x](" + fileURL("b/.c.md") + ")"},
		// a /files/ url gets the default extension like link metadata reads it
		{"files url no ext", "[x](/files/a)", "[x](" + fileURL("a.md") + ")"},
		{"files url docs folder", "[x](/files/docs/a)", "[x](" + fileURL("docs/a.md") + ")"},
		{"media url", "[x](/media/a%20b.png)", "[x](" + pathutils.ToMediaURL("a b.png") + ")"},
		// a same-page heading text becomes the heading id too
		{"pure anchor text slugged", "[x](<#Phase 1>)", "[x](#phase-1)"},
		// code is never a link, an unclosed "<" is no destination
		{"inline code", "`[x](a.md)` [y](a.md)", "`[x](a.md)` [y](" + fileURL("a.md") + ")"},
		{"fenced code", "```\n[x](a.md)\n```", "```\n[x](a.md)\n```"},
		{"unclosed angle", "[x](<a.md) ![y](<b.png)", "[x](<a.md) ![y](<b.png)"},
		// a link text spanning lines keeps its text
		{"multi-line text", "[a\nb](a.md)", "[a\nb](" + fileURL("a.md") + ")"},
		// an image alt spanning lines or holding code stays an image
		{"multi-line image alt", "![a\nb](a%20b.png)", "![a\nb](/media/a%20b.png)"},
		{"image alt with code", "![`x` y](a.png)", "![`x` y](/media/a.png)"},
		{"bracket in code text", "[`]` x](a.md)", "[`]` x](" + fileURL("a.md") + ")"},
		{"unclosed angle after space", "[x]( <a.md)", "[x]( <a.md)"},
		// a linked image or one after a stray "[" stays an image
		{"linked image", "[![b](img.png)](a.md)", "[![b](/media/img.png)](" + fileURL("a.md") + ")"},
		{"linked image external", "[![b](img.png)](https://x.y)", "[![b](/media/img.png)](https://x.y)"},
		{"image after stray bracket", "a [stray\n\n![i](img.png)", "a [stray\n\n![i](/media/img.png)"},
		// a reference definition to a docs file gets its /files/ url like an inline link, media,
		// images, pure anchors and external ones stay as written
		{"ref def doc", "[r][id]\n\n[id]: <a b#Phase 1> \"t\"", "[r][id]\n\n[id]: " + fileURL("a b.md") + "#phase-1 \"t\""},
		{"ref def media image anchor external", "[a]: media/x.png\n[b]: pic.png\n[c]: #x\n[d]: https://x.y", "[a]: /media/x.png\n[b]: /media/pic.png\n[c]: #x\n[d]: https://x.y"},
		{"ref def in code", "```\n[id]: a.md\n```", "```\n[id]: a.md\n```"},
		// html src/href get their app url like markdown links, app routes, anchors and external ones stay
		{"html href", `<a href="a b.md">x</a> <img src='/media/p.png'>`, `<a href="` + fileURL("a b.md") + `">x</a> <img src='` + pathutils.ToMediaURL("p.png") + `'>`},
		{"html app route anchor external", `<a href="/dashboard"> <a href="#top"> <img src="https://x.y/p.png">`, `<a href="/dashboard"> <a href="#top"> <img src="https://x.y/p.png">`},
		{"html entity", `<a href="a&amp;b.md">`, `<a href="/files/a&amp;b.md">`},
		// an external wikilink links its url
		{"wiki external", "[[https://example.com/a]]", "[a](<https://example.com/a>)"},
		{"ref def like text with code span", "[id]: a.md `x`", "[id]: a.md `x`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RenderLinks(c.in, PathlessRender); got != c.want {
				t.Errorf("RenderLinks(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// humanizeSlug capitalizes unicode runes (e.g. German umlauts) correctly via unicode.ToUpper
// rather than a naive ASCII-only uppercase, for both a single-word and a multi-word
// (hyphenated) slug.
func TestUnicodeHeaderSlugCapitalization(t *testing.T) {
	if got, want := RenderLinks("[](#übersicht)", PathlessRender), "[Übersicht](#übersicht)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := RenderLinks("[](#persönliche-übersicht)", PathlessRender), "[Persönliche Übersicht](#persönliche-übersicht)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// wrapTrailingTodoDate (parser_todo.go), exercised end-to-end through MarkdownHandler.Render
// since it operates on the parsed AST, not on a string it's handed directly. Each case's
// trailing " (YYYY-MM-DD)" stamp must end up wrapped in exactly one "todo-date" span - never
// left as leftover plain text *and* duplicated into a styled span, which is the failure mode a
// broken overlap-resolution walk would silently produce instead of hitting the documented safe
// bail-out.
func TestTodoDateNoDuplication(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"single line", "- [ ] Buy milk (2024-01-15)"},
		{"cancelled placeholder", "- [-] Task done (2024-01-15)"},
		{"waiting placeholder", "- [O] Task waiting (2024-01-15)"},
		{"hyphenated word before date", "- [ ] pre-order item - final (2024-01-15)"},
		{"multi-line wrapped", "- [ ] first line\n  second line (2024-01-15)"},
	}
	const wantSpan = `<span class="todo-date">(2024-01-15)</span>`

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := NewMarkdownHandler().Render([]byte(c.in), PathlessRender, false)
			if err != nil {
				t.Fatal(err)
			}
			got := string(out)
			if n := strings.Count(got, "2024-01-15"); n != 1 {
				t.Errorf("date appears %d times, want 1: %s", n, got)
			}
			if !strings.Contains(got, wantSpan) {
				t.Errorf("date not wrapped in %s: %s", wantSpan, got)
			}
		})
	}
}

// ResolveWikiTarget - the wikilink-body normalizer shared with internal/book: drop the
// "|alias" and "#anchor", default a missing extension to ".md", URL-decode the
// path, and yield an empty path for a pure "[[#header]]" anchor.
func TestWikiTargetExtraction(t *testing.T) {
	cases := []struct {
		in, wantPath string
	}{
		{"note", "note.md"},
		{"note.md", "note.md"},
		{"sub/note.md#section", "sub/note.md"},
		{"note#section|Display Text", "note.md"},
		{"#header", ""},
		{"mein%20ordner/notiz", "mein ordner/notiz.md"},
		{`sub.d\note`, "sub.d/note.md"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := ResolveWikiTarget(c.in); got != c.wantPath {
				t.Errorf("ResolveWikiTarget(%q) = %q, want %q", c.in, got, c.wantPath)
			}
		})
	}
}

// a wikilink anchor holding a quote or ")" stays the whole anchor, it isn't read as a title or
// the end of the destination
func TestWikiLinkAnchorSpecialChars(t *testing.T) {
	for in, want := range map[string]string{
		`[[a#Say "hi"]]`: "[a - Say \"hi\"](" + fileURL("a.md") + "#say-hi)",
		`[[a#x) y|z]]`:   "[z](" + fileURL("a.md") + "#x-y)",
		`[[#a "b"]]`:     `[A "b"](#a-b)`,
	} {
		if got := RenderLinks(in, PathlessRender); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

// RenderLinks keeps a pure "[[#header]]" as a real same-page link (labelled with the header text)
// rather than routing it through /files/ or collapsing it to ".". A wikilink in code stays as
// written, a wikilink-like path in a markdown destination is no wikilink.
func TestWikiLinkPureAnchor(t *testing.T) {
	for in, want := range map[string]string{
		"[[#some-header]]":         "[Some Header](#some-header)",
		"`[[a]]`\n```\n[[a]]\n```": "`[[a]]`\n```\n[[a]]\n```",
		"[x](a[[b]].md)":           "[x](" + fileURL("a[[b]].md") + ")",
	} {
		if got := RenderLinks(in, PathlessRender); got != want {
			t.Errorf("RenderLinks(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractLinksDestination(t *testing.T) {
	in := `![x](</media/a%20b.png> "title") [y](note.md 'title') ![z](img/c.png) <img src="/media/d.png"> <a href='#top'> <a href="mailto:a@b.c"> <img src="data:image/png;base64,AAAA"> <img data-src="lazy.png"> <script src="//cdn.example.com/x.js"> <a href="/dashboard"> set src="prose.png" [[ns:page]] [[https://example.com]] [[\\server\share]] ![w](C:\x.png) [e](a\_b.md) [f](sub\_res\a.md)
[ref]: <ref img.png> "title"
[note]: remember this
[^1]: footnote text`
	want := []string{"media/a b.png", "docs/note.md", "docs/img/c.png", "media/d.png", "docs/ns:page.md", "docs/C:/x.png", "docs/a_b.md", "docs/sub/_res/a.md", "docs/ref img.png"}
	if got := NewMarkdownHandler().ExtractLinks([]byte(in), PathlessRender); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ExtractLinks(%q) = %q, want %q", in, got, want)
	}
}

// a "\" that isn't a markdown escape makes the whole path a windows path, so every "\" is a
// separator - otherwise all "\" are markdown escapes.
func TestMarkdownLinkPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{`a\_b.md`, "a_b.md"},                          // escape only
		{`a\\b.md`, "a/b.md"},                          // escaped backslash
		{`sub\note.md`, "sub/note.md"},                 // letter after "\"
		{`sub\1.md`, "sub/1.md"},                       // digit after "\"
		{`sub\_resources\a.md`, "sub/_resources/a.md"}, // punctuation folder
		{`a\..\b.md`, "a/../b.md"},                     // "\." is no escape
		{`..\.git\x`, "../.git/x"},                     // hidden folder
		{`sub\`, "sub/"},                               // trailing "\"
		{`_a\_b.md`, "_a_b.md"},                        // ambiguous, read as escapes
	}
	for _, c := range cases {
		if got := markdownLinkPath(c.in); got != c.want {
			t.Errorf("markdownLinkPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRewriteLinksBackslashes(t *testing.T) {
	var got []string
	RewriteLinks(`[a](sub\_res\x.md) [b](a\_b.md) [[sub\_res\y]]`, func(l Link) (string, bool) {
		got = append(got, l.Path)
		return "", false
	})
	// markdown links get their escapes or windows separators resolved, wiki links every "\" as "/"
	want := []string{"sub/_res/x.md", "a_b.md", "sub/_res/y"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("RewriteLinks paths = %q, want %q", got, want)
	}
}

// every special-char file name, written by Link.Dest (or as an app url) in each link form,
// is read back as the same file by every reader: link metadata, wikilink resolving and the
// rendered href / media preview
func TestSpecialCharLinksRoundTrip(t *testing.T) {
	h := NewMarkdownHandler()
	for _, p := range specialchars.Names {
		md, wiki := encodeLinkPath(p, LinkMarkdown), encodeLinkPath(p, LinkWiki)
		img := strings.Replace(p, ".md", ".png", 1)
		writers := []struct{ name, link, want string }{
			{"markdown", "[x](" + md + ")", p},
			{"markdown <>", "[x](<" + md + ">)", p},
			{"markdown anchor", "[x](" + md + "#sec)", p},
			{"markdown file url", "[x](" + fileURL(p) + ")", p},
			{"wiki", "[[" + wiki + "]]", p},
			{"wiki anchor", "[[" + wiki + "#sec]]", p},
			{"image", "![x](" + encodeLinkPath("media/"+img, LinkMarkdown) + ")", "media/" + img},
			{"image media url", "![x](" + pathutils.ToMediaURL(img) + ")", "media/" + img},
			{"markdown media url", "[x](" + pathutils.ToMediaURL(img) + ")", "media/" + img},
			{"markdown media path", "[x](" + encodeLinkPath("/media/"+img, LinkMarkdown) + ")", "media/" + img},
		}
		// the extensionless form is only written when it reads as the same file (see renameLinkFunc)
		if bare := strings.TrimSuffix(p, ".md"); utils.WithDefaultLinkExt(bare) == p {
			writers = append(writers, struct{ name, link, want string }{"wiki no ext", "[[" + encodeLinkPath(bare, LinkWiki) + "]]", p})
		}
		for _, w := range writers {
			if got := h.ExtractLinks([]byte(w.link), PathlessRender); len(got) != 1 || got[0] != metaPathOf(w.want) {
				t.Errorf("%s %q: ExtractLinks(%q) = %q", w.name, p, w.link, got)
			}
			if strings.HasPrefix(w.link, "[[") {
				if got := ResolveWikiTarget(w.link[2 : len(w.link)-2]); got != w.want {
					t.Errorf("%s %q: ResolveWikiTarget(%q) = %q", w.name, p, w.link, got)
				}
			}
			// the file view pipeline: Parse resolves the links, Render runs goldmark
			parsed, _ := h.Parse([]byte(w.link), PathlessRender)
			out, err := h.Render(parsed, "", false)
			if got := renderedTarget(string(out)); err != nil || got != metaPathOf(w.want) {
				t.Errorf("%s %q: Render(%q) links to %q: %s", w.name, p, w.link, got, out)
			}
		}
	}
}

var (
	renderedHrefRe    = regexp.MustCompile(`<a href="([^"]*)"`)
	renderedPreviewRe = regexp.MustCompile(`/api/media/preview\?path=([^&"]*)`)
)

// renderedTarget returns the metadata path the first link or media preview in rendered html
// points at, as the browser would request it
func renderedTarget(out string) string {
	if m := renderedPreviewRe.FindStringSubmatch(out); m != nil {
		p, _ := url.QueryUnescape(m[1])
		return "media/" + p
	}
	m := renderedHrefRe.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	u, err := url.Parse(html.UnescapeString(m[1]))
	if err != nil {
		return ""
	}
	if rel, ok := strings.CutPrefix(u.Path, "/media/"); ok {
		return "media/" + rel
	}
	return pathutils.FileFromURL(u.String()).String()
}

// a ":" is only encoded before the first "/", where it would read as a scheme
func TestEncodeLinkPathColon(t *testing.T) {
	if got := encodeLinkPath("ns:a/b:c.md", LinkMarkdown); got != "ns%3Aa/b:c.md" {
		t.Errorf("encodeLinkPath = %q, want ns%%3Aa/b:c.md", got)
	}
}

// ParseLink splits a destination into its parts and Dest writes it back
func TestParseLinkDest(t *testing.T) {
	cases := []struct {
		dest string
		kind LinkKind
		want Link
		out  string
	}{
		{`<a b.md?x=1#sec> "t"`, LinkMarkdown, Link{Kind: LinkMarkdown, Path: "a b.md", Query: "?x=1", Anchor: "#sec", Title: ` "t"`, Angle: true}, `<a%20b.md?x=1#sec> "t"`},
		// spaces and quotes in a <...> anchor stay inside the brackets, a title only follows the ">"
		{`<a b.md#My "Heading"> =100x "t"`, LinkMarkdown, Link{Kind: LinkMarkdown, Path: "a b.md", Anchor: `#My "Heading"`, Title: ` "t"`, Angle: true}, `<a%20b.md#My "Heading"> "t"`},
		{`img.png =100x =200x "t"`, LinkMarkdown, Link{Kind: LinkMarkdown, Path: "img.png", Title: ` =200x "t"`}, `img.png =200x "t"`},
		{`it's "x".md`, LinkMarkdown, Link{Kind: LinkMarkdown, Path: "it's", Title: ` "x".md`}, `it's "x".md`},
		{" a%20b#sec|Text ", LinkWiki, Link{Kind: LinkWiki, Path: "a b", Anchor: "#sec", Alias: "|Text "}, "a b#sec|Text "},
		{"a&amp;b.png?x#y", LinkHTML, Link{Kind: LinkHTML, Path: "a&b.png", Query: "?x", Anchor: "#y"}, "a%26b.png?x#y"},
		{"mailto:a@b.c", LinkMarkdown, Link{Kind: LinkMarkdown, Path: "mailto:a@b.c", External: true}, "mailto:a@b.c"},
		// an unclosed "<" is part of the path
		{"<abc.md", LinkMarkdown, Link{Kind: LinkMarkdown, Path: "<abc.md"}, "%3Cabc.md"},
	}
	for _, c := range cases {
		if got := ParseLink(c.dest, c.kind); got != c.want || got.Dest() != c.out {
			t.Errorf("ParseLink(%q) = %#v, Dest %q, want %#v, %q", c.dest, got, got.Dest(), c.want, c.out)
		}
	}
}

// a new link escapes its text, so a "[" "]" in it can't end the link early
func TestLinkString(t *testing.T) {
	l := Link{Kind: LinkMarkdown, Text: "a/[1].md", Path: "a/[1].md"}
	if got, want := l.String(), `[a/\[1\].md](a/[1].md)`; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if got := RenderLinks(l.String(), PathlessRender); got != `[a/\[1\].md](`+fileURL("a/[1].md")+")" {
		t.Errorf("RenderLinks(%q) = %q", l.String(), got)
	}
}

// an encoded ":" is part of the path, not a scheme
func TestExtractLinksEncodedColon(t *testing.T) {
	if got := (&MarkdownHandler{}).ExtractLinks([]byte("[x](ns%3Apage.md) [y](mailto:a@b.c)"), PathlessRender); !slices.Equal(got, []string{"docs/ns:page.md"}) {
		t.Errorf("ExtractLinks = %q, want [ns:page.md]", got)
	}
}

// a callback returning the path it got leaves the link unchanged, also an equivalently encoded one
func TestRewriteLinksUnchangedEncoded(t *testing.T) {
	in := "[x](a%20b.md) [[a%25b]] [y](a%41.md)"
	got, changed := RewriteLinks(in, func(l Link) (string, bool) { return l.Path, true })
	if changed || got != in {
		t.Errorf("RewriteLinks = %q, %v, want %q unchanged", got, changed, in)
	}
}

// an unclosed "<" is no destination (CommonMark), so it isn't rewritten
func TestRewriteLinksUnclosedAngle(t *testing.T) {
	in := "[x](<abc.md) [y]: <abc.md"
	if got, changed := RewriteLinks(in, func(l Link) (string, bool) { return "new name.md", true }); changed || got != in {
		t.Errorf("RewriteLinks = %q, %v, want %q unchanged", got, changed, in)
	}
}

// a hand-written wikilink resolves to the same file for rendering and link metadata
func TestWikiLinkRenderMatchesExtract(t *testing.T) {
	for _, in := range []string{"page?x", "a b", "a%41", "dir/page.md#sec", "media/a"} {
		want := ResolveWikiTarget(in)
		if got := (&MarkdownHandler{}).ExtractLinks([]byte("[["+in+"]]"), PathlessRender); len(got) != 1 || got[0] != pathutils.ToWithPrefix(want) {
			t.Errorf("[[%s]]: ExtractLinks = %q, ResolveWikiTarget = %q", in, got, want)
		}
	}
}

// a "[id]: dest" after an inline code span is no reference definition
func TestRewriteLinksRefDefAfterCode(t *testing.T) {
	in := "`x` [id]: a.md"
	if got, changed := RewriteLinks(in, func(l Link) (string, bool) { return "b.md", true }); changed || got != in {
		t.Errorf("RewriteLinks = %q, %v, want %q unchanged", got, changed, in)
	}
}

// a "(...)" right after a [[wikilink]] is text, not a markdown link destination - for rendering
// and link metadata alike
func TestWikiLinkFollowedByParens(t *testing.T) {
	in := "see [[note]](draft) and [a [b]](c.md)"
	want := "see [note](" + fileURL("note.md") + ")(draft) and [a [b]](" + fileURL("c.md") + ")"
	if got := RenderLinks(in, PathlessRender); got != want {
		t.Errorf("RenderLinks = %q, want %q", got, want)
	}
	if got := (&MarkdownHandler{}).ExtractLinks([]byte(in), PathlessRender); !slices.Equal(got, []string{"docs/note.md", "docs/c.md"}) {
		t.Errorf("ExtractLinks = %q, want [note c.md]", got)
	}
}

// a wikilink to media opens the media detail page like a markdown link to it
func TestWikiLinkMediaDetail(t *testing.T) {
	if got, want := RenderLinks("[[media/a b.png]]", PathlessRender), RenderLinks("[a b](<media/a b.png>)", PathlessRender); got != want {
		t.Errorf("RenderLinks = %q, want %q", got, want)
	}
}

// a protocol-relative image is external, not a media preview path
func TestRenderImageProtocolRelative(t *testing.T) {
	out, err := NewMarkdownHandler().Render([]byte("![x](//cdn.example.com/x.png)"), "", false)
	if err != nil || !strings.Contains(string(out), `src="//cdn.example.com/x.png"`) {
		t.Errorf("Render = %q, %v, want the external src", out, err)
	}
}

// an inline code span over a line break is code on both lines, a blank line ends it
func TestRewriteLinksMultilineCodeSpan(t *testing.T) {
	in := "`x [a](a.md)\n[b](b.md) ü` [c](c.md)\n\n`[d](d.md)\n\n[e](e.md)`"
	want := "`x [a](a.md)\n[b](b.md) ü` [c](z.md)\n\n`[d](z.md)\n\n[e](z.md)`"
	if got, _ := RewriteLinks(in, func(l Link) (string, bool) { return "z.md", true }); got != want {
		t.Errorf("RewriteLinks = %q, want %q", got, want)
	}
	if got := RenderLinks(in, PathlessRender); !strings.Contains(got, "[b](b.md)") || strings.Contains(got, "[c](c.md)") {
		t.Errorf("RenderLinks = %q, want [b] kept and [c] rewritten", got)
	}
}

// a code span doesn't continue onto a new block - heading, list item, table row
func TestRewriteLinksCodeSpanEndsAtBlock(t *testing.T) {
	for _, in := range []string{"# t `x\n[a](a.md) `y`", "- type ``` to open\n- see [a](a.md)\n- close ```", "| a ` b |\n| [a](a.md) ` |"} {
		if got, _ := RewriteLinks(in, func(l Link) (string, bool) { return "z.md", true }); !strings.Contains(got, "[a](z.md)") {
			t.Errorf("RewriteLinks(%q) = %q, want [a] rewritten", in, got)
		}
	}
}

// bare "." and ".." are folder links, not notes named like the folder
func TestRenderDotFolderLinks(t *testing.T) {
	if got := RenderLinks("[up](..) [here](.)", "docs/a/x/n.md"); strings.Contains(got, ".md") {
		t.Errorf("RenderLinks = %q, want folder urls", got)
	}
	if got := RenderLinks("[root](..)", "docs/a/n.md"); got != "[root](/files/)" {
		t.Errorf("RenderLinks = %q, want the docs root url", got)
	}
}

// fileURL is the /files/ url of the docs-relative path rel
func fileURL(rel string) string { return pathutils.ToFileURL(pathutils.DocsPath(rel).String()) }

// a "|" in a link path written into a table row is escaped (GFM ends the cell at an unescaped one),
// in other lines it stays - and both read back as the same path
func TestRewriteLinksEscapesPipeInTableRows(t *testing.T) {
	content := "intro [w](old.md)\n\n| a | b |\n|---|:-:|\n| [x](old.md) | <a href=\"old.md\">h</a> |\n| [[old]] | [y](old.md) |\nlazy [z](old.md)\n\n[o](old.md)\n"
	got, changed := RewriteLinks(content, func(l Link) (string, bool) {
		if l.Kind == LinkWiki {
			return "", false
		}
		return "a|b.md", true
	})
	want := "intro [w](a|b.md)\n\n| a | b |\n|---|:-:|\n| [x](a\\|b.md) | <a href=\"a%7Cb.md\">h</a> |\n| [[old]] | [y](a\\|b.md) |\nlazy [z](a\\|b.md)\n\n[o](a|b.md)\n"
	if !changed || got != want {
		t.Errorf("RewriteLinks = %q, want %q", got, want)
	}
	for _, l := range NewMarkdownHandler().ExtractLinks([]byte(got), "docs/n.md") {
		if l != "docs/old.md" && l != "docs/a|b.md" {
			t.Errorf("ExtractLinks read %q", l)
		}
	}
}

// rewriting many links to a path with a "|" in a doc without a table finds the tables once, not
// once per link (20k reference definitions took 10s)
func TestRewriteLinksPipeNoTableIsLinear(t *testing.T) {
	content := strings.Repeat("[i]: a.md\n", 20000)
	start := time.Now()
	got, changed := RewriteLinks(content, func(l Link) (string, bool) { return "z|q.md", true })
	if !changed || !strings.Contains(got, "[i]: z|q.md\n") {
		t.Fatalf("RewriteLinks changed=%v", changed)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("rewriting 20k links took %s, want well under 3s", d)
	}
}

// YAML front matter is stripped before a doc renders (MarkdownHandler.Parse), so a link in it is
// no used link and rename / move leave it alone
func TestFrontMatterLinksAreNoLinks(t *testing.T) {
	content := "---\ntitle: [x](a.md)\nparent: [[b]]\nsrc: <img src=\"c.png\">\n---\n[y](a.md) [[b]]\n"
	if got := NewMarkdownHandler().ExtractLinks([]byte(content), "docs/n.md"); !slices.Equal(got, []string{"docs/a.md", "docs/b.md"}) {
		t.Errorf("ExtractLinks = %q, want the two body links", got)
	}
	got, changed := RewriteLinks(content, func(l Link) (string, bool) { return "new.md", true })
	want := "---\ntitle: [x](a.md)\nparent: [[b]]\nsrc: <img src=\"c.png\">\n---\n[y](new.md) [[new.md]]\n"
	if !changed || got != want {
		t.Errorf("RewriteLinks = %q, want %q", got, want)
	}
	// an unclosed "---" is no front matter
	if got := NewMarkdownHandler().ExtractLinks([]byte("---\n[y](a.md)\n"), "docs/n.md"); len(got) != 1 {
		t.Errorf("ExtractLinks without closing --- = %q", got)
	}
}

// CRLF line endings: links are read and rewritten like with LF, the line endings stay
func TestRewriteLinksCRLF(t *testing.T) {
	in := "# t\r\n\r\n[x](a.md)\r\n\r\n```\r\n[y](a.md)\r\n```\r\n\r\n[id]:\r\n  a.md\r\n\r\n    [z](a.md)\r\n"
	got, ok := RewriteLinks(in, func(l Link) (string, bool) { return "b.md", true })
	want := "# t\r\n\r\n[x](b.md)\r\n\r\n```\r\n[y](a.md)\r\n```\r\n\r\n[id]:\r\n  b.md\r\n\r\n    [z](a.md)\r\n"
	if !ok || got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTypedLinkPath(t *testing.T) {
	for _, c := range []struct {
		typed     string
		kind      LinkKind
		rel, anch string
		has       bool
	}{
		{"sub/a b", LinkWiki, "sub/a b", "", false},
		{"docs/media/a#intro", LinkWiki, "media/a", "intro", true},
		{"#intro", LinkWiki, "", "intro", true},
		{"/files/sub/a%20b.md#my-sec", LinkMarkdown, "sub/a b.md", "my-sec", true},
		{"/files/sub/", LinkMarkdown, "sub/", "", false},
		{"a%2", LinkMarkdown, "a%2", "", false},
	} {
		rel, anch, has := TypedLinkPath(c.typed, c.kind)
		if rel != c.rel || anch != c.anch || has != c.has {
			t.Errorf("%q: got (%q, %q, %v), want (%q, %q, %v)", c.typed, rel, anch, has, c.rel, c.anch, c.has)
		}
	}
}

// metaPathOf is the metadata path a special-char test case expects: a "media/..." one as it is, a
// docs name under docs/ - taken literally, so a "\" stays a file name char on every host.
func metaPathOf(want string) string {
	if strings.HasPrefix(want, "media/") {
		return want
	}
	return pathutils.DocsPath(want).String()
}
