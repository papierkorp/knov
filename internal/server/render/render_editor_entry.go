// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	htmlpkg "html"
	"slices"
	"strings"
	"sync/atomic"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/filter"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// entryRowCounter gives htmx-added entry rows a unique DOM id; their array index is
// always the placeholder 999 and would otherwise collide.
var entryRowCounter atomic.Uint64

// ----------------------------------------------------------------------------
// -------------------------- Entry Editor (index / book) ----------------------
// ----------------------------------------------------------------------------
//
// One editor drives `.index`/`.moc` and `.book` files, as the list editor drives `.list`
// and `.todo`. bookMode adds the "include subheaders" toggle and tags the file as a book;
// the entry model, row markup and move/remove script are shared.

// RenderIndexEditor renders the `.index`/`.moc` entry editor as an htmx form.
func RenderIndexEditor(filePath string) (string, error) { return renderEntryEditor(filePath, false) }

// RenderBookEditor renders the `.book` entry editor: renderEntryEditor in bookMode.
func RenderBookEditor(filePath string) (string, error) { return renderEntryEditor(filePath, true) }

// renderEntryEditor is the shared index/book entry editor; bookMode picks the save route,
// the "include subheaders" toggle and the labels.
func renderEntryEditor(filePath string, bookMode bool) (string, error) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	// .book-mode is a styling hook for custom.css; the builtin theme doesn't rely on it
	modeClass := ""
	if bookMode {
		modeClass = " book-mode"
	}

	var html strings.Builder
	fmt.Fprintf(&html, `<div class="entry-editor%s" id="entry-editor">`, modeClass)

	// load existing entries if editing
	var entries []book.Entry
	if filePath != "" {
		if content, err := contentStorage.ReadFile(pathutils.ToDocsPath(filePath)); err == nil && len(content) > 0 {
			entries = book.Parse(string(content))
		}
	}
	heading := t("index configuration")
	saveLabel := t("save index")
	action := "/api/editor/indexeditor"
	if bookMode {
		heading = t("book configuration")
		saveLabel = t("save book")
		action = "/api/editor/bookeditor"
	}

	html.WriteString(`<div class="entry-form-container">`)
	fmt.Fprintf(&html, `<h4>%s</h4>`, heading)

	isEdit := filePath != ""
	cancelURL := "/"
	if isEdit {
		cancelURL = pathutils.ToFileURL(filePath)
	}

	fmt.Fprintf(&html, `<form hx-post="%s" hx-target="#entry-editor-status" hx-swap="innerHTML" id="entry-form">`, action)

	if !isEdit {
		html.WriteString(`<div class="form-group">`)
		fmt.Fprintf(&html, `<label>%s</label>`, t("file path"))
		html.WriteString(GenerateDatalistInput("filepath-input", "filepath", "", t("path/to/file"), "/api/files/folder-suggestions", true))
		html.WriteString(`</div>`)
	} else {
		fmt.Fprintf(&html, `<input type="hidden" name="filepath" value="%s"/>`, htmlpkg.EscapeString(filePath))
	}

	// entries container
	html.WriteString(`<div id="entries-container" class="entries-container">`)
	for i, entry := range entries {
		html.WriteString(renderEntryRow(i, entry, bookMode))
	}
	html.WriteString(`</div>`)

	// add entry buttons
	addVals := func(entryType string) string {
		if bookMode {
			return fmt.Sprintf(`{"type":"%s","mode":"book"}`, entryType)
		}
		return fmt.Sprintf(`{"type":"%s"}`, entryType)
	}
	html.WriteString(`<div class="form-actions">`)
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/entry/add-entry" hx-vals='%s' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, addVals("separator"), t("add separator"))
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/entry/add-entry" hx-vals='%s' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, addVals("file"), t("add file"))
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/entry/add-entry" hx-vals='%s' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, addVals("title"), t("add title"))
	if bookMode {
		fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/entry/add-entry" hx-vals='%s' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, addVals("filter"), t("add filter"))
	}
	html.WriteString(`</div>`)

	// save + cancel buttons
	html.WriteString(`<div class="form-actions">`)
	fmt.Fprintf(&html, `<button type="submit" class="btn-primary">%s</button>`, saveLabel)
	// single-quoted attribute wrapping a JSON string literal, both layers escaped, so a
	// filepath containing a quote/apostrophe can't break out of the attribute or the JS
	fmt.Fprintf(&html, `<button type="button" onclick='location.href=%s' class="btn-secondary">%s</button>`, htmlpkg.EscapeString(jsEscapeString(cancelURL)), t("cancel"))
	html.WriteString(`<div id="entry-editor-status"></div>`)
	html.WriteString(`</div>`)
	html.WriteString(`</form>`)

	// move/remove/reindex script - use window scope for HTMX compatibility
	html.WriteString(renderEntryEditorScript())

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String(), nil
}

