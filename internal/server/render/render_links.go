// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/utils"
)

// RenderNoLinksMessage renders a "no links" message with appropriate class
func RenderNoLinksMessage(message string) string {
	return fmt.Sprintf(`<div class="component-no-links">%s</div>`, message)
}

// RenderLinksList renders a list of file links as HTML
func RenderLinksList(links []string) string {
	if len(links) == 0 {
		return ""
	}

	var html strings.Builder
	html.WriteString(`<ul class="component-link-list">`)
	for _, link := range links {
		linkPath := utils.ToRelativePath(link)
		filename := filepath.Base(linkPath)
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s" title="%s">%s</a></li>`, linkPath, linkPath, filename))
	}
	html.WriteString(`</ul>`)
	return html.String()
}

// RenderParentLinks renders parent links or no parents message
func RenderParentLinks(parents []string) string {
	if len(parents) == 0 {
		return RenderNoLinksMessage("no parents")
	}
	return RenderLinksList(parents)
}

// RenderAncestorLinks renders ancestor links or no ancestors message
func RenderAncestorLinks(ancestors []string) string {
	if len(ancestors) == 0 {
		return RenderNoLinksMessage("no ancestors")
	}
	return RenderLinksList(ancestors)
}

// RenderKidsLinks renders children links or no children message
func RenderKidsLinks(kids []string) string {
	if len(kids) == 0 {
		return RenderNoLinksMessage("no children")
	}
	return RenderLinksList(kids)
}

// RenderUsedLinks renders used/outbound links or no outbound links message
func RenderUsedLinks(usedLinks []string) string {
	if len(usedLinks) == 0 {
		return RenderNoLinksMessage("no outbound links")
	}
	return RenderLinksList(usedLinks)
}

// RenderLinksToHere renders inbound links or no inbound links message
func RenderLinksToHere(linksToHere []string) string {
	if len(linksToHere) == 0 {
		return RenderNoLinksMessage("no inbound links")
	}
	return RenderLinksList(linksToHere)
}
