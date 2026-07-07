// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"
	"sync/atomic"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// indexEntryRowCounter generates unique DOM ids for index entry rows added
// dynamically via htmx, whose array index is always the placeholder value 999
// and would otherwise collide when multiple rows are added in one session.
var indexEntryRowCounter atomic.Uint64

// ----------------------------------------------------------------------------
// ---------------------------------- Index Editor ------------------------------------
// ----------------------------------------------------------------------------

// ParseMarkdownToIndexConfig parses markdown content back into IndexConfig
func ParseMarkdownToIndexConfig(content string) *IndexConfig {
	config := &IndexConfig{
		Entries: []IndexEntry{},
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// separator
		if line == "---" {
			config.Entries = append(config.Entries, IndexEntry{
				Type:  "separator",
				Value: "",
			})
			continue
		}

		// title (## header)
		if title, found := strings.CutPrefix(line, "## "); found {
			config.Entries = append(config.Entries, IndexEntry{
				Type:  "title",
				Value: title,
			})
			continue
		}

		// file link, stored on disk as a [[wikilink]] (- [[path]] or - [[path#anchor]])
		if value, found := strings.CutPrefix(line, "- [["); found {
			value = strings.TrimSuffix(value, "]]")
			config.Entries = append(config.Entries, IndexEntry{
				Type:  "file",
				Value: value,
			})
			continue
		}
	}

	return config
}

// IndexEntry represents an entry in the index/MOC editor
type IndexEntry struct {
	Type  string `json:"type"`  // "separator", "file", "title"
	Value string `json:"value"` // file path or title text
}

// IndexConfig represents the configuration for an index/MOC file
type IndexConfig struct {
	Entries []IndexEntry `json:"entries"`
}

// RenderIndexEditor renders an index/MOC editor with htmx form
func RenderIndexEditor(filePath string, initialTitle ...string) (string, error) {
	var html strings.Builder

	html.WriteString(`<div class="index-editor" id="index-editor">`)

	// load existing config if editing
	var config *IndexConfig
	if filePath != "" {
		fullPath := pathutils.ToDocsPath(filePath)
		if content, err := contentStorage.ReadFile(fullPath); err == nil {
			if len(content) > 0 {
				// parse markdown to extract entries
				config = ParseMarkdownToIndexConfig(string(content))
			}
		}
	}

	if config == nil {
		config = &IndexConfig{
			Entries: []IndexEntry{},
		}
	}

	if len(initialTitle) > 0 && initialTitle[0] != "" {
		config.Entries = append([]IndexEntry{{Type: "title", Value: initialTitle[0]}}, config.Entries...)
	}

	// form header
	html.WriteString(`<div class="index-form-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "index configuration") + `</h4>`)

	// determine action and cancel destination
	isEdit := filePath != ""
	action := "/api/editor/indexeditor"
	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	fmt.Fprintf(&html, `<form hx-post="%s" hx-target="#index-editor-status" hx-swap="innerHTML" id="index-form">`, action)

	// filepath input for new files
	if !isEdit {
		html.WriteString(`<div class="form-group">`)
		fmt.Fprintf(&html, `<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file path"))
		datalistInput := GenerateDatalistInput("filepath-input", "filepath", "", translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/file"), "/api/files/folder-suggestions")
		// add required attribute
		datalistInput = strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1)
		html.WriteString(datalistInput)
		html.WriteString(`</div>`)
	} else {
		fmt.Fprintf(&html, `<input type="hidden" name="filepath" value="%s"/>`, filePath)
	}

	// entries container
	html.WriteString(`<div id="entries-container" class="entries-container">`)

	// render existing entries
	for i, entry := range config.Entries {
		html.WriteString(renderIndexEntryRow(i, entry))
	}

	html.WriteString(`</div>`)

	// add entry buttons
	html.WriteString(`<div class="form-actions">`)
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"separator"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add separator"))
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"file"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add file"))
	fmt.Fprintf(&html, `<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"title"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add title"))
	html.WriteString(`</div>`)

	// save + cancel buttons
	html.WriteString(`<div class="form-actions">`)
	fmt.Fprintf(&html, `<button type="submit" class="btn-primary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "save index"))
	fmt.Fprintf(&html, `<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>`, cancelURL, translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"))
	html.WriteString(`</div>`)
	html.WriteString(`<div id="index-editor-status"></div>`)

	html.WriteString(`</form>`)

	// add JavaScript functions BEFORE closing the container - use window scope for HTMX compatibility
	html.WriteString(`<script>
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
</script>`)

	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	return html.String(), nil
}

// renderIndexEntryRow renders a single index entry row
func renderIndexEntryRow(index int, entry IndexEntry) string {
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
	fmt.Fprintf(&html, `<input type="hidden" name="entries[%d][type]" value="%s"/>`, index, entry.Type)

	switch entry.Type {
	case "separator":
		html.WriteString(`<div class="entry-separator">`)
		fmt.Fprintf(&html, `<span>%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "separator"))
		html.WriteString(`</div>`)

	case "file":
		html.WriteString(`<div class="entry-file">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file"))
		inputID := fmt.Sprintf("entry-file-%d", indexEntryRowCounter.Add(1))
		html.WriteString(GenerateDatalistInput(inputID, fmt.Sprintf("entries[%d][value]", index), entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "search files"), "/api/files/list?format=datalist"))
		html.WriteString(`</div>`)

	case "title":
		html.WriteString(`<div class="entry-title">`)
		fmt.Fprintf(&html, `<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "title"))
		fmt.Fprintf(&html, `<input type="text" name="entries[%d][value]" value="%s" class="form-input" placeholder="%s"/>`, index, entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "enter title"))
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}

// RenderIndexEntryRowHelper generates HTML for a single index entry row added
// dynamically via htmx, reusing the same row markup as the initial render so
// there is only one place that builds an entry row.
func RenderIndexEntryRowHelper(index int, entry IndexEntry) string {
	html := renderIndexEntryRow(index, entry)

	// Use HTMX event to trigger reindexing after content is swapped
	html += `<script>
document.body.addEventListener('htmx:afterSwap', function(evt) {
	if (evt.detail.target.id === 'entries-container') {
		if (typeof window.reindexEntries === 'function') {
			window.reindexEntries();
		}
	}
});
</script>`

	return html
}
