// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
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

	action := "/api/files/create"
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

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	return "`" + s + "`"
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
			config = &filter.Config{}
			if err := json.Unmarshal([]byte(content), config); err != nil {
				logging.LogError("failed to parse existing filter config: %v", err)
				config = nil
			}
		}
	}

	// render the filter form with save functionality
	html.WriteString(`<div class="filter-form-container">`)
	html.WriteString(`<h4>filter configuration</h4>`)

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
	if isEdit {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">apply filter`, `class="btn-primary">save filter`, 1)
	} else {
		filterFormHTML = strings.Replace(filterFormHTML, `hx-target="#filter-results"`, `hx-target="#editor-status"`, 1)
		filterFormHTML = strings.Replace(filterFormHTML, `class="btn-primary">apply filter`, `class="btn-primary">save filter`, 1)
	}

	html.WriteString(filterFormHTML)
	html.WriteString(`<div id="editor-status"></div>`)
	html.WriteString(`</div>`)

	// render results container with preview functionality
	html.WriteString(`<div class="filter-results-container">`)
	html.WriteString(`<h4>filter preview</h4>`)
	html.WriteString(`<button type="button" hx-post="/api/filter" hx-include="#filter-form" hx-target="#filter-results" class="btn-secondary">preview results</button>`)
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-no-results">configure filter above and click preview to see results</p>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	return html.String(), nil
}
