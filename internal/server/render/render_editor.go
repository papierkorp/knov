// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/files"
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
