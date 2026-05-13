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
	html.WriteString(`<div id="metadata-save-status" class="save-status"></div>`)

	// basic metadata section - all fields in one section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "metadata") + `</h3>`)

	// file path field (editable)
	path := filePath
	if path == "" {
		path = ""
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-path">` + translation.SprintfForRequest(configmanager.GetLanguage(), "file path") + `</label>`)
	html.WriteString(GenerateInputWithSaveOnBlur("meta-path", "newpath", path,
		translation.SprintfForRequest(configmanager.GetLanguage(), "enter file path"),
		filePath, "/api/metadata/path"))
	html.WriteString(`</div>`)

	// priority field
	priority := ""
	if metadata != nil {
		priority = string(metadata.Priority)
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-priority">` + translation.SprintfForRequest(configmanager.GetLanguage(), "priority") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-priority", "priority", priority,
		translation.SprintfForRequest(configmanager.GetLanguage(), "set priority (1-5)"),
		"/api/metadata/priorities?format=options", filePath, "/api/metadata/priority"))
	html.WriteString(`</div>`)

	// collection field
	collection := ""
	if metadata != nil {
		collection = metadata.Collection
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-collection">` + translation.SprintfForRequest(configmanager.GetLanguage(), "collection") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-collection", "collection", collection,
		translation.SprintfForRequest(configmanager.GetLanguage(), "assign to collection"),
		"/api/metadata/collections?format=options", filePath, "/api/metadata/collection"))
	html.WriteString(`</div>`)

	// filetype field - use defaultFiletype if provided and no existing metadata
	filetype := defaultFiletype
	if metadata != nil {
		filetype = string(metadata.FileType)
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-filetype">` + translation.SprintfForRequest(configmanager.GetLanguage(), "file type") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-filetype", "filetype", filetype,
		translation.SprintfForRequest(configmanager.GetLanguage(), "select file type"),
		"/api/metadata/filetypes?format=options", filePath, "/api/metadata/filetype"))
	html.WriteString(`</div>`)

	// status field
	status := ""
	if metadata != nil {
		status = string(metadata.Status)
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-status">` + translation.SprintfForRequest(configmanager.GetLanguage(), "status") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-status", "status", status,
		translation.SprintfForRequest(configmanager.GetLanguage(), "select status"),
		"/api/metadata/statuses?format=options", filePath, "/api/metadata/status"))
	html.WriteString(`</div>`)

	// target date field
	targetDate := ""
	if metadata != nil && !metadata.TargetDate.IsZero() {
		targetDate = metadata.TargetDate.Format("2006-01-02")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-targetdate">` + translation.SprintfForRequest(configmanager.GetLanguage(), "target date") + `</label>`)
	html.WriteString(GenerateDateInputWithSave("meta-targetdate", "targetdate", targetDate, filePath, "/api/metadata/targetdate"))
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

	// folders field
	foldersStr := ""
	if metadata != nil && len(metadata.Folders) > 0 {
		foldersStr = strings.Join(metadata.Folders, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-folders">` + translation.SprintfForRequest(configmanager.GetLanguage(), "folders") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-folders", "folders", foldersStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add folder categories"),
		"/api/metadata/folders?format=options", filePath, "/api/metadata/folders"))
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
		fmt.Fprintf(&html, `<div class="reference-item"><a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
			ref.URL, ref.URL)
		if ref.Description != "" {
			fmt.Fprintf(&html, `<span class="reference-description">%s</span>`, ref.Description)
		}
		fmt.Fprintf(&html, `<button hx-delete="/api/metadata/references" hx-vals='{"url":"%s"}' hx-include="#reference-filepath" hx-target="#component-references-list" hx-swap="outerHTML" class="btn-danger btn-sm">%s</button>`,
			ref.URL, translation.SprintfForRequest(configmanager.GetLanguage(), "remove"))
		html.WriteString(`</div>`)
	}
	html.WriteString(`</div>`)
	return html.String()
}

// RenderMetadataCSV generates CSV content for metadata export
func RenderMetadataCSV(metadata []*files.Metadata) string {
	var csv strings.Builder

	// header
	csv.WriteString("path,title,collection,filetype,status,priority,createdat,lastedited,tags,folders\n")

	for _, m := range metadata {
		if m == nil {
			continue
		}

		// escape csv values
		path := escapeCSV(m.Path)
		name := escapeCSV(m.Title)
		collection := escapeCSV(m.Collection)
		filetype := escapeCSV(string(m.FileType))
		status := escapeCSV(string(m.Status))
		priority := escapeCSV(string(m.Priority))
		createdat := m.CreatedAt.Format("2006-01-02 15:04:05")
		lastedited := m.LastEdited.Format("2006-01-02 15:04:05")
		tags := escapeCSV(strings.Join(m.Tags, ";"))
		folders := escapeCSV(strings.Join(m.Folders, ";"))

		csv.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			path, name, collection, filetype, status, priority, createdat, lastedited,
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
		translation.SprintfForRequest(configmanager.GetLanguage(), "type"), metadata.FileType)
	fmt.Fprintf(&html, `<p>%s: %s</p>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "status"), metadata.Status)
	fmt.Fprintf(&html, `<p>%s: %s</p>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "priority"), metadata.Priority)

	if len(metadata.Tags) > 0 {
		fmt.Fprintf(&html, `<p>%s: %s</p>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "tags"),
			strings.Join(metadata.Tags, ", "))
	}
	html.WriteString(`</div>`)

	return html.String()
}
