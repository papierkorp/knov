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

	// basic metadata section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "basic metadata") + `</h3>`)

	// name field
	name := ""
	if metadata != nil {
		name = metadata.Name
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-name">` + translation.SprintfForRequest(configmanager.GetLanguage(), "name") + `</label>`)
	html.WriteString(GenerateDatalistInputWithSave("meta-name", "name", name,
		translation.SprintfForRequest(configmanager.GetLanguage(), "enter name"),
		"", filePath, "/api/metadata/name"))
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

	html.WriteString(`</div>`) // close basic form group

	// tags and folders section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "tags & folders") + `</h3>`)

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
		translation.SprintfForRequest(configmanager.GetLanguage(), "add folders"),
		"/api/metadata/folders?format=options", filePath, "/api/metadata/folders"))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`) // close tags form group

	// PARA section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "PARA method") + `</h3>`)

	// projects field
	projectsStr := ""
	if metadata != nil && len(metadata.PARA.Projects) > 0 {
		projectsStr = strings.Join(metadata.PARA.Projects, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-projects">` + translation.SprintfForRequest(configmanager.GetLanguage(), "projects") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-projects", "projects", projectsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add projects"),
		"/api/metadata/para/projects?format=options", filePath, "/api/metadata/para/projects"))
	html.WriteString(`</div>`)

	// areas field
	areasStr := ""
	if metadata != nil && len(metadata.PARA.Areas) > 0 {
		areasStr = strings.Join(metadata.PARA.Areas, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-areas">` + translation.SprintfForRequest(configmanager.GetLanguage(), "areas") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-areas", "areas", areasStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add areas of responsibility"),
		"/api/metadata/para/areas?format=options", filePath, "/api/metadata/para/areas"))
	html.WriteString(`</div>`)

	// resources field
	resourcesStr := ""
	if metadata != nil && len(metadata.PARA.Resources) > 0 {
		resourcesStr = strings.Join(metadata.PARA.Resources, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-resources">` + translation.SprintfForRequest(configmanager.GetLanguage(), "resources") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-resources", "resources", resourcesStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add resources"),
		"/api/metadata/para/resources?format=options", filePath, "/api/metadata/para/resources"))
	html.WriteString(`</div>`)

	// archive field
	archiveStr := ""
	if metadata != nil && len(metadata.PARA.Archive) > 0 {
		archiveStr = strings.Join(metadata.PARA.Archive, ", ")
	}
	html.WriteString(`<div class="form-field">`)
	html.WriteString(`<label for="meta-archive">` + translation.SprintfForRequest(configmanager.GetLanguage(), "archive") + `</label>`)
	html.WriteString(GenerateTagChipsInputWithSave("meta-archive", "archive", archiveStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add archived items"),
		"/api/metadata/para/archive?format=options", filePath, "/api/metadata/para/archive"))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)  // close para form group
	html.WriteString(`</form>`) // close metadata form

	return html.String(), nil
}

// RenderMetadataCSV generates CSV content for metadata export
func RenderMetadataCSV(metadata []*files.Metadata) string {
	var csv strings.Builder

	// header
	csv.WriteString("path,name,collection,filetype,status,priority,createdat,lastedited,tags,folders,para_projects,para_areas,para_resources,para_archive\n")

	for _, m := range metadata {
		if m == nil {
			continue
		}

		// escape csv values
		path := escapeCSV(m.Path)
		name := escapeCSV(m.Name)
		collection := escapeCSV(m.Collection)
		filetype := escapeCSV(string(m.FileType))
		status := escapeCSV(string(m.Status))
		priority := escapeCSV(string(m.Priority))
		createdat := m.CreatedAt.Format("2006-01-02 15:04:05")
		lastedited := m.LastEdited.Format("2006-01-02 15:04:05")
		tags := escapeCSV(strings.Join(m.Tags, ";"))
		folders := escapeCSV(strings.Join(m.Folders, ";"))
		projects := escapeCSV(strings.Join(m.PARA.Projects, ";"))
		areas := escapeCSV(strings.Join(m.PARA.Areas, ";"))
		resources := escapeCSV(strings.Join(m.PARA.Resources, ";"))
		archive := escapeCSV(strings.Join(m.PARA.Archive, ";"))

		csv.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			path, name, collection, filetype, status, priority, createdat, lastedited,
			tags, folders, projects, areas, resources, archive))
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
