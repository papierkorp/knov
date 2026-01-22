// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"errors"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/translation"
	"knov/internal/utils"
)

// RenderWidget renders a widget based on its type and configuration
func RenderWidget(widgetType dashboard.WidgetType, config dashboard.WidgetConfig) (string, error) {
	switch widgetType {
	case dashboard.WidgetTypeFilter:
		// convert dashboard FilterConfig to filter.Config
		if config.Filter == nil {
			return "", errors.New(translation.SprintfForRequest(configmanager.GetLanguage(), "filter config is required"))
		}
		filterConfig := &filter.Config{
			Criteria: config.Filter.Criteria,
			Logic:    config.Filter.Logic,
			Display:  config.Filter.Display,
			Limit:    config.Filter.Limit,
		}
		return renderFilterWidget(filterConfig)
	case dashboard.WidgetTypeFilterForm:
		return renderFilterFormWidget()
	case dashboard.WidgetTypeFileContent:
		return renderFileContentWidget(config.FileContent)
	case dashboard.WidgetTypeStatic:
		return renderStaticWidget(config.Static)
	case dashboard.WidgetTypeTags:
		return renderTagsWidget()
	case dashboard.WidgetTypeCollections:
		return renderCollectionsWidget()
	case dashboard.WidgetTypeFolders:
		return renderFoldersWidget()
	case dashboard.WidgetTypeParaProjects:
		return renderParaProjectsWidget()
	case dashboard.WidgetTypeParaAreas:
		return renderParaAreasWidget()
	case dashboard.WidgetTypeParaResources:
		return renderParaResourcesWidget()
	case dashboard.WidgetTypeParaArchive:
		return renderParaArchiveWidget()
	default:
		msg := translation.SprintfForRequest(configmanager.GetLanguage(), "unknown widget type: %s", widgetType)
		return "", errors.New(msg)
	}
}

func renderFileContentWidget(config *dashboard.FileContentConfig) (string, error) {
	if config == nil || config.FilePath == "" {
		return "", errors.New(translation.SprintfForRequest(configmanager.GetLanguage(), "file path is required"))
	}

	fullPath := pathutils.ToDocsPath(config.FilePath)
	content, err := files.GetFileContent(fullPath)
	if err != nil {
		logging.LogError("failed to get file content: %v", err)
		return "", err
	}

	return string(content.HTML), nil
}

func renderStaticWidget(config *dashboard.StaticConfig) (string, error) {
	if config == nil || config.Content == "" {
		return "", errors.New(translation.SprintfForRequest(configmanager.GetLanguage(), "static content is required"))
	}

	switch config.Format {
	case "html":
		return config.Content, nil
	case "markdown":
		// could add markdown processing here if needed
		return fmt.Sprintf("<div class=\"markdown-content\">%s</div>", config.Content), nil
	default:
		return fmt.Sprintf("<pre>%s</pre>", config.Content), nil
	}
}

func renderTagsWidget() (string, error) {
	tagCount, err := files.GetAllTags()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(tagCount, "/browse/tags"), nil
}

func renderCollectionsWidget() (string, error) {
	collectionCount, err := files.GetAllCollections()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(collectionCount, "/browse/collection"), nil
}

func renderFoldersWidget() (string, error) {
	folderCount, err := files.GetAllFolders()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(folderCount, "/browse/folders"), nil
}

func renderParaProjectsWidget() (string, error) {
	projectCount, err := files.GetAllPARAProjects()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(projectCount, "/browse/para_projects"), nil
}

func renderParaAreasWidget() (string, error) {
	areaCount, err := files.GetAllPARAreas()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(areaCount, "/browse/para_areas"), nil
}

func renderParaResourcesWidget() (string, error) {
	resourceCount, err := files.GetAllPARAResources()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(resourceCount, "/browse/para_resources"), nil
}

func renderParaArchiveWidget() (string, error) {
	archiveCount, err := files.GetAllPARAArchive()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(archiveCount, "/browse/para_archive"), nil
}

// renderFilterWidget renders a filter widget for dashboards
func renderFilterWidget(config *filter.Config) (string, error) {
	if config == nil {
		return "", errors.New(translation.SprintfForRequest(configmanager.GetLanguage(), "filter config is required"))
	}

	result, err := filter.FilterFilesWithConfig(config)
	if err != nil {
		return "", err
	}

	return RenderFilterResult(result, config.Display), nil
}

