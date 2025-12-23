// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/translation"
)

// RenderMarkdownEditorForm renders a markdown editor form for file creation/editing
func RenderMarkdownEditorForm(filePath string) string {
	content := ""
	isEdit := filePath != ""

	if isEdit {
		rawContent, err := files.GetRawContent(filePath)
		if err == nil {
			content = rawContent
		}
	}

	// use same endpoint for both create and edit
	action := "/api/files/save"

	pathReadonly := ""
	if isEdit {
		pathReadonly = "readonly"
	}

	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="filepath" value="%s" placeholder="%s" %s required />
			</div>
			<div class="form-group">
				<div id="markdown-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
			</div>
			<div id="editor-status"></div>
		</form>
		<script>
			(function() {
				const editor = new toastui.Editor({
					el: document.querySelector('#markdown-editor'),
					height: '500px',
					initialEditType: 'markdown',
					previewStyle: 'tab',
					initialValue: %s,
					theme: document.body.getAttribute('data-dark-mode') === 'true' ? 'dark' : 'default'
				});

				document.querySelector('.file-form').addEventListener('submit', function(e) {
					document.getElementById('editor-content').value = editor.getMarkdown();
				});
			})();
		</script>
	`, action,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/file.md"),
		pathReadonly,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		jsEscapeString(content))
}

// RenderTextareaEditorComponent renders a textarea editor component with save/cancel buttons
func RenderTextareaEditorComponent(filepath, content string) string {
	cancelURL := "/"
	if filepath != "" {
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	return fmt.Sprintf(`
		<div id="component-textarea-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">%s</button>
					<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"))
}

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		logging.LogError("failed to marshal string for javascript: %v", err)
		return `""`
	}
	return string(jsonBytes)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- Filter Editor -----------------------------------
// ----------------------------------------------------------------------------------------

// RenderFilterEditor renders a filter editor with form and result display
func RenderFilterEditor(filePath string) (string, error) {
	var html strings.Builder

	html.WriteString(`<div class="filter-editor" id="filter-editor">`)

	// load existing config if editing
	var config *filter.Config
	if filePath != "" {
		// for existing filter files, try to load the saved JSON
		if content, err := files.GetRawContent(filePath); err == nil {
			if len(content) == 0 {
				// use default configuration for empty files
				config = &filter.Config{
					Criteria: []filter.Criteria{},
					Logic:    "and",
					Display:  "list",
					Limit:    50,
				}
				logging.LogInfo("using default configuration for empty filter file in editor: %s", filePath)
			} else {
				config = &filter.Config{}
				if err := json.Unmarshal([]byte(content), config); err != nil {
					logging.LogError("failed to parse existing filter config: %v", err)
					config = nil
				}
			}
		}
	}

	// render the filter form with save functionality
	html.WriteString(`<div class="filter-form-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter configuration") + `</h4>`)

	// determine action and whether to include filepath input
	isEdit := filePath != ""
	action := "/api/filter/save"
	if isEdit {
		action = fmt.Sprintf("/api/files/save/%s", filePath)
	}
	includeFilePath := !isEdit

	// use the updated RenderFilterFormWithAction
	filterFormHTML := RenderFilterFormWithAction(config, action, filePath, includeFilePath)

	// modify the form to change button text and target
	applyFilterText := translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter")
	saveFilterText := translation.SprintfForRequest(configmanager.GetLanguage(), "save filter")

	if isEdit {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">`+applyFilterText, `class="btn-primary">`+saveFilterText, 1)
	} else {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">`+applyFilterText, `class="btn-primary">`+saveFilterText, 1)
	}

	html.WriteString(filterFormHTML)
	html.WriteString(`<div id="editor-status"></div>`)
	html.WriteString(`</div>`)

	// render results container with preview functionality
	html.WriteString(`<div class="filter-results-container">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter preview") + `</h4>`)
	html.WriteString(`<button type="button" hx-post="/api/filter" hx-include="#filter-form" hx-target="#filter-results" class="btn-secondary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "preview results") + `</button>`)
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "configure filter above and click preview to see results") + `</p>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	return html.String(), nil
}

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
		if strings.HasPrefix(line, "## ") {
			title := strings.TrimPrefix(line, "## ")
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
		if content, err := files.GetRawContent(filePath); err == nil {
			if len(content) > 0 {
				// parse markdown to extract entries
				config = ParseMarkdownToIndexConfig(content)
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

	html.WriteString(fmt.Sprintf(`<form hx-post="%s" hx-target="#editor-status" hx-swap="innerHTML" id="index-form">`, action))

	// filepath input for new files
	if !isEdit {
		html.WriteString(`<div class="form-group">`)
		html.WriteString(fmt.Sprintf(`<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file path")))
		html.WriteString(fmt.Sprintf(`<input type="text" name="filepath" class="form-input" placeholder="%s" required/>`, translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/file")))
		html.WriteString(`</div>`)
	} else {
		html.WriteString(fmt.Sprintf(`<input type="hidden" name="filepath" value="%s"/>`, filePath))
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
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"separator"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add separator")))
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"file"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add file")))
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/editor/indexeditor/add-entry" hx-vals='{"type":"title"}' hx-target="#entries-container" hx-swap="beforeend" class="btn-secondary">%s</button>`, translation.SprintfForRequest(configmanager.GetLanguage(), "add title")))
	html.WriteString(`</div>`)

	// save button
	html.WriteString(`<div class="form-actions">`)
	saveText := translation.SprintfForRequest(configmanager.GetLanguage(), "save index")
	html.WriteString(fmt.Sprintf(`<button type="submit" class="btn-primary">%s</button>`, saveText))
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

	html.WriteString(fmt.Sprintf(`<div class="entry-row" data-entry-index="%d">`, index))

	// controls on the left
	html.WriteString(`<div class="entry-controls">`)
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, -1)" class="btn-move"><i class="fa-solid fa-arrow-up"></i></button>`, index))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, 1)" class="btn-move"><i class="fa-solid fa-arrow-down"></i></button>`, index))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="removeEntry(this)" class="btn-remove"><i class="fa-solid fa-xmark"></i></button>`))
	html.WriteString(`</div>`)

	// content on the right
	html.WriteString(`<div class="entry-content">`)
	html.WriteString(fmt.Sprintf(`<input type="hidden" name="entries[%d][type]" value="%s"/>`, index, entry.Type))

	switch entry.Type {
	case "separator":
		html.WriteString(`<div class="entry-separator">`)
		html.WriteString(fmt.Sprintf(`<span>%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "separator")))
		html.WriteString(`</div>`)

	case "file":
		html.WriteString(`<div class="entry-file">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file")))
		inputID := fmt.Sprintf("entry-file-%d", index)
		html.WriteString(GenerateDatalistInput(inputID, fmt.Sprintf("entries[%d][value]", index), entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "search files"), "/api/files/list?format=datalist"))
		html.WriteString(`</div>`)

	case "title":
		html.WriteString(`<div class="entry-title">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "title")))
		html.WriteString(fmt.Sprintf(`<input type="text" name="entries[%d][value]" value="%s" class="form-input" placeholder="%s"/>`, index, entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "enter title")))
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}
