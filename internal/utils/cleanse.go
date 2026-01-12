// Package utils provides utility functions
package utils

import (
	"regexp"
	"strings"
	"time"
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

// SanitizeFilename creates a sanitized filename from content, with optional length limit
func SanitizeFilename(content string, maxLength int) string {
	if content == "" {
		// fallback to timestamp if no content
		return time.Now().Format("2006-01-02-150405")
	}

	// split into lines and get first non-empty line
	lines := strings.Split(content, "\n")
	var firstLine string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			firstLine = line
			break
		}
	}

	if firstLine == "" {
		// fallback to timestamp if no meaningful content
		return time.Now().Format("2006-01-02-150405")
	}

	// remove markdown syntax
	firstLine = strings.TrimSpace(firstLine)
	// remove markdown headers
	firstLine = strings.TrimLeft(firstLine, "#")
	firstLine = strings.TrimSpace(firstLine)
	// remove markdown bold/italic
	firstLine = strings.ReplaceAll(firstLine, "**", "")
	firstLine = strings.ReplaceAll(firstLine, "*", "")
	firstLine = strings.ReplaceAll(firstLine, "__", "")
	firstLine = strings.ReplaceAll(firstLine, "_", "")

	// sanitize for filesystem - keep only alphanumeric, hyphens, underscores, and spaces
	var sanitized strings.Builder
	for _, r := range firstLine {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ' ' {
			sanitized.WriteRune(r)
		}
	}

	filename := sanitized.String()
	filename = strings.TrimSpace(filename)

	// replace spaces with hyphens
	filename = strings.ReplaceAll(filename, " ", "-")
	// remove multiple consecutive hyphens
	for strings.Contains(filename, "--") {
		filename = strings.ReplaceAll(filename, "--", "-")
	}
	// trim leading/trailing hyphens
	filename = strings.Trim(filename, "-")

	if filename == "" {
		// fallback to timestamp if sanitization resulted in empty string
		return time.Now().Format("2006-01-02-150405")
	}

	// truncate to specified length (default to 20 if maxLength <= 0)
	if maxLength <= 0 {
		maxLength = 20
	}

	if len(filename) > maxLength {
		filename = filename[:maxLength]
		// trim trailing hyphen if cut in the middle
		filename = strings.TrimSuffix(filename, "-")
	}

	// ensure it's not empty after truncation
	if filename == "" {
		return time.Now().Format("2006-01-02-150405")
	}

	return filename
}