// renderFilterFormWidget renders an interactive filter form widget
func renderFilterFormWidget() (string, error) {
	var html strings.Builder

	html.WriteString(`<div class="widget-filter-form">`)
	html.WriteString(`<h4>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter") + `</h4>`)
	html.WriteString(`<form id="filter-form" hx-post="/api/filter" hx-target="#filter-results">`)

	// controls row
	html.WriteString(`<div class="filter-controls">`)
	html.WriteString(`<button type="submit" class="btn-primary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "apply filter") + `</button>`)
	html.WriteString(`<select name="logic" class="form-select">`)
	html.WriteString(`<option value="and">` + translation.SprintfForRequest(configmanager.GetLanguage(), "and") + `</option>`)
	html.WriteString(`<option value="or">` + translation.SprintfForRequest(configmanager.GetLanguage(), "or") + `</option>`)
	html.WriteString(`</select>`)
	html.WriteString(`<button type="button" hx-post="/api/filter/add-criteria" hx-target="#filter-criteria-container" hx-swap="beforeend" class="btn-secondary">` + translation.SprintfForRequest(configmanager.GetLanguage(), "add filter") + `</button>`)
	html.WriteString(`</div>`)

	// criteria container
	html.WriteString(`<div id="filter-criteria-container" class="filter-criteria-container">`)
	html.WriteString(RenderFilterCriteriaRow(0, nil))
	html.WriteString(`</div>`)

	// results container
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-placeholder">` + translation.SprintfForRequest(configmanager.GetLanguage(), "filtered results will appear here") + `</p>`)
	html.WriteString(`</div>`)

	html.WriteString(`</form>`)
	html.WriteString(`</div>`)

	return html.String(), nil
}

// RenderFilterWidgetConfig renders widget-specific configuration form for filter widgets
func RenderFilterWidgetConfig(index int, config *dashboard.WidgetConfig) string {
	var html strings.Builder

	html.WriteString(`<div class="config-form">`)
	html.WriteString(`<h5>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter configuration") + `</h5>`)

	// filter criteria section
	html.WriteString(`<div class="config-section">`)
	html.WriteString(`<h6>` + translation.SprintfForRequest(configmanager.GetLanguage(), "filter criteria") + `</h6>`)

	// logic selection
	html.WriteString(`<div class="config-row">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "logic") + `:</label>`)
	html.WriteString(`<select name="logic" class="form-select">`)

	selectedLogic := "and"
	if config != nil && config.Filter != nil {
		selectedLogic = config.Filter.Logic
	}

	html.WriteString(fmt.Sprintf(`<option value="and" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "and")+`</option>`, utils.Ternary(selectedLogic == "and", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="or" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "or")+`</option>`, utils.Ternary(selectedLogic == "or", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/filter/add-criteria" hx-target="#filter-criteria-container-%d" hx-swap="beforeend" class="btn-secondary btn-small">%s</button>`, index, translation.SprintfForRequest(configmanager.GetLanguage(), "add criteria")))
	html.WriteString(`</div>`)

	html.WriteString(fmt.Sprintf(`<div id="filter-criteria-container-%d" class="filter-criteria-container">`, index))

	// render existing criteria or default first criteria
	if config != nil && config.Filter != nil && len(config.Filter.Criteria) > 0 {
		for i, criteria := range config.Filter.Criteria {
			html.WriteString(RenderFilterCriteriaRow(i, &criteria))
		}
	} else {
		html.WriteString(RenderFilterCriteriaRow(0, nil))
	}
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// preview section
	html.WriteString(`<div class="config-section">`)
	html.WriteString(`<h6>` + translation.SprintfForRequest(configmanager.GetLanguage(), "preview") + `</h6>`)
	html.WriteString(`<div class="config-row">`)
	html.WriteString(fmt.Sprintf(`<button type="button" hx-post="/api/filter" hx-include="#filter-criteria-container-%d" hx-target="#filter-preview-results-%d" hx-swap="innerHTML" class="btn-secondary">%s</button>`, index, index, translation.SprintfForRequest(configmanager.GetLanguage(), "view results")))
	html.WriteString(`</div>`)

	html.WriteString(fmt.Sprintf(`<div id="filter-preview-results-%d" class="filter-results">`, index))
	html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "configure filter above and click view results to preview") + `</p>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// display & limits section
	html.WriteString(`<div class="config-section">`)
	html.WriteString(`<h6>` + translation.SprintfForRequest(configmanager.GetLanguage(), "display & limits") + `</h6>`)

	// display type
	html.WriteString(`<div class="config-row">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "display") + `:</label>`)
	html.WriteString(`<select name="display" class="form-select">`)

	selectedDisplay := "list"
	if config != nil && config.Filter != nil {
		selectedDisplay = config.Filter.Display
	}

	html.WriteString(fmt.Sprintf(`<option value="list" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "list")+`</option>`, utils.Ternary(selectedDisplay == "list", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="cards" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "cards")+`</option>`, utils.Ternary(selectedDisplay == "cards", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="dropdown" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "dropdown")+`</option>`, utils.Ternary(selectedDisplay == "dropdown", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="content" %s>`+translation.SprintfForRequest(configmanager.GetLanguage(), "content")+`</option>`, utils.Ternary(selectedDisplay == "content", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// limit
	html.WriteString(`<div class="config-row">`)
	html.WriteString(`<label>` + translation.SprintfForRequest(configmanager.GetLanguage(), "limit") + `:</label>`)
	limitValue := "10"
	if config != nil && config.Filter != nil && config.Filter.Limit > 0 {
		limitValue = fmt.Sprintf("%d", config.Filter.Limit)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="limit" value="%s" min="1" class="form-input"/>`, limitValue))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}
