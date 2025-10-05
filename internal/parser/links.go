// Package parser handles link extraction
package parser

import (
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
)

// ExtractLinks extracts all links from content based on file type
func ExtractLinks(content string, filePath string) []string {
	fileType := DetectFileType(filePath, content)

	switch fileType {
	case "markdown":
		return ExtractMarkdownLinks(content)
	case "dokuwiki":
		return ExtractDokuWikiLinks(content)
	default:
		// fallback to config-based regex patterns
		return extractLinksByRegex(content)
	}
}

// extractLinksByRegex uses config patterns for unknown file types
func extractLinksByRegex(content string) []string {
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
	if strings.Contains(linkText, "\n") || len(linkText) > 100 {
		return ""
	}

	return linkText
}
