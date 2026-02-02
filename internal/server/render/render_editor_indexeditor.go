// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

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

		// file link (- [text](path))
		if strings.HasPrefix(line, "- [") {
			// extract path from markdown link: - [text](path)
			start := strings.Index(line, "](")
			end := strings.Index(line, ")")
			if start != -1 && end != -1 && end > start {
				path := line[start+2 : end]
				config.Entries = append(config.Entries, IndexEntry{
					Type:  "file",
					Value: path,
				})
			}
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
func RenderIndexEditor(filePath string) (string, error) {
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

	// form header
	html.WriteString(`<div class="index-form-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "index configuration") + `</h4>`)

	// determine action
	isEdit := filePath != ""
	action := "/api/editor/indexeditor"

	fmt.Fprintf(&html, `<form hx-post="%s" hx-target="#editor-status" hx-swap="innerHTML" id="index-form">`, action)

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

	// save button
	html.WriteString(`<div class="form-actions">`)
	saveText := translation.SprintfForRequest(configmanager.GetLanguage(), "save index")
	fmt.Fprintf(&html, `<button type="submit" class="btn-primary">%s</button>`, saveText)
	html.WriteString(`</div>`)

	html.WriteString(`</form>`)
	html.WriteString(`<div id="editor-status"></div>`)

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
		console.log('reindexEntries: container not found');
		return;
	}
	const allRows = container.querySelectorAll('.entry-row');
	console.log('reindexEntries: found', allRows.length, 'rows');
	allRows.forEach((r, i) => {
		r.setAttribute('data-entry-index', i);
		r.querySelectorAll('[name^="entries["]').forEach(input => {
			const name = input.getAttribute('name');
			// Replace entries[any_number] with entries[i]
			const newName = name.replace(/entries\[\d+\]/, 'entries[' + i + ']');
			console.log('reindexEntries: renaming', name, '->', newName);
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
		inputID := fmt.Sprintf("entry-file-%d", index)
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
