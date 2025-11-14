// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderMetadataForm renders the complete metadata form for a file
func RenderMetadataForm(filePath string) (string, error) {
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
	html.WriteString(GenerateDatalistInputWithSave("meta-name", "name", name,
		translation.SprintfForRequest(configmanager.GetLanguage(), "enter name"),
		"", filePath, "/api/metadata/name"))

	// priority field
	priority := ""
	if metadata != nil {
		priority = string(metadata.Priority)
	}
	html.WriteString(GenerateDatalistInputWithSave("meta-priority", "priority", priority,
		translation.SprintfForRequest(configmanager.GetLanguage(), "set priority (1-5)"),
		"/api/metadata/priority/all?format=options", filePath, "/api/metadata/priority"))

	// collection field
	collection := ""
	if metadata != nil {
		collection = metadata.Collection
	}
	html.WriteString(GenerateDatalistInputWithSave("meta-collection", "collection", collection,
		translation.SprintfForRequest(configmanager.GetLanguage(), "assign to collection"),
		"/api/metadata/collection/all?format=options", filePath, "/api/metadata/collection"))

	html.WriteString(`</div>`) // close basic form group

	// tags and folders section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "tags & folders") + `</h3>`)

	// tags field
	tagsStr := ""
	if metadata != nil && len(metadata.Tags) > 0 {
		tagsStr = strings.Join(metadata.Tags, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-tags", "tags", tagsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
		"/api/metadata/tags/all?format=options", filePath, "/api/metadata/tags"))

	// folders field
	foldersStr := ""
	if metadata != nil && len(metadata.Folders) > 0 {
		foldersStr = strings.Join(metadata.Folders, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-folders", "folders", foldersStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add folders"),
		"/api/metadata/folders/all?format=options", filePath, "/api/metadata/folders"))

	html.WriteString(`</div>`) // close tags form group

	// PARA section
	html.WriteString(`<div class="form-group">`)
	html.WriteString(`<h3>` + translation.SprintfForRequest(configmanager.GetLanguage(), "PARA method") + `</h3>`)

	// projects field
	projectsStr := ""
	if metadata != nil && len(metadata.PARA.Projects) > 0 {
		projectsStr = strings.Join(metadata.PARA.Projects, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-projects", "projects", projectsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add projects"),
		"/api/metadata/para/projects/all?format=options", filePath, "/api/metadata/para/projects"))

	// areas field
	areasStr := ""
	if metadata != nil && len(metadata.PARA.Areas) > 0 {
		areasStr = strings.Join(metadata.PARA.Areas, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-areas", "areas", areasStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add areas of responsibility"),
		"/api/metadata/para/areas/all?format=options", filePath, "/api/metadata/para/areas"))

	// resources field
	resourcesStr := ""
	if metadata != nil && len(metadata.PARA.Resources) > 0 {
		resourcesStr = strings.Join(metadata.PARA.Resources, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-resources", "resources", resourcesStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add resources"),
		"/api/metadata/para/resources/all?format=options", filePath, "/api/metadata/para/resources"))

	// archive field
	archiveStr := ""
	if metadata != nil && len(metadata.PARA.Archive) > 0 {
		archiveStr = strings.Join(metadata.PARA.Archive, ", ")
	}
	html.WriteString(GenerateTagChipsInputWithSave("meta-archive", "archive", archiveStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add archived items"),
		"/api/metadata/para/archive/all?format=options", filePath, "/api/metadata/para/archive"))

	html.WriteString(`</div>`)  // close para form group
	html.WriteString(`</form>`) // close metadata form

	return html.String(), nil
}
