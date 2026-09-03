package parsertest

import (
	"fmt"

	"knov/internal/parser"
	"knov/internal/test"
)

// caseWikiTargetExtraction covers parser.ResolveWikiTarget - the wikilink-body normalizer
// now shared with internal/book: drop the "|alias", split off the "#anchor", default a
// missing extension to ".md", URL-decode the path, and yield an empty path for a pure
// "[[#header]]" anchor.
func caseWikiTargetExtraction() test.CaseResult {
	name := "wiki-target-extraction"

	type tc struct{ in, wantPath, wantAnchor string }
	tcs := []tc{
		{"note", "note.md", ""},
		{"note.md", "note.md", ""},
		{"sub/note.md#section", "sub/note.md", "#section"},
		{"note#section|Display Text", "note.md", "#section"},
		{"#header", "", "#header"},
		{"mein%20ordner/notiz", "mein ordner/notiz.md", ""},
	}

	var failures []string
	for _, c := range tcs {
		gotPath, gotAnchor := parser.ResolveWikiTarget(c.in)
		if gotPath != c.wantPath || gotAnchor != c.wantAnchor {
			failures = append(failures, fmt.Sprintf("ResolveWikiTarget(%q) = (%q, %q), want (%q, %q)",
				c.in, gotPath, gotAnchor, c.wantPath, c.wantAnchor))
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: "every wikilink body normalizes to the documented (path, anchor) pair",
		Actual:   fmt.Sprintf("%d/%d ok", len(tcs)-len(failures), len(tcs)),
		Success:  len(failures) == 0,
	}
	if len(failures) != 0 {
		cr.Error = fmt.Sprintf("%v", failures)
	}
	return cr
}

// caseWikiLinkPureAnchor covers ResolveWikiLinks keeping a pure "[[#header]]" as a real
// same-page link (empty label, filled in later by ProcessMarkdownLinks) rather than routing
// it through /files/ or collapsing it to ".".
func caseWikiLinkPureAnchor() test.CaseResult {
	name := "wiki-link-pure-anchor"
	got := parser.ResolveWikiLinks("[[#some-header]]")
	want := "[](#some-header)"
	cr := test.CaseResult{
		Name:     name,
		Expected: want,
		Actual:   got,
		Success:  got == want,
	}
	if !cr.Success {
		cr.Error = "ResolveWikiLinks did not keep the pure anchor as a same-page link"
	}
	return cr
}
