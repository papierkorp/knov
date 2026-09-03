// Package book models a `.book` file: an ordered list of file/section references, titles and
// separators that compose into one markdown document (exportable as markdown or pdf).
//
// A `.book` is a plain markdown file, so git, backup, search and the link graph handle it for
// free. Over an index/MOC file it adds only per-section extraction (IncludeSubheaders) and
// export of the composed document, so the editor is the index entry editor plus a
// "subheaders" toggle and the Entry model is shared.
package book

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

// entry type discriminators, as stored in a `.book` / `.index` file
const (
	EntryFile      = "file"
	EntryTitle     = "title"
	EntrySeparator = "separator"
	// EntryUnknown is a non-blank line Parse did not recognize (a hand edit, or a syntax
	// this parser predates). It is kept verbatim in Value so an editor round-trip
	// (Parse -> edit -> ToMarkdown) never silently drops it; Compose ignores it.
	EntryUnknown = "unknown"
)

// Entry is one line of a `.book`/`.index` file, in composition order: a file/section
// reference (EntryFile, Value "path" or "path#anchor"), a "# text" heading (EntryTitle) or a
// "---" rule (EntrySeparator). IncludeSubheaders round-trips as a trailing
// "<!-- subheaders -->" comment and only affects a section reference.
type Entry struct {
	Type              string `json:"type"`
	Value             string `json:"value"`
	IncludeSubheaders bool   `json:"subheaders,omitempty"`
}

// Parse reads `.book`/`.index` markdown into entries: a bullet line with a "[[path]]"
// wikilink is a file entry ("[[path#anchor]]" a section entry, a trailing
// "<!-- subheaders -->" sets IncludeSubheaders), a "#"-prefixed line a title, "---" a
// separator. Any other non-blank line becomes an EntryUnknown that round-trips verbatim.
// The [[ ]] body is taken verbatim (alias syntax not split out here). Parse assumes the
// shape ToMarkdown writes, not arbitrary markdown; "#" lines inside closed ``` / ~~~
// fences are skipped, an unterminated fence is ignored.
func Parse(content string) []Entry {
	var entries []Entry
	lines := strings.Split(content, "\n")
	inFence := fencedLines(lines)

	for i, line := range lines {
		if inFence[i] {
			continue
		}
		line = strings.TrimSpace(line)

		switch {
		case line == "---":
			entries = append(entries, Entry{Type: EntrySeparator})

		case strings.HasPrefix(line, "#"):
			if text := strings.TrimSpace(strings.TrimLeft(line, "#")); text != "" {
				entries = append(entries, Entry{Type: EntryTitle, Value: text})
			}

		default:
			if line == "" {
				continue
			}
			if rest := strings.TrimLeft(line, "-*+ \t"); strings.HasPrefix(rest, "[[") {
				if end := strings.Index(rest[2:], "]]"); end >= 0 {
					entries = append(entries, Entry{
						Type:              EntryFile,
						Value:             rest[2 : 2+end],
						IncludeSubheaders: strings.Contains(rest[2+end+2:], "<!-- subheaders -->"),
					})
					continue
				}
			}
			// keep any other non-blank line verbatim rather than dropping it on save
			entries = append(entries, Entry{Type: EntryUnknown, Value: line})
		}
	}

	return entries
}

// fencedLines marks line indices inside a ``` or ~~~ fenced block (fence lines included): a
// marker opens a block that a later line starting with the same three chars closes. An
// unterminated fence marks nothing, so its lines stay eligible as entries.
func fencedLines(lines []string) []bool {
	inside := make([]bool, len(lines))
	open := -1 // opening-line index of the currently open fence, -1 when none is open
	var marker string
	for i, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case open >= 0:
			inside[i] = true
			if strings.HasPrefix(t, marker) {
				open = -1 // closed; its lines stay marked
			}
		case strings.HasPrefix(t, "```"), strings.HasPrefix(t, "~~~"):
			open, marker = i, t[:3]
			inside[i] = true
		}
	}
	// unterminated fence: roll its lines back so they stay eligible as entries
	for i := open; i >= 0 && i < len(lines); i++ {
		inside[i] = false
	}
	return inside
}

