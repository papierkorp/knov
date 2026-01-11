// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/translation"
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

	// get the components we might need
	filename := filepath.Base(filePath)
	var title string
	metadata, err := files.MetaDataGet(filePath)
	if err == nil && metadata != nil && metadata.Title != "" {
		title = metadata.Title
	}

	switch displayMode {
	case "filename":
		return filename
	case "filepath":
		return filePath
	case "title":
		if title != "" {
			return title
		}
		// fallback to filename if no title available
		return filename
	case "title-filepath":
		if title != "" {
			return fmt.Sprintf(`%s <small>(%s)</small>`, title, filePath)
		}
		// fallback to filename with filepath if no title available
		return fmt.Sprintf(`%s <small>(%s)</small>`, filename, filePath)
	case "title-filename":
		if title != "" {
			return fmt.Sprintf(`%s <small>(%s)</small>`, title, filename)
		}
		// fallback to just filename if no title available
		return filename
	case "filename-title":
		if title != "" {
			return fmt.Sprintf(`%s <small>(%s)</small>`, filename, title)
		}
		// fallback to just filename if no title available
		return filename
	case "filepath-title":
		if title != "" {
			return fmt.Sprintf(`%s <small>(%s)</small>`, filePath, title)
		}
		// fallback to just filepath if no title available
		return filePath
	default:
		// fallback to filename for unknown modes
		return filename
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
		linkPath := contentStorage.ToRelativePath(link)
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
