// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// GetLinkDisplayText returns filename, filepath, or title based on theme setting.
// It fetches metadata itself, so only use it where the caller doesn't already
// have the file's metadata loaded (e.g. a bare path from a links list). When
// iterating a []files.File whose .Metadata is already populated (as returned by
// GetAllFiles/GetAllFilesCached), call GetLinkDisplayTextWithMetadata instead to
// avoid a redundant metadata store lookup per file.
func GetLinkDisplayText(filePath string) string {
	// these modes never need the title, so skip the metadata lookup entirely
	if mode := linkDisplayMode(); mode == "filename" || mode == "filepath" {
		return renderLinkDisplayText(filePath, mode, nil)
	}

	metadata, err := files.MetaDataGet(filePath)
	if err != nil {
		metadata = nil
	}
	return GetLinkDisplayTextWithMetadata(filePath, metadata)
}

// GetLinkDisplayTextWithMetadata is identical to GetLinkDisplayText but takes
// already-loaded metadata instead of fetching it, avoiding a metadata store
// lookup per file when rendering a tree/list/search result that already has it.
func GetLinkDisplayTextWithMetadata(filePath string, metadata *files.Metadata) string {
	return renderLinkDisplayText(filePath, linkDisplayMode(), metadata)
}

// linkDisplayMode reads the current theme's configured link display mode.
func linkDisplayMode() string {
	currentTheme := configmanager.GetTheme()
	linkDisplayMode := configmanager.GetThemeSetting(currentTheme, "linkDisplayMode")

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
	return displayMode
}

func renderLinkDisplayText(filePath string, displayMode string, metadata *files.Metadata) string {
	// get the components we might need
	filename := filepath.Base(filePath)

	switch displayMode {
	case "filename":
		return filename
	case "filepath":
		return filePath
	}

	var title string
	if metadata != nil && metadata.Title != "" {
		title = metadata.Title
	}

	switch displayMode {
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
	return fmt.Sprintf(`<div class="connection-empty">%s</div>`, message)
}

// RenderLinksList renders a list of file links (non-media) as HTML with configurable display text
func RenderLinksList(links []string, _ bool) string {
	if len(links) == 0 {
		return ""
	}

	var html strings.Builder
	for _, link := range links {
		if pathutils.IsMedia(link) {
			continue
		}
		rel := pathutils.ToRelative(link)
		url := pathutils.ToFileURL(rel)
		displayText := GetLinkDisplayText(rel)
		html.WriteString(fmt.Sprintf(`<a href="%s" title="%s" class="connection-link">%s</a>`, url, rel, displayText))
	}
	return html.String()
}

// RenderMediaLinks renders outbound media links as HTML
func RenderMediaLinks(links []string) string {
	if len(links) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no media files"))
	}

	var html strings.Builder
	hasMedia := false
	for _, link := range links {
		if !pathutils.IsMedia(link) {
			continue
		}
		hasMedia = true
		rel := pathutils.ToRelative(link)
		url := pathutils.ToMediaURL(rel)
		html.WriteString(fmt.Sprintf(`<a href="%s" title="%s" class="connection-link">%s</a>`, url, rel, filepath.Base(rel)))
	}
	if !hasMedia {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no media files"))
	}
	return html.String()
}

// RenderParentLinks renders parent links or no parents message
func RenderParentLinks(parents []string) string {
	if len(parents) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no parents"))
	}
	return RenderLinksList(parents, false)
}

// RenderAncestorLinks renders ancestor links or no ancestors message
func RenderAncestorLinks(ancestors []string) string {
	if len(ancestors) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no ancestors"))
	}
	return RenderLinksList(ancestors, false)
}

// RenderKidsLinks renders children links or no children message
func RenderKidsLinks(kids []string) string {
	if len(kids) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no children"))
	}
	return RenderLinksList(kids, false)
}

// RenderUsedLinks renders used/outbound links (non-media) or no outbound links message
func RenderUsedLinks(usedLinks []string) string {
	if len(usedLinks) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no outbound links"))
	}
	return RenderLinksList(usedLinks, false)
}

