// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/files"
	"knov/internal/utils"
)

// RenderFilesOptions renders file list as select options
func RenderFilesOptions(allFiles []files.File) string {
	var html strings.Builder
	html.WriteString(`<option value="">select a file...</option>`)
	for _, file := range allFiles {
		path := strings.TrimPrefix(file.Path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, path, path))
	}
	return html.String()
}

// RenderFilesList renders files as interactive list with HTMX
func RenderFilesList(allFiles []files.File) string {
	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range allFiles {
		html.WriteString(fmt.Sprintf(`
			<li>
				<a href="#"
					hx-get="/files/%s?snippet=true"
					hx-target="#file-content"
					hx-on::after-request="htmx.ajax('GET', '/api/files/header?filepath=%s', {target: '#file-header'})"
				>%s</a>
			</li>`,
			file.Path,
			file.Path,
			file.Path))
	}
	html.WriteString("</ul>")
	return html.String()
}

// RenderFilteredFiles renders filtered files list with count - reuses RenderFileList
func RenderFilteredFiles(filteredFiles []files.File) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>found %d files</p>", len(filteredFiles)))
	html.WriteString(RenderFileList(filteredFiles))
	return html.String()
}

// RenderFileHeader renders file header with breadcrumb
func RenderFileHeader(filepath string) string {
	return fmt.Sprintf(`<div id="current-file-breadcrumb"><a href="/files/%s">→ %s</a></div>`, filepath, filepath)
}

// RenderBrowseFilesHTML renders browsed files as list - reuses RenderFileList
func RenderBrowseFilesHTML(files []files.File) string {
	if len(files) == 0 {
		return "<p>no files found</p>"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>found %d files</p>", len(files)))
	html.WriteString(RenderFileList(files))
	return html.String()
}

// RenderFileForm renders a simple file creation/editing form
func RenderFileForm(filePath string) string {
	return fmt.Sprintf(`
		<form class="file-form">
			<div class="form-group">
				<label>File Path:</label>
				<input type="text" name="filepath" value="%s" placeholder="path/to/file.md" />
			</div>
			<div class="form-group">
				<label>Content:</label>
				<textarea name="content" rows="10" placeholder="File content here..."></textarea>
			</div>
			<button type="submit">Save File</button>
		</form>`, filePath)
}

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
					previewStyle: 'vertical',
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

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	return "`" + s + "`"
}
