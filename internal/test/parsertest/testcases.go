package parsertest

import (
	"fmt"

	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/test"
)

func checkCase(name, input, expected string) test.CaseResult {
	actual := parser.ProcessMarkdownLinks(input)
	success := actual == expected
	cr := test.CaseResult{
		Name:     name,
		Expected: expected,
		Actual:   actual,
		Success:  success,
	}
	if !success {
		cr.Error = fmt.Sprintf("ProcessMarkdownLinks(%q) did not produce the expected output", input)
	}
	return cr
}

// caseFallbackLabelPlainPath covers "[](path.md)" - empty link text falls back to the
// filename (no anchor).
func caseFallbackLabelPlainPath() test.CaseResult {
	return checkCase("fallback-label-plain-path",
		"[](note.md)",
		"["+"note"+"]("+pathutils.ToFileURL("note.md")+")",
	)
}

// caseFallbackLabelPathPlusAnchor covers "[](path.md#anchor)" - empty link text falls back to
// "filename - Header Text".
func caseFallbackLabelPathPlusAnchor() test.CaseResult {
	return checkCase("fallback-label-path-plus-anchor",
		"[](note.md#todo-vorlage)",
		"[note - Todo Vorlage]("+pathutils.ToFileURL("note.md")+"#todo-vorlage)",
	)
}

// caseFallbackLabelPureAnchor covers "[](#anchor)" (a same-page link) - empty link text falls
// back to just the humanized header text, no filename prefix.
func caseFallbackLabelPureAnchor() test.CaseResult {
	return checkCase("fallback-label-pure-anchor",
		"[](#todo-vorlage)",
		"[Todo Vorlage](#todo-vorlage)",
	)
}

// casePercentEncodedPathDecodedBeforeLabel covers a percent-encoded path segment (a space in
// the folder name) being decoded before the fallback label is built from it - the label reads
// "notiz", not the raw "notiz" masked behind "%20".
func casePercentEncodedPathDecodedBeforeLabel() test.CaseResult {
	return checkCase("percent-encoded-path-decoded-before-label",
		"[](mein%20ordner/notiz.md#eintrag-eins)",
		"[notiz - Eintrag Eins]("+pathutils.ToFileURL("mein ordner/notiz.md")+"#eintrag-eins)",
	)
}

// casePercentEncodedAnchorDecodedBeforeLabel covers a percent-encoded anchor segment being
// decoded before the fallback label is built - the label humanizes the decoded unicode text,
// while the href's anchor fragment itself is left exactly as written (only the label needs the
// human-readable decoded form).
func casePercentEncodedAnchorDecodedBeforeLabel() test.CaseResult {
	return checkCase("percent-encoded-anchor-decoded-before-label",
		"[](notiz.md#einf%C3%BChrung-teil-1)",
		"[notiz - Einführung Teil 1]("+pathutils.ToFileURL("notiz.md")+"#einf%C3%BChrung-teil-1)",
	)
}

// caseUnicodeHeaderSlugCapitalization covers humanizeSlug capitalizing unicode runes (e.g.
// German umlauts) correctly via unicode.ToUpper rather than a naive ASCII-only uppercase, for
// both a single-word and a multi-word (hyphenated) slug.
func caseUnicodeHeaderSlugCapitalization() test.CaseResult {
	name := "unicode-header-slug-capitalization"

	singleWord := parser.ProcessMarkdownLinks("[](#übersicht)")
	multiWord := parser.ProcessMarkdownLinks("[](#persönliche-übersicht)")

	expectedSingle := "[Übersicht](#übersicht)"
	expectedMulti := "[Persönliche Übersicht](#persönliche-übersicht)"

	success := singleWord == expectedSingle && multiWord == expectedMulti
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%q and %q", expectedSingle, expectedMulti),
		Actual:   fmt.Sprintf("%q and %q", singleWord, multiWord),
		Success:  success,
	}
	if !success {
		cr.Error = "humanizeSlug did not capitalize the unicode header slug correctly"
	}
	return cr
}

// caseExternalLinkUntouched covers external (non-relative) links being left exactly as
// written - no fallback label, no /files/ routing.
func caseExternalLinkUntouched() test.CaseResult {
	input := "[Example](https://example.com/some/path)"
	return checkCase("external-link-untouched", input, input)
}

// caseImageEmbedUntouched covers image embeds being left untouched: a plain image link passes
// through unchanged, and a Windows-style backslash path is normalized to forward slashes (the
// only transformation the image branch ever applies).
func caseImageEmbedUntouched() test.CaseResult {
	name := "image-embed-untouched"

	plain := "![Diagram](media/diagram.png)"
	plainOut := parser.ProcessMarkdownLinks(plain)

	windowsIn := `![Diagram](sub\diagram.png)`
	windowsOut := parser.ProcessMarkdownLinks(windowsIn)
	windowsExpected := "![Diagram](sub/diagram.png)"

	success := plainOut == plain && windowsOut == windowsExpected
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%q unchanged, %q normalized to %q", plain, windowsIn, windowsExpected),
		Actual:   fmt.Sprintf("plainOut=%q windowsOut=%q", plainOut, windowsOut),
		Success:  success,
	}
	if !success {
		cr.Error = "image embed handling did not behave as expected"
	}
	return cr
}
