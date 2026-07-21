// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"path/filepath"
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
		if !ref.AddedAt.IsZero() {
			fmt.Fprintf(&html, `<span class="reference-date">%s</span>`, configmanager.FormatDate(ref.AddedAt))
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
		if !ref.AddedAt.IsZero() {
			fmt.Fprintf(&html, `<span class="reference-date">%s</span>`, configmanager.FormatDate(ref.AddedAt))
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

// RenderBrokenLinksHTML renders the scan result of FindBrokenLinks as a
// checkbox list of proposed repairs, all checked by default. Broken links
// with no suggested fix are omitted - there's nothing to select for those.
func RenderBrokenLinksHTML(broken []files.BrokenLink) string {
	repairable := make([]files.BrokenLink, 0, len(broken))
	for _, bl := range broken {
		if bl.Suggested != "" {
			repairable = append(repairable, bl)
		}
	}

	var html strings.Builder
	html.WriteString(`<div id="component-broken-links">`)

	if len(repairable) == 0 {
		fmt.Fprintf(&html, `<p class="no-items">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no repairable broken links found"))
		html.WriteString(`</div>`)
		return html.String()
	}

	html.WriteString(`<table class="broken-links-table"><thead><tr><th><input type="checkbox" checked onclick="toggleAllBrokenLinks(this)"></th>`)
	fmt.Fprintf(&html, `<th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "broken link"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "suggested fix"))

	for _, bl := range repairable {
		value := fmt.Sprintf("%s|%s|%s", bl.SourceFile, bl.Target, bl.Suggested)
		fmt.Fprintf(&html, `<tr><td><input type="checkbox" name="repair" value="%s" checked></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			value, bl.SourceFile, bl.Target, brokenLinkSuggestedCell(bl.Suggested))
	}

	html.WriteString(`</tbody></table>`)
	fmt.Fprintf(&html, `<button type="button" class="btn-danger" onclick="repairBrokenLinks()">%s</button>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "Repair Selected"))
	html.WriteString(`</div>`)
	return html.String()
}

// brokenLinkSuggestedCell renders the suggested-fix path, with a thumbnail
// preview when the suggestion is an image, so the fix can be eyeballed before applying.
func brokenLinkSuggestedCell(suggested string) string {
	if !strings.HasPrefix(suggested, "media/") {
		return suggested
	}
	relativePath := strings.TrimPrefix(suggested, "media/")
	if !files.IsImageFile(strings.ToLower(filepath.Ext(relativePath))) {
		return suggested
	}
	return fmt.Sprintf(`<img src="/media/%s" alt="%s" class="media-compact-thumb" loading="lazy"> %s`,
		relativePath, filepath.Base(relativePath), suggested)
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
		createdat := configmanager.FormatDateTimeSeconds(m.CreatedAt)
		lastedited := configmanager.FormatDateTimeSeconds(m.LastEdited)
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

// -----------------------------------------------------------------------------------------
// -------------------------------- sidebar inline edit ------------------------------------
// -----------------------------------------------------------------------------------------

// renderSidebarEditBtn renders the small pencil edit button
func renderSidebarEditBtn(filePath, field string) string {
	return fmt.Sprintf(`<button class="meta-edit-btn" title="%s"
		hx-get="/api/metadata/inline-edit?field=%s&filepath=%s"
		hx-swap="outerHTML" hx-target="closest .meta-inline-wrap">
		<i class="fa fa-pen"></i></button>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "edit"),
		field, filePath)
}

// renderSidebarCancelBtn renders the cancel/stop button shown during editing
func renderSidebarCancelBtn(filePath, field string) string {
	return fmt.Sprintf(`<button class="meta-edit-btn meta-edit-btn--cancel" title="%s"
		hx-get="/api/metadata/inline-display?field=%s&filepath=%s"
		hx-swap="outerHTML" hx-target="closest .meta-inline-wrap">
		<i class="fa fa-xmark"></i></button>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		field, filePath)
}

// RenderSidebarFieldDisplay renders the read-only display row for an editable sidebar field.
// Returned HTML is the full .meta-inline-wrap so hx-swap="outerHTML" replaces it cleanly.
func RenderSidebarFieldDisplay(field, filePath string, metadata *files.Metadata) string {
	var value string
	switch field {
	case "tags":
		if metadata != nil {
			value = RenderMetadataLinksHTML(metadata.Tags, "tags")
		} else {
			value = `<span class="meta-empty">-</span>`
		}
	case "parents":
		if metadata != nil && len(metadata.Parents) > 0 {
			value = RenderMetadataLinksHTML(metadata.Parents, "")
		} else {
			value = `<span class="meta-empty">-</span>`
		}
	case "editor":
		if metadata != nil {
			value = RenderMetadataLinkHTML(string(metadata.Editor), "editor")
		} else {
			value = `<span class="meta-empty">-</span>`
		}
	case "path":
		if metadata != nil {
			value = fmt.Sprintf(`<span class="path">%s</span>`, metadata.Path)
		} else {
			value = `<span class="meta-empty">-</span>`
		}
	}
	return fmt.Sprintf(`<div class="meta-inline-wrap">
	<span class="meta-inline-display">%s</span>
	%s
</div>`, value, renderSidebarEditBtn(filePath, field))
}

// RenderSidebarFieldEdit renders the inline edit widget for a sidebar field.
// Returned HTML is the full .meta-inline-wrap so hx-swap="outerHTML" replaces it cleanly.
// After any successful save the wrap auto-swaps back to display mode.
func RenderSidebarFieldEdit(field, filePath string, metadata *files.Metadata) string {
	var input string
	switch field {
	case "tags":
		tagsStr := ""
		if metadata != nil {
			tagsStr = strings.Join(metadata.Tags, ", ")
		}
		input = GenerateTagChipsInputWithSave("sidebar-tags", "tags", tagsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
			"/api/metadata/tags?format=options", filePath, "/api/metadata/tags")
	case "parents":
		parentsStr := ""
		if metadata != nil {
			parentsStr = strings.Join(metadata.Parents, ", ")
		}
		input = GenerateTagChipsInputWithSave("sidebar-parents", "parents", parentsStr,
			translation.SprintfForRequest(configmanager.GetLanguage(), "add parent files"),
			"/api/files/list?format=options", filePath, "/api/metadata/parents")
	case "editor":
		editor := ""
		if metadata != nil {
			editor = string(metadata.Editor)
		}
		// use a select so only valid editor types can be chosen
		var opts strings.Builder
		for _, et := range files.AllEditorTypes() {
			sel := ""
			if string(et) == editor {
				sel = ` selected`
			}
			fmt.Fprintf(&opts, `<option value="%s"%s>%s</option>`, et, sel, et)
		}
		input = fmt.Sprintf(`<select id="sidebar-editor" name="editor" class="form-input"
			hx-post="/api/metadata/editor" hx-vals='{"filepath": "%s"}' hx-trigger="change" hx-swap="none">%s</select>`,
			filePath, opts.String())
	case "path":
		path := filePath
		if metadata != nil {
			path = metadata.Path
		}
		input = GenerateInputWithSaveOnBlur("sidebar-path", "newpath", path,
			translation.SprintfForRequest(configmanager.GetLanguage(), "enter file path"),
			filePath, "/api/metadata/path")
	}
	displayURL := fmt.Sprintf("/api/metadata/inline-display?field=%s&filepath=%s", field, filePath)
	return fmt.Sprintf(`<div class="meta-inline-wrap meta-inline-wrap--editing"
	hx-on:htmx:after-request="if(event.detail.successful && event.detail.requestConfig.verb==='post') htmx.ajax('GET','%s',{target:this,swap:'outerHTML'})">
	<div class="meta-inline-editor">%s%s</div>
</div>`, displayURL, renderSidebarCancelBtn(filePath, field), input)
}
