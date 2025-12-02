// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
	"knov/internal/utils"
)

// GetLinkDisplayText returns filename, filepath, or title based on theme setting
func GetLinkDisplayText(filePath string) string {
	// get current theme setting for link display mode
	currentTheme := configmanager.GetTheme()
	linkDisplayMode := configmanager.GetThemeSetting(currentTheme, "linkDisplayMode")

	// convert interface{} to string for comparison
	var displayMode string
	if linkDisplayMode != nil {
		if mode, ok := linkDisplayMode.(string); ok {
			displayMode = mode
		}
	}

	// default to filename if setting not found
	if displayMode == "" {
		displayMode = "filename"
	}

	switch displayMode {
	case "filename":
		return filepath.Base(filePath)
	case "filepath":
		return filePath
	case "title":
		metadata, err := files.MetaDataGet(filePath)
		if err == nil && metadata != nil && metadata.Title != "" {
			return metadata.Title
		}
		// fallback to filename if no title available
		return filepath.Base(filePath)
	default:
		// fallback to filename for unknown modes
		return filepath.Base(filePath)
	}
}

// RenderNoLinksMessage renders a "no links" message with appropriate class
func RenderNoLinksMessage(message string) string {
	return fmt.Sprintf(`<div class="component-no-links">%s</div>`, message)
}

// RenderLinksList renders a list of file links as HTML with configurable display text
func RenderLinksList(links []string) string {
	if len(links) == 0 {
		return ""
	}

	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, link := range links {
		linkPath := utils.ToRelativePath(link)
		displayText := GetLinkDisplayText(linkPath)
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s" title="%s">%s</a></li>`, linkPath, linkPath, displayText))
	}
	html.WriteString(`</ul>`)
	return html.String()
}

// RenderParentLinks renders parent links or no parents message
func RenderParentLinks(parents []string) string {
	if len(parents) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no parents"))
	}
	return RenderLinksList(parents)
}

// RenderAncestorLinks renders ancestor links or no ancestors message
func RenderAncestorLinks(ancestors []string) string {
	if len(ancestors) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no ancestors"))
	}
	return RenderLinksList(ancestors)
}

// RenderKidsLinks renders children links or no children message
func RenderKidsLinks(kids []string) string {
	if len(kids) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no children"))
	}
	return RenderLinksList(kids)
}

// RenderUsedLinks renders used/outbound links or no outbound links message
func RenderUsedLinks(usedLinks []string) string {
	if len(usedLinks) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no outbound links"))
	}
	return RenderLinksList(usedLinks)
}

// RenderLinksToHere renders inbound links or no inbound links message
func RenderLinksToHere(linksToHere []string) string {
	if len(linksToHere) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no inbound links"))
	}
	return RenderLinksList(linksToHere)
}
