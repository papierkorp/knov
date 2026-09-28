package parser

import (
	"strings"
	"testing"

	"knov/internal/pathutils"
)

func TestProcessMarkdownLinks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		// empty link text falls back to the filename (no anchor).
		{"fallback label plain path", "[](note.md)", "[note](" + pathutils.ToFileURL("note.md") + ")"},
		// empty link text falls back to "filename - Header Text".
		{"fallback label path plus anchor", "[](note.md#todo-vorlage)", "[note - Todo Vorlage](" + pathutils.ToFileURL("note.md") + "#todo-vorlage)"},
		// a same-page link: empty link text falls back to just the humanized header text, no
		// filename prefix.
		{"fallback label pure anchor", "[](#todo-vorlage)", "[Todo Vorlage](#todo-vorlage)"},
		// a percent-encoded path segment (a space in the folder name) is decoded before the
		// fallback label is built from it.
		{"percent-encoded path decoded before label", "[](mein%20ordner/notiz.md#eintrag-eins)", "[notiz - Eintrag Eins](" + pathutils.ToFileURL("mein ordner/notiz.md") + "#eintrag-eins)"},
		// a percent-encoded anchor segment is decoded before the fallback label is built, while
		// the href's anchor fragment itself is left exactly as written.
		{"percent-encoded anchor decoded before label", "[](notiz.md#einf%C3%BChrung-teil-1)", "[notiz - Einführung Teil 1](" + pathutils.ToFileURL("notiz.md") + "#einf%C3%BChrung-teil-1)"},
		// external (non-relative) links are left exactly as written - no fallback label, no
		// /files/ routing.
		{"external link untouched", "[Example](https://example.com/some/path)", "[Example](https://example.com/some/path)"},
		// a plain image link passes through unchanged.
		{"image embed untouched", "![Diagram](media/diagram.png)", "![Diagram](media/diagram.png)"},
		// a Windows-style backslash path is normalized to forward slashes - the only
		// transformation the image branch ever applies.
		{"image embed backslash path normalized", `![Diagram](sub\diagram.png)`, "![Diagram](sub/diagram.png)"},
		{"doc link backslash path normalized", `[](sub\note.md)`, "[note](" + pathutils.ToFileURL("sub/note.md") + ")"},
		// a markdown escape (\_) is no separator, and the anchor is left as written
		{"doc link markdown escape resolved", `[x](a\_b.md#c\d)`, "[x](" + pathutils.ToFileURL("a_b.md") + `#c\d)`},
		{"image embed markdown escape kept", `![D](a\_b.png)`, `![D](a\_b.png)`},
		// in a windows path every "\" is a separator, also before punctuation (_resources)
		{"doc link windows punctuation folder", `[x](sub\_resources\a.md)`, "[x](" + pathutils.ToFileURL("sub/_resources/a.md") + ")"},
		{"image embed title and anchor ignored", `![D](a\_b.png#c\d "t\x")`, `![D](a\_b.png#c\d "t\x")`},
		{"image embed windows punctuation folder", `![D](sub\_resources\a.png)`, "![D](sub/_resources/a.png)"},
		{"doc link backslash dot segments", `[x](a\..\b\.c.md)`, "[x](" + pathutils.ToFileURL("a/../b/.c.md") + ")"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ProcessMarkdownLinks(c.in); got != c.want {
				t.Errorf("ProcessMarkdownLinks(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// humanizeSlug capitalizes unicode runes (e.g. German umlauts) correctly via unicode.ToUpper
// rather than a naive ASCII-only uppercase, for both a single-word and a multi-word
// (hyphenated) slug.
func TestUnicodeHeaderSlugCapitalization(t *testing.T) {
	if got, want := ProcessMarkdownLinks("[](#übersicht)"), "[Übersicht](#übersicht)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ProcessMarkdownLinks("[](#persönliche-übersicht)"), "[Persönliche Übersicht](#persönliche-übersicht)"; got != want {
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
// "|alias", split off the "#anchor", default a missing extension to ".md", URL-decode the
// path, and yield an empty path for a pure "[[#header]]" anchor.
func TestWikiTargetExtraction(t *testing.T) {
	cases := []struct {
		in, wantPath, wantAnchor string
	}{
		{"note", "note.md", ""},
		{"note.md", "note.md", ""},
		{"sub/note.md#section", "sub/note.md", "#section"},
		{"note#section|Display Text", "note.md", "#section"},
		{"#header", "", "#header"},
		{"mein%20ordner/notiz", "mein ordner/notiz.md", ""},
		{`sub.d\note`, "sub.d/note.md", ""},
		{`note#a\b`, "note.md", `#a\b`},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			gotPath, gotAnchor := ResolveWikiTarget(c.in)
			if gotPath != c.wantPath || gotAnchor != c.wantAnchor {
				t.Errorf("ResolveWikiTarget(%q) = (%q, %q), want (%q, %q)", c.in, gotPath, gotAnchor, c.wantPath, c.wantAnchor)
			}
		})
	}
}

// ResolveWikiLinks keeps a pure "[[#header]]" as a real same-page link (empty label, filled in
// later by ProcessMarkdownLinks) rather than routing it through /files/ or collapsing it to ".".
func TestWikiLinkPureAnchor(t *testing.T) {
	if got, want := ResolveWikiLinks("[[#some-header]]"), "[](#some-header)"; got != want {
		t.Errorf("ResolveWikiLinks(%q) = %q, want %q", "[[#some-header]]", got, want)
	}
}

func TestExtractLinksDestination(t *testing.T) {
	in := `![x](</media/a%20b.png> "title") [y](note.md 'title') ![z](img/c.png) <img src="/media/d.png"> <a href='#top'> <a href="mailto:a@b.c"> <img src="data:image/png;base64,AAAA"> <img data-src="lazy.png"> <script src="//cdn.example.com/x.js"> <a href="/dashboard"> set src="prose.png" [[ns:page]] [[https://example.com]] ![w](C:\x.png) [e](a\_b.md) [f](sub\_res\a.md)
[ref]: <ref img.png> "title"
[note]: remember this
[^1]: footnote text`
	want := []string{"/media/a%20b.png", "note.md", "img/c.png", "C:/x.png", "a_b.md", "sub/_res/a.md", "ns:page", "/media/d.png", "ref img.png"}
	if got := NewMarkdownHandler().ExtractLinks([]byte(in)); strings.Join(got, ",") != strings.Join(want, ",") {
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
	RewriteLinks(`[a](sub\_res\x.md) [b](a\_b.md) [[sub\_res\y]]`, func(p string, _ LinkKind) (string, bool) {
		got = append(got, p)
		return "", false
	})
	// markdown links get their escapes or windows separators resolved, wiki links every "\" as "/"
	want := []string{"sub/_res/x.md", "a_b.md", "sub/_res/y"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("RewriteLinks paths = %q, want %q", got, want)
	}
}
