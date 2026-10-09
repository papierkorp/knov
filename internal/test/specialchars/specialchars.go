// Package specialchars - the shared list of file names with characters that are special in
// link text, urls or markdown. Every link writer and reader is tested against it (go tests and
// the linkstest suite), so it has no imports and can be used from any package's tests.
package specialchars

import "strings"

// Names are docs-relative file names, each one hard for a different reason: percent signs that
// do or don't form an escape, link syntax (# ? | [ ] ( ) < > " '), an html entity, a scheme-like
// ":", a dot in an extensionless note name, non-ascii, a "\" (a separator on windows), leading /
// trailing spaces and a special-char folder.
var Names = []string{
	"100%.md",
	"a%41.md",
	"a#b.md",
	"a?b.md",
	"a|b.md",
	"[1].md",
	"x (1).md",
	"a&copy;b.md",
	`it's "x".md`,
	"a<b>.md",
	"ns:page.md",
	"v1.2 notes.md",
	"ö ü.md",
	`a\b.md`,
	" lead.md",
	"trail.md ",
	"x (1)/ö ü.md",
}

// windowsInvalid are the characters a windows filename can't contain.
const windowsInvalid = `\:*?"<>|`

// ValidOn reports whether name can be a real file on goos ("windows" or anything else), for
// suites that create the files - the codec still has to handle all of them (git sync, copies).
func ValidOn(goos, name string) bool {
	if goos != "windows" {
		return true
	}
	for _, seg := range strings.Split(name, "/") {
		if strings.ContainsAny(seg, windowsInvalid) || strings.HasSuffix(seg, " ") || strings.HasSuffix(seg, ".") {
			return false
		}
	}
	return true
}

// SplitsOn reports whether name has a "\" that goos reads as a path separator ("windows"), so
// the name is two path segments there and not a file name - a test of host paths skip it.
func SplitsOn(goos, name string) bool {
	return goos == "windows" && strings.Contains(name, `\`)
}