// renderEntryEditorScript returns the shared move/remove/reindex `<script>` block. It
// hard-codes the `entries-container` id and `entries[i][...]` names renderEntryRow emits.
func renderEntryEditorScript() string {
	return `<script>
window.moveEntry = function(index, direction) {
	const container = document.getElementById('entries-container');
	const rows = Array.from(container.querySelectorAll('.entry-row'));
	const row = rows[index];

	if (!row) return;

	const targetIndex = index + direction;
	if (targetIndex < 0 || targetIndex >= rows.length) return;

	if (direction === -1 && index > 0) {
		container.insertBefore(row, rows[index - 1]);
	} else if (direction === 1 && index < rows.length - 1) {
		container.insertBefore(rows[index + 1], row);
	}

	window.reindexEntries();
};

window.removeEntry = function(button) {
	button.closest('.entry-row').remove();
	window.reindexEntries();
};

window.reindexEntries = function() {
	const container = document.getElementById('entries-container');
	if (!container) {
		return;
	}
	const allRows = container.querySelectorAll('.entry-row');
	allRows.forEach((r, i) => {
		r.setAttribute('data-entry-index', i);
		r.querySelectorAll('[name^="entries["]').forEach(input => {
			const name = input.getAttribute('name');
			// Replace entries[any_number] with entries[i]
			const newName = name.replace(/entries\[\d+\]/, 'entries[' + i + ']');
			input.setAttribute('name', newName);
		});
		const upBtn = r.querySelector('.btn-move:first-of-type');
		const downBtn = r.querySelector('.btn-move:nth-of-type(2)');
		if (upBtn) upBtn.setAttribute('onclick', 'moveEntry(' + i + ', -1)');
		if (downBtn) downBtn.setAttribute('onclick', 'moveEntry(' + i + ', 1)');
	});
};
</script>`
}

