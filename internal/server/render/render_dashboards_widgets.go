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
	"knov/internal/mapping"
	"knov/internal/pathutils"
	"knov/internal/translation"
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

	return fmt.Sprintf(`<article class="file-content">%s</article>`, content.HTML), nil
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

	return RenderBrowseHTML(map[string]int(tagCount), "/browse/"+mapping.DatabaseToURL("tags"), false, ""), nil
}

func renderCollectionsWidget() (string, error) {
	collectionCount, err := files.GetAllCollections()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(map[string]int(collectionCount), "/browse/collection", false, ""), nil
}

func renderFoldersWidget() (string, error) {
	folderCount, err := files.GetAllFolders()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(map[string]int(folderCount), "/browse/"+mapping.DatabaseToURL("folders"), false, ""), nil
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

// RenderFilterWidgetConfig renders widget-specific configuration form for filter widgets
func RenderFilterWidgetConfig(index int, config *dashboard.WidgetConfig) string {
	var fc *filter.Config
	if config != nil && config.Filter != nil {
		fc = &filter.Config{
			Criteria: config.Filter.Criteria,
			Logic:    config.Filter.Logic,
			Display:  config.Filter.Display,
			Limit:    config.Filter.Limit,
		}
	}

	var html strings.Builder
	html.WriteString(`<div class="config-form">`)
	html.WriteString(RenderFilterForm(FilterFormOpts{
		Context:     FilterFormContextDashboard,
		Config:      fc,
		WidgetIndex: index,
	}))
	// preview button — posts the widget config form to the filter API
	previewBtn := fmt.Sprintf(
		`<button type="button" class="btn-secondary" style="margin-bottom:8px;"
		 hx-post="/api/filters" hx-include="#widget-config-%d" hx-target="#filter-preview-results-%d">%s</button>`,
		index, index,
		translation.SprintfForRequest(configmanager.GetLanguage(), "preview results"))
	html.WriteString(previewBtn)
	html.WriteString(fmt.Sprintf(`<div id="filter-preview-results-%d" class="filter-results">`, index))
	html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "configure filter above and click view results to preview") + `</p>`)
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	return html.String()
}

// renderFilterFormWidget renders an interactive filter form widget
func renderFilterFormWidget() (string, error) {
	var html strings.Builder
	html.WriteString(`<div class="widget-filter-form">`)
	html.WriteString(RenderFilterForm(FilterFormOpts{
		Context: FilterFormContextApply,
	}))
	html.WriteString(`<div id="filter-results" class="filter-results">`)
	html.WriteString(`<p class="filter-placeholder">` + translation.SprintfForRequest(configmanager.GetLanguage(), "filtered results will appear here") + `</p>`)
	html.WriteString(`</div></div>`)
	return html.String(), nil
}
