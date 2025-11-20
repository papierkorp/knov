// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/utils"
)

// RenderWidget renders a widget based on its type and configuration
func RenderWidget(widgetType dashboard.WidgetType, config dashboard.WidgetConfig) (string, error) {
	switch widgetType {
	case dashboard.WidgetTypeFilter:
		// convert dashboard FilterConfig to filter.Config
		if config.Filter == nil {
			return "", fmt.Errorf("filter config is required")
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
		return "", fmt.Errorf("unknown widget type: %s", widgetType)
	}
}

func renderFileContentWidget(config *dashboard.FileContentConfig) (string, error) {
	if config == nil || config.FilePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	fullPath := utils.ToFullPath(config.FilePath)
	content, err := files.GetFileContent(fullPath)
	if err != nil {
		logging.LogError("failed to get file content: %v", err)
		return "", err
	}

	return string(content.HTML), nil
}

func renderStaticWidget(config *dashboard.StaticConfig) (string, error) {
	if config == nil || config.Content == "" {
		return "", fmt.Errorf("static content is required")
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
		return "", fmt.Errorf("filter config is required")
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
	html.WriteString(`<h4>filter</h4>`)
	html.WriteString(`<form id="filter-form" hx-post="/api/filter" hx-target="#filter-results">`)

	// controls row
	html.WriteString(`<div class="filter-controls">`)
	html.WriteString(`<button type="submit" class="btn-primary">apply filter</button>`)
	html.WriteString(`<select name="logic" class="form-select">`)
	html.WriteString(`<option value="and">and</option>`)
	html.WriteString(`<option value="or">or</option>`)
	html.WriteString(`</select>`)
	html.WriteString(`<button type="button" hx-get="/api/filter/criteria-row" hx-target="#filter-criteria-container" hx-swap="beforeend" class="btn-secondary">add filter</button>`)
	html.WriteString(`</div>`)

	// criteria container
	html.WriteString(`<div id="filter-criteria-container" class="filter-criteria-container">`)
	html.WriteString(RenderFilterCriteriaRow(0, nil))
	html.WriteString(`</div>`)

	// results container
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-placeholder">filtered results will appear here</p>`)
	html.WriteString(`</div>`)

	html.WriteString(`</form>`)
	html.WriteString(`</div>`)

	return html.String(), nil
}

// RenderFilterWidgetConfig renders widget-specific configuration form for filter widgets
func RenderFilterWidgetConfig(index int, config *dashboard.WidgetConfig) string {
	var html strings.Builder

	html.WriteString(`<div class="config-form">`)
	html.WriteString(`<h5>filter configuration</h5>`)

	// display & limits section
	html.WriteString(`<div class="config-section">`)
	html.WriteString(`<h6>display & limits</h6>`)

	// display type
	html.WriteString(`<div class="config-row">`)
	html.WriteString(`<label>display:</label>`)
	html.WriteString(fmt.Sprintf(`<select name="widgets[%d][config][filter][display]" class="form-select">`, index))

	selectedDisplay := "list"
	if config != nil && config.Filter != nil {
		selectedDisplay = config.Filter.Display
	}

	html.WriteString(fmt.Sprintf(`<option value="list" %s>list</option>`, ternary(selectedDisplay == "list", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="cards" %s>cards</option>`, ternary(selectedDisplay == "cards", "selected", "")))
	html.WriteString(fmt.Sprintf(`<option value="dropdown" %s>dropdown</option>`, ternary(selectedDisplay == "dropdown", "selected", "")))
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	// limit
	html.WriteString(`<div class="config-row">`)
	html.WriteString(`<label>limit:</label>`)
	limitValue := "10"
	if config != nil && config.Filter != nil && config.Filter.Limit > 0 {
		limitValue = fmt.Sprintf("%d", config.Filter.Limit)
	}
	html.WriteString(fmt.Sprintf(`<input type="number" name="widgets[%d][config][filter][limit]" value="%s" min="1" class="form-input"/>`, index, limitValue))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	return html.String()
}
