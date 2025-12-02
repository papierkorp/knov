// Package utils provides utility functions
package utils

import (
	"regexp"
	"strings"
)

func CleanseID(input string) string {
	result := strings.ToLower(input)

	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	result = reg.ReplaceAllString(result, "_")

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	return result
}

// CleanLink normalizes a link by removing anchors, aliases, and adding extensions
func CleanLink(link string) string {
	cleanLink := strings.Split(link, "#")[0]
	cleanLink = strings.Split(cleanLink, "|")[0]
	cleanLink = strings.TrimSpace(cleanLink)

	// only add .md if no extension is present (preserve .txt, .md, etc.)
	if !strings.HasSuffix(cleanLink, ".md") && !strings.HasSuffix(cleanLink, ".txt") {
		cleanLink = cleanLink + ".md"
	}

	return cleanLink
}
