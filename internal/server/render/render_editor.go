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
func RenderMarkdownEditorForm(filePath string, filetype ...string) string {
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

	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	// determine filetype for new files
	var currentFiletype string
	if len(filetype) > 0 {
		currentFiletype = filetype[0]
	}

	// add filepath input for new files (except fleeting)
	filepathInput := ""
	if !isEdit {
		if currentFiletype == "fleeting" {
			// for fleeting files, auto-generate filename and hide from user
			filepathInput = fmt.Sprintf(`<input type="hidden" name="filetype" value="%s" />`, currentFiletype)
		} else {
			filepathInput = fmt.Sprintf(`
				<div class="form-group">
					<label for="filepath-input">%s</label>
					<input type="text" id="filepath-input" name="filepath" required placeholder="%s" class="form-input" list="folder-suggestions" />
					<datalist id="folder-suggestions" hx-get="/api/files/folder-suggestions" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
				</div>`,
				translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
				translation.SprintfForRequest(configmanager.GetLanguage(), "my-file.md"))

			// add hidden filetype input for new files
			if currentFiletype != "" {
				filepathInput += fmt.Sprintf(`<input type="hidden" name="filetype" value="%s" />`, currentFiletype)
			}
		}
	} else {
		filepathInput = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, filePath)
	}

	formTemplate := `
		<form hx-post="%s" hx-target="#editor-status" class="file-form">
			%s
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
					theme: document.body.getAttribute('data-dark-mode') === 'true' ? 'dark' : 'default',
					hooks: {
						addImageBlobHook: function(blob, callback, source) {
							// Get current file path from URL - much simpler!
							const currentPath = window.location.pathname;
							let contextPath = null;

							// Extract filepath from URLs like /files/path/to/file.md or /files/edit/path/to/file.md
							if (currentPath.startsWith('/files/edit/')) {
								contextPath = currentPath.substring('/files/edit/'.length);
							} else if (currentPath.startsWith('/files/')) {
								contextPath = currentPath.substring('/files/'.length);
							}

							if (!contextPath) {
								alert('Please save the document first to enable image uploads.');
								return;
							}

							// Create form data for upload
							const formData = new FormData();
							formData.append('file', blob);
							formData.append('context_path', contextPath);

							// Show upload progress with proper dark mode styling
							const uploadMessage = document.createElement('div');
							const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';
							uploadMessage.className = 'upload-notification';
							uploadMessage.style.cssText = 'position: fixed; top: 10px; right: 10px; padding: 12px 16px; border-radius: 6px; z-index: 9999; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.15);';
							uploadMessage.style.backgroundColor = isDarkMode ? '#374151' : '#0ea5e9';
							uploadMessage.style.color = isDarkMode ? '#f9fafb' : '#ffffff';
							uploadMessage.textContent = 'Uploading image...';
							document.body.appendChild(uploadMessage);

							// Upload to media API
							fetch('/api/media/upload', {
								method: 'POST',
								body: formData,
								headers: {
									'Accept': 'application/json'
								}
							})
							.then(response => {
								if (!response.ok) {
									throw new Error('Upload failed: ' + response.statusText);
								}
								return response.json();
							})
							.then(data => {
								// Remove upload message
								document.body.removeChild(uploadMessage);

								// Insert the uploaded image with relative path
								const imagePath = 'media/' + data.path;
								callback(imagePath, blob.name || 'Uploaded Image');

								console.log('Image uploaded successfully:', data);
							})
							.catch(error => {
								// Remove upload message
								if (document.body.contains(uploadMessage)) {
									document.body.removeChild(uploadMessage);
								}

								console.error('Image upload failed:', error);
								alert('Failed to upload image: ' + error.message);

								// Call callback with empty string to cancel insertion
								callback('', '');
							});
						}
					}
				});

				document.querySelector('.file-form').addEventListener('submit', function(e) {
					document.getElementById('editor-content').value = editor.getMarkdown();
				});
			})();
		</script>
	`

	// determine save button text based on filetype
	saveButtonText := translation.SprintfForRequest(configmanager.GetLanguage(), "save file")
	if currentFiletype == "fleeting" && !isEdit {
		saveButtonText = translation.SprintfForRequest(configmanager.GetLanguage(), "save note")
	}

	return fmt.Sprintf(formTemplate, action,
		filepathInput,
		saveButtonText,
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		jsEscapeString(content))
}

// RenderMarkdownSectionEditorForm renders a markdown editor form for editing a specific section
func RenderMarkdownSectionEditorForm(filePath, sectionID string) string {
	content := ""

	// get section content
	if filePath != "" && sectionID != "" {
		sectionContent, err := files.ExtractSectionContent(filePath, sectionID)
		if err == nil {
			content = sectionContent
		}
	}

	// use section save endpoint
	action := "/api/files/section/save"

	cancelURL := fmt.Sprintf("/files/%s", filePath)

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="sectionid" value="%s" readonly />
			</div>
			<div class="form-group">
				<div id="markdown-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
				<input type="hidden" name="filepath" value="%s" />
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
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
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

	// determine action - always use filter save endpoint for filter files
	isEdit := filePath != ""
	action := "/api/filter/save"
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