// renderEntryRow renders one index/book entry row. In bookMode a file row also gets an
// "include subheaders" checkbox and a "#section" placeholder hint.
func renderEntryRow(index int, entry book.Entry, bookMode bool) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var html strings.Builder
	fmt.Fprintf(&html, `<div class="entry-row" data-entry-index="%d">`, index)

	// controls on the left
	html.WriteString(`<div class="entry-controls">`)
	fmt.Fprintf(&html, `<button type="button" onclick="moveEntry(%d, -1)" class="btn-move"><i class="fa-solid fa-arrow-up"></i></button>`, index)
	fmt.Fprintf(&html, `<button type="button" onclick="moveEntry(%d, 1)" class="btn-move"><i class="fa-solid fa-arrow-down"></i></button>`, index)
	html.WriteString(`<button type="button" onclick="removeEntry(this)" class="btn-remove"><i class="fa-solid fa-xmark"></i></button>`)
	html.WriteString(`</div>`)

	// content on the right
	html.WriteString(`<div class="entry-content">`)
	fmt.Fprintf(&html, `<input type="hidden" name="entries[%d][type]" value="%s"/>`, index, htmlpkg.EscapeString(entry.Type))

	switch entry.Type {
	case book.EntrySeparator:
		fmt.Fprintf(&html, `<div class="entry-separator"><span>%s</span></div>`, t("separator"))

	case book.EntryTitle:
		html.WriteString(`<div class="entry-title">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, t("title"))
		// preserve-only: no UI control for the depth, but a hand-written "## title" keeps
		// its "#" count across a save instead of collapsing to one "#" (new titles are H1)
		fmt.Fprintf(&html, `<input type="hidden" name="entries[%d][level]" value="%d"/>`, index, entry.Level)
		fmt.Fprintf(&html, `<input type="text" name="entries[%d][value]" value="%s" class="form-input" placeholder="%s"/>`, index, htmlpkg.EscapeString(entry.Value), t("enter title"))
		html.WriteString(`</div>`)

	case book.EntryFile:
		placeholder := t("search files")
		if bookMode {
			placeholder = t("search files (append #section)")
		}
		inputID := fmt.Sprintf("entry-file-%d", entryRowCounter.Add(1))
		html.WriteString(`<div class="entry-file">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, t("file"))
		html.WriteString(GenerateDatalistInput(inputID, fmt.Sprintf("entries[%d][value]", index), entry.Value, placeholder, "/api/files/autocomplete", false))
		// flag an entry whose target file no longer exists so a stale reference is
		// obvious in the editor, not only as a "could not include" marker in the view
		if p, _ := parser.ResolveWikiTarget(entry.Value); p != "" {
			if ok, _ := contentStorage.FileExists(pathutils.ToDocsPath(p)); !ok {
				fmt.Fprintf(&html, `<span class="entry-file-missing" title="%s">%s</span>`, htmlpkg.EscapeString(p), t("file not found"))
			}
		}
		if bookMode {
			checked := ""
			if entry.IncludeSubheaders {
				checked = " checked"
			}
			fmt.Fprintf(&html, `<label><input type="checkbox" name="entries[%d][subheaders]" value="true"%s/> %s</label>`, index, checked, t("include subheaders"))
		}
		html.WriteString(`</div>`)

	case book.EntryFilter:
		// a saved filter, resolved to its matching files when the book is composed
		html.WriteString(`<div class="entry-filter">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, t("filter"))
		fmt.Fprintf(&html, `<select name="entries[%d][value]" class="form-input">`, index)
		ids, _ := filter.GetAllFilters()
		// a missing (renamed/deleted) filter stays selected so a save doesn't silently swap it
		if entry.Value != "" && !slices.Contains(ids, entry.Value) {
			fmt.Fprintf(&html, `<option value="%s" selected>%s (%s)</option>`, htmlpkg.EscapeString(entry.Value), htmlpkg.EscapeString(entry.Value), t("filter not found"))
		}
		for _, id := range ids {
			selected := ""
			if id == entry.Value {
				selected = " selected"
			}
			fmt.Fprintf(&html, `<option value="%s"%s>%s</option>`, htmlpkg.EscapeString(id), selected, htmlpkg.EscapeString(id))
		}
		html.WriteString(`</select>`)
		html.WriteString(`</div>`)

	case book.EntryUnknown:
		// a block book.Parse didn't model (prose, a pasted table, a fenced snippet); shown
		// read-only but still submitted so a hand edit round-trips through a save instead of
		// being silently dropped
		rows := min(strings.Count(entry.Value, "\n")+1, 12)
		html.WriteString(`<div class="entry-unknown">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, t("unrecognized entry (kept as-is)"))
		fmt.Fprintf(&html, `<textarea name="entries[%d][value]" class="form-input" rows="%d" readonly>%s</textarea>`, index, rows, htmlpkg.EscapeString(entry.Value))
		html.WriteString(`</div>`)

	default:
		// genuinely unknown type (only reachable via a hand-crafted POST) - skip it
		return ""
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}

// RenderEntryRowHelper renders one htmx-added entry row, reusing renderEntryRow.
func RenderEntryRowHelper(index int, entry book.Entry, bookMode bool) string {
	html := renderEntryRow(index, entry, bookMode)

	// appended with placeholder index 999; reindex now so field names are correct even if
	// the user submits without reordering
	html += `<script>if(typeof window.reindexEntries==='function')window.reindexEntries();</script>`

	return html
}
