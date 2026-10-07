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
	"path/filepath"
	"strings"

	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/markdown"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// entry type discriminators, as stored in a `.book` / `.index` file
const (
	EntryFile      = "file"
	EntryTitle     = "title"
	EntrySeparator = "separator"
	// EntryUnknown is a run of consecutive lines Parse did not model (a hand edit, a pasted
	// table, a fenced block, or a syntax this parser predates), coalesced into one block and
	// kept verbatim in Value - interior blank lines and indentation intact. The round-trip
	// (Parse -> edit -> ToMarkdown) never drops it; Compose emits it verbatim into the document.
	EntryUnknown = "unknown"
	// EntryFilter references a saved filter by id (Value); its matching files are resolved
	// at compose time, so the book follows the filter instead of a frozen file list
	EntryFilter = "filter"
)

// FilterResolver returns the docs-relative paths matching a saved filter id. Set by the
// filter package; book can't import it (filter -> files -> book).
var FilterResolver func(filterID string) ([]string, error)

// Entry is one line of a `.book`/`.index` file, in composition order: a file/section
// reference (EntryFile, Value the [[...]] body "path" or "path#anchor", see DecodeFileRef), a "# text" heading (EntryTitle,
// Level 1-6 for the "#" depth) or a "---" rule (EntrySeparator). IncludeSubheaders
// round-trips as a trailing "<!-- subheaders -->" comment and only affects a section
// reference.
type Entry struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	// Level is the "#" depth of a title (1-6); 0 on a non-title entry. Kept in range by
	// ClampLevel - see there for where each guard runs.
	Level             int  `json:"level,omitempty"`
	IncludeSubheaders bool `json:"subheaders,omitempty"`
}

// Parse reads `.book`/`.index` markdown into entries: a list item whose body is a "[[path]]"
// wikilink is a file entry ("[[path#anchor]]" makes it a section entry, a trailing
// "<!-- subheaders -->" sets IncludeSubheaders), an ATX heading ("## text", 1-6 "#" then a
// space) a title with its "#" count kept as Level, "---" a separator - each recognized only
// flush-left. Any run of other lines (prose, an indented line, a "#" run that is not a
// heading, 7+ "#") is coalesced into one EntryUnknown that round-trips verbatim.
// The [[ ]] body is taken verbatim (alias syntax not split out here). Parse assumes the
// shape ToMarkdown writes, not arbitrary markdown; lines inside ``` / ~~~ fences are kept
// verbatim inside the surrounding unknown block, never parsed as entries (an unterminated
// fence runs to end of file - see markdown.FenceMask). A "<!-- filter: id -->" line is a
// filter entry.
func Parse(content string) []Entry {
	var entries []Entry
	content = strings.ReplaceAll(content, "\r\n", "\n") // a Windows hand edit / textarea POST must not leave stray \r in a verbatim block
	lines := strings.Split(content, "\n")
	inFence := markdown.FenceMask(lines)

	// consecutive lines Parse doesn't model (prose, a pasted table, a fenced block) build up
	// one raw block, flushed as a single EntryUnknown when a modeled entry or end of file
	// ends the run; interior blank lines are kept, surrounding ones trimmed off
	var block []string
	flush := func() {
		if raw := strings.Trim(strings.Join(block, "\n"), "\n"); raw != "" {
			entries = append(entries, Entry{Type: EntryUnknown, Value: raw})
		}
		block = block[:0]
	}

	for i, line := range lines {
		if inFence[i] {
			block = append(block, line) // verbatim, never parsed as a modeled entry
			continue
		}
		trimmed := strings.TrimSpace(line)
		// a hand-written line with leading indentation is nested structure (a sub-list, an
		// indented code block); keep it verbatim instead of promoting + flattening it to a
		// flush-left title/reference/separator
		flushLeft := line == strings.TrimLeft(line, " \t")

		switch {
		case flushLeft && trimmed == "---":
			flush()
			entries = append(entries, Entry{Type: EntrySeparator})

		case trimmed == "":
			block = append(block, "")

		default:
			// a real ATX heading ("## text") keeps its "#" depth as Level; a bare "#" run
			// with no text, or 7+ "#", is not a heading and stays verbatim in the block
			if flushLeft {
				if level, text, ok := markdown.ATXHeading(trimmed); ok && text != "" {
					flush()
					entries = append(entries, Entry{Type: EntryTitle, Value: text, Level: level})
					continue
				}
				if id, ok := strings.CutPrefix(trimmed, "<!-- filter:"); ok && strings.HasSuffix(id, "-->") {
					if id = strings.TrimSpace(strings.TrimSuffix(id, "-->")); id != "" {
						flush()
						entries = append(entries, Entry{Type: EntryFilter, Value: id})
						continue
					}
				}
				// a "[[...]]" wikilink (with or without a bullet) may be a file/section reference
				if ref, sub, ok := parseFileRef(strings.TrimLeft(trimmed, "-*+ \t")); ok {
					flush()
					entries = append(entries, Entry{Type: EntryFile, Value: ref, IncludeSubheaders: sub})
					continue
				}
			}
			// keep any other line verbatim (indentation and all) rather than dropping it on save
			block = append(block, line)
		}
	}
	flush()

	return entries
}