// RenderLinksToHere renders inbound links or no inbound links message
func RenderLinksToHere(linksToHere []string) string {
	if len(linksToHere) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no inbound links"))
	}
	return RenderLinksList(linksToHere, false)
}

// RenderRelatedFiles renders related files links or a fallback message
func RenderRelatedFiles(paths []string) string {
	if len(paths) == 0 {
		return RenderNoLinksMessage(translation.SprintfForRequest(configmanager.GetLanguage(), "no related files found"))
	}
	return RenderLinksList(paths, false)
}

// RenderConflictBanner renders a prominent warning banner above the file content,
// or empty string if no conflict exists (outerHTML swap removes the placeholder).
func RenderConflictBanner(originalFilePath string, conflictFile string) string {
	if conflictFile == "" {
		return ""
	}
	conflictRelPath := pathutils.ToRelative(conflictFile)
	display := filepath.Base(conflictRelPath)
	diffURL := "/api/links/conflicts/diff?filepath=" + url.QueryEscape(originalFilePath) + "&conflict=" + url.QueryEscape(conflictFile)
	showText := translation.SprintfForRequest(configmanager.GetLanguage(), "diff")
	hideText := translation.SprintfForRequest(configmanager.GetLanguage(), "hide diff")

	var html strings.Builder
	html.WriteString(`<div class="conflict-banner" id="component-conflict-banner">`)
	fmt.Fprintf(&html, `<span class="conflict-banner-icon"><i class="fa fa-triangle-exclamation"></i></span>`)
	fmt.Fprintf(&html, `<span class="conflict-banner-text">%s</span> `,
		translation.SprintfForRequest(configmanager.GetLanguage(), "this file has an unresolved conflict:"))
	fmt.Fprintf(&html, `<a href="/files/%s" class="conflict-banner-files">%s</a>`, conflictRelPath, display)
	fmt.Fprintf(&html, ` &mdash; <button class="conflict-diff-link" data-show="%s" data-hide="%s" onclick="toggleConflictDiff(this,'conflict-diff-banner','%s')">%s</button>`,
		showText, hideText, diffURL, showText)
	html.WriteString(`<div id="conflict-diff-banner" class="conflict-diff-container"></div>`)
	fmt.Fprintf(&html, `<button class="conflict-banner-dismiss" onclick="this.closest('.conflict-banner').remove()">%s</button>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "dismiss"))
	html.WriteString(`</div>`)
	return html.String()
}

// RenderConflictOfBanner renders a banner on the .conflict.md file itself,
// showing a diff against the original file it was copied from.
// Returns empty string if this file is not a conflict copy.
func RenderConflictOfBanner(conflictFilePath string, originalFilePath string) string {
	if originalFilePath == "" {
		return ""
	}
	origRelPath := pathutils.ToRelative(originalFilePath)
	origDisplay := filepath.Base(origRelPath)
	diffURL := "/api/links/conflicts/diff?filepath=" + url.QueryEscape(originalFilePath) + "&conflict=" + url.QueryEscape(conflictFilePath)
	showText := translation.SprintfForRequest(configmanager.GetLanguage(), "diff")
	hideText := translation.SprintfForRequest(configmanager.GetLanguage(), "hide diff")

	var html strings.Builder
	html.WriteString(`<div class="conflict-banner" id="component-conflict-of-banner">`)
	fmt.Fprintf(&html, `<span class="conflict-banner-icon"><i class="fa fa-triangle-exclamation"></i></span>`)
	fmt.Fprintf(&html, `<span class="conflict-banner-text">%s</span>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "this is a conflict copy of"))
	fmt.Fprintf(&html, ` <a href="/files/%s" class="conflict-banner-files">%s</a>`, origRelPath, origDisplay)
	fmt.Fprintf(&html, ` &mdash; <button class="conflict-diff-link" data-show="%s" data-hide="%s" onclick="toggleConflictDiff(this,'conflict-of-diff','%s')">%s</button>`,
		showText, hideText, diffURL, showText)
	html.WriteString(`<div id="conflict-of-diff" class="conflict-diff-container"></div>`)
	html.WriteString(`</div>`)
	return html.String()
}
