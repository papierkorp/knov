// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderMetadataForm renders the complete metadata form for a file
func RenderMetadataForm(filePath string, defaultFiletype string) (string, error) {
	var html strings.Builder
	var metadata *files.Metadata
	var err error

	if filePath != "" {
		metadata, err = files.MetaDataGet(filePath)
		if err != nil {
			return "", err
		}
	}

	html.WriteString(`<form id="metadata-form" class="metadata-form">`)
	// basic metadata section - all fields in one section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "metadata") + `</h3>`)

	// file path field (editable)
	path := filePath
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-path">` + translation.SprintfForRequest(configmanager.GetLanguage(), "file path") + `</label>`)
	html.WriteString(GenerateInputWithSaveOnBlur("meta-path", "newpath", path,
		translation.SprintfForRequest(configmanager.GetLanguage(), "enter file path"),
		filePath, "/api/metadata/path"))
	html.WriteString(`</div>`)

	// editor field - use defaultFiletype if provided and no existing metadata
	editor := defaultFiletype
	if metadata != nil {
		editor = string(metadata.Editor)
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-editor">` + translation.SprintfForRequest(configmanager.GetLanguage(), "editor") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-editor", "editor", editor,
		translation.SprintfForRequest(configmanager.GetLanguage(), "select editor"),
		"/api/metadata/editors?format=options", filePath, "/api/metadata/editor"))
	html.WriteString(`</div>`)

	// parents field
	parentsStr := ""
	if metadata != nil && len(metadata.Parents) > 0 {
		parentsStr = strings.Join(metadata.Parents, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-parents">` + translation.SprintfForRequest(configmanager.GetLanguage(), "parents") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-parents", "parents", parentsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add parent files"),
		"/api/files/list?format=options", filePath, "/api/metadata/parents"))
	html.WriteString(`</div>`)

	// tags field
	tagsStr := ""
	if metadata != nil && len(metadata.Tags) > 0 {
		tagsStr = strings.Join(metadata.Tags, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-tags">` + translation.SprintfForRequest(configmanager.GetLanguage(), "tags") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-tags", "tags", tagsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
		"/api/metadata/tags?format=options", filePath, "/api/metadata/tags"))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)  // close basic form group
	html.WriteString(`</form>`) // close metadata form

	return html.String(), nil
}

// RenderReferencesSidebarHTML renders a read-only references list for the sidebar
func RenderReferencesSidebarHTML(refs []files.Reference) string {
	var html strings.Builder
	html.WriteString(`<div class="references-list">`)
	if len(refs) == 0 {
		fmt.Fprintf(&html, `<span class="no-items">%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no references"))
	}
	for _, ref := range refs {
		html.WriteString(`<div class="reference-item">`)
		fmt.Fprintf(&html, `<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, ref.URL, ref.URL)
		if ref.Description != "" {
			fmt.Fprintf(&html, `<span class="reference-description">%s</span>`, ref.Description)
		}
		html.WriteString(`</div>`)
	}
	html.WriteString(`</div>`)
	return html.String()
}

// RenderReferencesHTML renders the references list with a delete button per entry
func RenderReferencesHTML(refs []files.Reference) string {
	var html strings.Builder
	html.WriteString(`<div id="component-references-list">`)
	if len(refs) == 0 {
		fmt.Fprintf(&html, `<p class="no-items">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no references"))
	}
	for _, ref := range refs {
		html.WriteString(`<div class="reference-item">`)
		html.WriteString(`<div class="reference-item-main">`)
		fmt.Fprintf(&html, `<a href="%s" target="_blank" rel="noopener noreferrer" class="reference-url">%s</a>`, ref.URL, ref.URL)
		if ref.Description != "" {
			fmt.Fprintf(&html, `<span class="reference-description">%s</span>`, ref.Description)
		}
		html.WriteString(`</div>`)
		html.WriteString(`<div class="reference-item-actions">`)
		fmt.Fprintf(&html, `<button hx-delete="/api/metadata/references" hx-vals='{"url":"%s"}' hx-include="#reference-filepath" hx-target="#component-references-list" hx-swap="outerHTML" class="btn-icon btn-danger-icon" title="%s"><i class="fa fa-trash"></i></button>`,
			ref.URL, translation.SprintfForRequest(configmanager.GetLanguage(), "remove"))
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)
	}
	html.WriteString(`</div>`)
	return html.String()
}

// RenderMetadataCSV generates CSV content for metadata export
func RenderMetadataCSV(metadata []*files.Metadata) string {
	var csv strings.Builder

	// header
	csv.WriteString("path,title,collection,editor,createdat,lastedited,tags,folders\n")

	for _, m := range metadata {
		if m == nil {
			continue
		}

		// escape csv values
		path := escapeCSV(m.Path)
		name := escapeCSV(m.Title)
		collection := escapeCSV(m.Collection)
		editor := escapeCSV(string(m.Editor))
		createdat := m.CreatedAt.Format("2006-01-02 15:04:05")
		lastedited := m.LastEdited.Format("2006-01-02 15:04:05")
		tags := escapeCSV(strings.Join(m.Tags, ";"))
		folders := escapeCSV(strings.Join(m.Folders, ";"))

		csv.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n",
			path, name, collection, editor, createdat, lastedited,
			tags, folders))
	}

	return csv.String()
}

// escapeCSV escapes a string for CSV format
func escapeCSV(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

// RenderFileMetadataSimple renders a simple metadata display for regular files
func RenderFileMetadataSimple(metadata *files.Metadata) string {
	if metadata == nil {
		return `<div class="component-error">` +
			translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found") +
			`</div>`
	}

	var html strings.Builder
	html.WriteString(`<div class="component-metadata">`)
	fmt.Fprintf(&html, `<p>%s: %s</p>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path"), metadata.Path)
	fmt.Fprintf(&html, `<p>%s: %s</p>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "collection"), metadata.Collection)
	fmt.Fprintf(&html, `<p>%s: %s</p>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "editor"), metadata.Editor)

	if len(metadata.Tags) > 0 {
		fmt.Fprintf(&html, `<p>%s: %s</p>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "tags"),
			strings.Join(metadata.Tags, ", "))
	}
	html.WriteString(`</div>`)

	return html.String()
}
