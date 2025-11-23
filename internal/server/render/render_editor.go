// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/utils"
)

// RenderMarkdownEditorForm renders a markdown editor form for file creation/editing
func RenderMarkdownEditorForm(filePath string) string {
	content := ""
	isEdit := filePath != ""

	if isEdit {
		fullPath := utils.ToFullPath(filePath)
		rawContent, err := files.GetRawContent(fullPath)
		if err == nil {
			content = rawContent
		}
	}

	action := "/api/files/new"
	if isEdit {
		action = fmt.Sprintf("/api/files/save/%s", filePath)
	}

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
				<label>file path:</label>
				<input type="text" name="filepath" value="%s" placeholder="path/to/file.md" %s required />
			</div>
			<div class="form-group">
				<div id="markdown-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">save file</button>
				<button type="button" onclick="window.location.href='%s'" class="btn-secondary">cancel</button>
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
	`, action, filePath, pathReadonly, cancelURL, jsEscapeString(content))
}

// RenderTextareaEditorComponent renders a textarea editor component with save/cancel buttons
func RenderTextareaEditorComponent(filepath, content string) string {
	return fmt.Sprintf(`
		<div id="component-textarea-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">save</button>
					<button type="button" onclick="location.reload()" class="btn-secondary">cancel</button>
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content)
}

// RenderFilterEditor renders a filter editor with live preview
func RenderFilterEditor(filepath, content string) string {
	isEdit := filepath != ""

	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	pathReadonly := ""
	pathPlaceholder := "path/to/myfilter.filter"
	if isEdit {
		pathReadonly = "readonly"
		pathPlaceholder = filepath
	}

	saveURL := "/api/files/new"
	if isEdit {
		saveURL = fmt.Sprintf("/api/files/save/%s", filepath)
	}

	// parse existing filter config from JSON content
	var config *filter.Config
	if content != "" {
		var tempConfig filter.Config
		if err := json.Unmarshal([]byte(content), &tempConfig); err == nil {
			config = &tempConfig
		}
	}

	// default config for new filters
	if config == nil {
		config = &filter.Config{
			Criteria: []filter.Criteria{},
			Logic:    "and",
			Display:  "list",
			Limit:    50,
		}
	}

	// render the filter criteria fields (logic selector + criteria, no submit button)
	filterFormHTML := RenderFilterCriteriaFields(config)

	return fmt.Sprintf(`
		<div id="component-filter-editor">
			<form id="filter-save-form" hx-post="%s" hx-target="#editor-status">
				<div class="form-group">
					<label>file path:</label>
					<input type="text" name="filepath" id="filter-filepath" value="%s" placeholder="%s" %s required class="form-input" />
				</div>
				<input type="hidden" name="content" id="filter-content" />
				<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 20px;">
					<div>
						<h3>filter configuration</h3>
						<div id="filter-form-container">
							%s
						</div>
						<div style="margin-top: 12px;">
							<label>display mode:</label>
							<select name="display" id="filter-display" class="form-select">
								<option value="list" %s>list</option>
								<option value="cards" %s>cards</option>
								<option value="dropdown" %s>dropdown</option>
								<option value="table" %s>table</option>
							</select>
						</div>
						<div style="margin-top: 12px;">
							<label>limit:</label>
							<input type="number" name="limit" id="filter-limit" value="%d" min="1" class="form-input" />
						</div>
						<div class="form-actions" style="margin-top: 12px;">
							<button type="button" onclick="prepareAndSubmitFilter()" class="btn-primary">save filter</button>
							<button type="button" onclick="window.location.href='%s'" class="btn-secondary">cancel</button>
							<button type="button" hx-post="/api/filter" hx-include="#filter-save-form" hx-target="#filter-preview-results" hx-vals='js:{display: document.getElementById("filter-display").value, limit: document.getElementById("filter-limit").value}' class="btn-secondary">preview results</button>
						</div>
						<div id="editor-status"></div>
					</div>
					<div>
						<h3>preview</h3>
						<div id="filter-preview-results">
							<p style="color: #666;">click "preview results" to see filtered files</p>
						</div>
					</div>
				</div>
			</form>
		</div>
		<script>
			function prepareAndSubmitFilter() {
				let filepath = document.getElementById('filter-filepath').value.trim();
				if (!filepath) {
					document.getElementById('editor-status').innerHTML = '<p style="color: red;">please enter a file path</p>';
					return;
				}

				// auto-append .filter extension if not present
				if (!filepath.endsWith('.filter')) {
					filepath += '.filter';
					document.getElementById('filter-filepath').value = filepath;
				}

				const filterForm = document.getElementById('filter-save-form');
				const formData = new FormData(filterForm);

				// build filter config JSON
				const config = {
					criteria: [],
					logic: formData.get('logic') || 'and',
					display: document.getElementById('filter-display').value,
					limit: parseInt(document.getElementById('filter-limit').value) || 50
				};

				// extract criteria from form data
				const criteriaMap = {};
				for (const [key, value] of formData.entries()) {
					if (key.startsWith('metadata[')) {
						const index = key.match(/\[(\d+)\]/)[1];
						if (!criteriaMap[index]) criteriaMap[index] = {};
						criteriaMap[index].metadata = value;
					} else if (key.startsWith('operator[')) {
						const index = key.match(/\[(\d+)\]/)[1];
						if (!criteriaMap[index]) criteriaMap[index] = {};
						criteriaMap[index].operator = value;
					} else if (key.startsWith('value[')) {
						const index = key.match(/\[(\d+)\]/)[1];
						if (!criteriaMap[index]) criteriaMap[index] = {};
						criteriaMap[index].value = value;
					} else if (key.startsWith('action[')) {
						const index = key.match(/\[(\d+)\]/)[1];
						if (!criteriaMap[index]) criteriaMap[index] = {};
						criteriaMap[index].action = value;
					}
				}

				// convert to array
				Object.keys(criteriaMap).forEach(index => {
					const c = criteriaMap[index];
					if (c.metadata && c.operator && c.value) {
						config.criteria.push({
							metadata: c.metadata,
							operator: c.operator || 'equals',
							value: c.value,
							action: c.action || 'include'
						});
					}
				});

				// convert to JSON and populate hidden field
				const jsonContent = JSON.stringify(config, null, 2);
				document.getElementById('filter-content').value = jsonContent;

				// submit form via HTMX
				document.getElementById('filter-save-form').dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
			}
		</script>
	`,
		saveURL,
		filepath,
		pathPlaceholder,
		pathReadonly,
		filterFormHTML,
		utils.Ternary(config.Display == "list", "selected", ""),
		utils.Ternary(config.Display == "cards", "selected", ""),
		utils.Ternary(config.Display == "dropdown", "selected", ""),
		utils.Ternary(config.Display == "table", "selected", ""),
		config.Limit,
		cancelURL)
}

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	return "`" + s + "`"
}