// parseFileRef reads a "[[path]]" / "[[path#anchor]]" wikilink from one list-item body (the
// bullet already stripped); a trailing "<!-- subheaders -->" sets sub. Anything else is not
// a reference, so ok is false and the caller keeps the line verbatim as an EntryUnknown.
func parseFileRef(body string) (ref string, sub bool, ok bool) {
	if !strings.HasPrefix(body, "[[") {
		return "", false, false
	}
	end := strings.Index(body, "]]")
	if end < 0 {
		return "", false, false
	}
	return body[2:end], strings.Contains(body[end+2:], "<!-- subheaders -->"), true
}

// DecodeFileRef turns a file entry's Value (the [[...]] body as written) into the plain path
// and section ("" for a whole file) the editor shows apart, so any file name works;
// EncodeFileRef is the inverse for a picked or typed pair. The typed path is trimmed, a typed
// section is stored as its heading id (see parser.AnchorID) - a "|alias" isn't kept.
func DecodeFileRef(v string) (path, section string) {
	l := parser.ParseLink(v, parser.LinkWiki)
	return l.Path, l.AnchorText()
}

// EncodeFileRef writes a plain path and section as a file entry Value - see DecodeFileRef.
func EncodeFileRef(path, section string) string {
	l := parser.Link{Kind: parser.LinkWiki, Path: strings.TrimSpace(path)}
	if id := parser.AnchorID(strings.TrimSpace(section)); id != "" {
		l.Anchor = "#" + id
	}
	return l.Dest()
}

// ToMarkdown serializes entries back to `.book`/`.index` markdown (inverse of Parse). File
// entries are written as plain "- [[path]]" wikilinks so link detection picks them up;
// alias syntax is never produced. A title is written with its Level worth of "#". An
// EntryUnknown block is written verbatim; an empty one (only reachable via a hand-crafted
// POST) is dropped so a round-trip stays stable.
func ToMarkdown(entries []Entry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case EntrySeparator:
			sb.WriteString("\n---\n\n")
		case EntryTitle:
			if e.Value != "" {
				fmt.Fprintf(&sb, "\n%s %s\n\n", titleHashes(e.Level), e.Value)
			}
		case EntryFile:
			if e.Value != "" {
				// round-trip IncludeSubheaders so the editor checkbox survives a reload
				if e.IncludeSubheaders {
					fmt.Fprintf(&sb, "- [[%s]] <!-- subheaders -->\n", e.Value)
				} else {
					fmt.Fprintf(&sb, "- [[%s]]\n", e.Value)
				}
			}
		case EntryFilter:
			// a newline or "-->" (only via a crafted POST) would break the comment line
			if e.Value != "" && !strings.ContainsAny(e.Value, "\r\n") && !strings.Contains(e.Value, "-->") {
				fmt.Fprintf(&sb, "<!-- filter: %s -->\n", e.Value)
			}
		case EntryUnknown:
			if e.Value != "" {
				sb.WriteString(e.Value + "\n")
			}
		}
	}
	return sb.String()
}

