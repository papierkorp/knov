// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderFilesOptions renders file list as select options
func RenderFilesOptions(allFiles []files.File) string {
	var html strings.Builder
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select a file...") + `</option>`)
	for _, file := range allFiles {
		path := strings.TrimPrefix(file.Path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, path, path))
	}
	return html.String()
}

// RenderFilesDatalist renders files as datalist options
func RenderFilesDatalist(allFiles []files.File) string {
	var html strings.Builder
	for _, file := range allFiles {
		path := strings.TrimPrefix(file.Path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">`, path))
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
	html.WriteString(fmt.Sprintf("<p>%s</p>", translation.SprintfForRequest(configmanager.GetLanguage(), "found %d files", len(filteredFiles))))
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
		return "<p>" + translation.SprintfForRequest(configmanager.GetLanguage(), "no files found") + "</p>"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>%s</p>", translation.SprintfForRequest(configmanager.GetLanguage(), "found %d files", len(files))))
	html.WriteString(RenderFileList(files))
	return html.String()
}

// RenderFileForm renders a simple file creation/editing form
func RenderFileForm(filePath string) string {
	return fmt.Sprintf(`
		<form class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="filepath" value="%s" placeholder="%s" />
			</div>
			<div class="form-group">
				<label>%s:</label>
				<textarea name="content" rows="10" placeholder="%s"></textarea>
			</div>
			<button type="submit">%s</button>
		</form>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/file.md"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "content"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "file content here..."),
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"))
}
