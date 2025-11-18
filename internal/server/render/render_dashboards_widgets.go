// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"

	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/utils"
)

// RenderWidget renders a widget based on its type and configuration
func RenderWidget(widgetType dashboard.WidgetType, config dashboard.WidgetConfig) (string, error) {
	switch widgetType {
	case dashboard.WidgetTypeFilter:
		return renderFilterWidget(config.Filter)
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