// ClampLevel maps a title "#" depth to the valid [1,6] range, treating a missing (0),
// negative or out-of-range value as broken input and falling back to 1. The save handler
// runs the form value through this; titleHashes applies it again as a last guard. (Parse
// needs no clamp - markdown.ATXHeading only reports a level for a real 1-6 "#" heading.)
func ClampLevel(level int) int {
	if level < 1 || level > 6 {
		return 1
	}
	return level
}

// titleHashes returns the "#"..."######" prefix for a title of the given level.
func titleHashes(level int) string {
	return strings.Repeat("#", ClampLevel(level))
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
// heading (Level "#"s), separator -> "---", whole-file entry -> the file's content, section
// entry -> that heading's section (optionally with subheaders); an EntryUnknown block is
// emitted verbatim (hand-written prose/markdown, e.g. a table or list). Entry values resolve
// like wikilinks (parser.ResolveWikiTarget). A binary target, an unreadable entry or one
// outside the docs root becomes a visible "> ⚠️ could not include ..." marker rather than
// an error, so the view and exports never 500. bookPath is only for log context.
func ComposeEntries(bookPath string, entries []Entry) string {
	handler := contentHandler.GetHandler("markdown")
	// no read cache: many sections from one file re-read it per entry. books are small
	// and this path is cold; revisit if that changes.
	readFile := func(p string) ([]byte, error) {
		return contentStorage.ReadFile(pathutils.ToDocsPath(p))
	}

	var parts []string

	for _, e := range expandFilters(bookPath, entries) {
		switch e.Type {
		case EntryUnknown:
			// one verbatim block (hand-written prose/markdown: a table, a list, a note)
			if e.Value != "" {
				parts = append(parts, e.Value)
			}
			continue
		case EntryTitle:
			if e.Value != "" {
				parts = append(parts, titleHashes(e.Level)+" "+e.Value)
			}
			continue
		case EntrySeparator:
			parts = append(parts, "---")
			continue
		}

		path := parser.ResolveWikiTarget(e.Value)
		// the visible heading text ("notes.md#My Section") works as anchor too; a bare-text
		// anchor matching several headings resolves to the first.
		section := parser.AnchorID(parser.ParseLink(e.Value, parser.LinkWiki).AnchorText())

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

// expandFilters replaces each filter entry with the verbatim content of each matching file
// (read here, not via an EntryFile, so a "#" in a filename isn't taken for an anchor); an
// unresolvable filter or unreadable match becomes the same visible "could not include"
// marker as a bad file. binary matches are skipped, not warned about; a filter with nothing
// to inline gets a visible note so the book doesn't look broken.
func expandFilters(bookPath string, entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Type != EntryFilter {
			out = append(out, e)
			continue
		}
		var paths []string
		err := fmt.Errorf("no filter resolver registered")
		if FilterResolver != nil {
			paths, err = FilterResolver(e.Value)
		}
		if err != nil {
			logging.LogWarning(logging.KeyApp, "book: skipping filter %q in %s: %v", e.Value, bookPath, err)
			out = append(out, Entry{Type: EntryUnknown, Value: "> ⚠️ could not include filter `" + strings.ReplaceAll(e.Value, "`", "'") + "`"})
			continue
		}
		before := len(out)
		for _, p := range paths {
			if !isInlineableWholeFile(p) {
				continue
			}
			raw, err := contentStorage.ReadFile(pathutils.ToDocsPath(p))
			if err != nil {
				logging.LogWarning(logging.KeyApp, "book: skipping filter match %q in %s: %v", p, bookPath, err)
				out = append(out, Entry{Type: EntryUnknown, Value: "> ⚠️ could not include `" + strings.ReplaceAll(p, "`", "'") + "`"})
				continue
			}
			out = append(out, Entry{Type: EntryUnknown, Value: strings.TrimSpace(string(raw))})
		}
		if len(out) == before {
			out = append(out, Entry{Type: EntryUnknown, Value: "> no files match filter `" + strings.ReplaceAll(e.Value, "`", "'") + "`"})
		}
	}
	return out
}

// isInlineableWholeFile reports whether a whole-file entry is safe to inline verbatim: a
// text/markdown file, not a binary asset that would dump as garbage.
func isInlineableWholeFile(path string) bool {
	return parser.IsMarkdownExtension(path) || strings.EqualFold(filepath.Ext(path), ".txt")
}