// ToMarkdown serializes entries back to `.book`/`.index` markdown (inverse of Parse). File
// entries are written as plain "- [[path]]" wikilinks so link detection picks them up;
// alias syntax is never produced.
func ToMarkdown(entries []Entry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case EntrySeparator:
			sb.WriteString("\n---\n\n")
		case EntryTitle:
			if e.Value != "" {
				fmt.Fprintf(&sb, "\n# %s\n\n", e.Value)
			}
		case EntryFile:
			if e.Value == "" {
				continue
			}
			// round-trip IncludeSubheaders so the editor checkbox survives a reload
			if e.IncludeSubheaders {
				fmt.Fprintf(&sb, "- [[%s]] <!-- subheaders -->\n", e.Value)
			} else {
				fmt.Fprintf(&sb, "- [[%s]]\n", e.Value)
			}
		case EntryUnknown:
			if e.Value != "" {
				sb.WriteString(e.Value + "\n")
			}
		}
	}
	return sb.String()
}

// Read loads and parses the `.book` file at bookPath (a docs-relative path).
func Read(bookPath string) ([]Entry, error) {
	raw, err := contentStorage.ReadFile(pathutils.ToDocsPath(bookPath))
	if err != nil {
		return nil, err
	}
	return Parse(string(raw)), nil
}

// Compose reads the `.book` at bookPath and composes it (see ComposeEntries). Only a
// `.book` that can't be read at all returns an error. The export endpoints use this; the
// file view parses once and calls ComposeEntries directly.
func Compose(bookPath string) (string, error) {
	entries, err := Read(bookPath)
	if err != nil {
		return "", err
	}
	return ComposeEntries(bookPath, entries), nil
}

// ComposeEntries concatenates entries in order into one markdown document: title -> "# "
// heading, separator -> "---", whole-file entry -> the file's content, section entry ->
// that heading's section (optionally with subheaders); EntryUnknown is skipped. Entry
// values resolve like wikilinks (parser.ResolveWikiTarget). A binary target, an unreadable
// entry or one outside the docs root becomes a visible "> ⚠️ could not include ..." marker
// rather than an error, so the view and exports never 500. bookPath is only for log context.
func ComposeEntries(bookPath string, entries []Entry) string {
	handler := contentHandler.GetHandler("markdown")
	// no read cache: many sections from one file re-read it per entry. books are small
	// and this path is cold; revisit if that changes.
	readFile := func(p string) ([]byte, error) {
		return contentStorage.ReadFile(pathutils.ToDocsPath(p))
	}

	var parts []string

	for _, e := range entries {
		switch e.Type {
		case EntryTitle:
			if e.Value != "" {
				parts = append(parts, "# "+e.Value)
			}
			continue
		case EntrySeparator:
			parts = append(parts, "---")
			continue
		case EntryUnknown:
			continue // kept in the `.book` file, omitted from the composed document
		}

		path, anchor := parser.ResolveWikiTarget(e.Value)
		// slugify the anchor to the id the renderer/TOC generate, so an entry can use the
		// visible heading text ("notes.md#My Section"), not just a pre-slugified id.
		// GenerateID is idempotent, so dedup ids ("#my-section-1") work too; a bare-text
		// anchor matching several headings resolves to the first.
		section := strings.TrimPrefix(anchor, "#")
		if decoded, decErr := url.PathUnescape(section); decErr == nil {
			section = decoded
		}
		if section != "" {
			section = utils.GenerateID(section, map[string]int{})
		}

		var (
			content  string
			entryErr error
		)
		switch {
		case section != "":
			var raw []byte
			if raw, entryErr = readFile(path); entryErr == nil {
				content, entryErr = handler.ExtractSectionFromString(string(raw), section, e.IncludeSubheaders)
			}
		case !isInlineableWholeFile(path):
			entryErr = fmt.Errorf("not a text file, refusing to inline whole")
		default:
			var raw []byte
			raw, entryErr = readFile(path)
			content = string(raw)
		}

		if entryErr != nil {
			logging.LogWarning(logging.KeyApp, "book: skipping entry %q in %s: %v", e.Value, bookPath, entryErr)
			// visible marker (plain backticks, not [[ ]], so the wikilink rewriter leaves
			// the path readable) so the gap shows in the view and exports, not just logs.
			// backticks in the value are dropped so it can't break out of the code span.
			parts = append(parts, "> ⚠️ could not include `"+strings.ReplaceAll(e.Value, "`", "'")+"`")
			continue
		}
		parts = append(parts, strings.TrimSpace(content))
	}

	// blank line between pieces so a "---" stays a rule, not a setext underline
	return strings.Join(parts, "\n\n")
}

// isInlineableWholeFile reports whether a whole-file entry is safe to inline verbatim: a
// text/markdown file, not a binary asset that would dump as garbage.
func isInlineableWholeFile(path string) bool {
	return parser.IsMarkdownExtension(path) || strings.EqualFold(filepath.Ext(path), ".txt")
}
