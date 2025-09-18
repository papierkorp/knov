// Package content handles content processing and transformations
package content

import (
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
)

// ProcessContent applies all content transformations
func ProcessContent(content string) string {
	content = ProcessLinks(content)
	return content
}

// ProcessLinks converts markdown-style links to HTML anchors
func ProcessLinks(content string) string {
	linkRegexes := configmanager.GetMetadataLinkRegex()

	for _, regexPattern := range linkRegexes {
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			logging.LogWarning("invalid regex pattern %s: %v", regexPattern, err)
			continue
		}

		content = re.ReplaceAllStringFunc(content, func(match string) string {
			return convertLinkToHTML(match, re)
		})
	}

	return content
}

// convertLinkToHTML converts a matched link to HTML anchor
func convertLinkToHTML(match string, re *regexp.Regexp) string {
	matches := re.FindStringSubmatch(match)
	if len(matches) < 2 {
		return match
	}

	linkText := matches[1]
	linkPath := linkText
	displayText := linkText

	// Handle wiki-style links with custom text: [[path|display]]
	if strings.Contains(linkText, "|") {
		parts := strings.SplitN(linkText, "|", 2)
		linkPath = parts[0]
		displayText = parts[1]
	}

	// Clean up the path
	linkPath = strings.TrimSpace(linkPath)
	displayText = strings.TrimSpace(displayText)

	// Remove .md extension from display text if present
	if strings.HasSuffix(displayText, ".md") {
		displayText = strings.TrimSuffix(displayText, ".md")
	}

	// Ensure .md extension for link path
	if !strings.HasSuffix(linkPath, ".md") {
		linkPath += ".md"
	}

	// Convert to HTML anchor
	return `<a href="/files/` + linkPath + `">` + displayText + `</a>`
}

// ExtractLinks extracts all links from content (used by metadata system)
func ExtractLinks(content string) []string {
	linkRegexes := configmanager.GetMetadataLinkRegex()
	var links []string
	linkSet := make(map[string]bool)

	for _, regexPattern := range linkRegexes {
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			logging.LogWarning("invalid regex pattern %s: %v", regexPattern, err)
			continue
		}

		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				link := extractLinkPath(match[1])
				if link != "" && !linkSet[link] {
					linkSet[link] = true
					links = append(links, link)
				}
			}
		}
	}

	return links
}

// extractLinkPath cleans up a link path for metadata storage
func extractLinkPath(linkText string) string {
	// Handle wiki-style links with custom text: [[path|display]]
	if idx := strings.Index(linkText, "|"); idx != -1 {
		linkText = linkText[:idx]
	}

	// Clean up other formats
	if idx := strings.Index(linkText, "]]"); idx != -1 {
		linkText = linkText[:idx]
	}
	if idx := strings.Index(linkText, "}}"); idx != -1 {
		linkText = linkText[:idx]
	}

	// Remove relative path prefix
	if strings.HasPrefix(linkText, "../") {
		linkText = strings.TrimPrefix(linkText, "../")
	}

	linkText = strings.TrimSpace(linkText)

	// Validate link
	if strings.Contains(linkText, "\n") || len(linkText) > 100 || !strings.HasSuffix(linkText, ".md") {
		return ""
	}

	return linkText
}
