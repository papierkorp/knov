// Package utils provides utility functions
package utils

import (
	"fmt"
	"os"
	"path/filepath"
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

	// don't add .md extension to media files or files with existing extensions
	if strings.HasPrefix(cleanLink, "media/") ||
		strings.HasSuffix(cleanLink, ".md") ||
		strings.HasSuffix(cleanLink, ".txt") ||
		strings.Contains(filepath.Base(cleanLink), ".") {
		return cleanLink
	}

	// only add .md if no extension is present
	cleanLink = cleanLink + ".md"

	return cleanLink
}

// SanitizeFilename creates a sanitized filename with configurable options
func SanitizeFilename(input string, maxLength int, preserveExtensions bool, extractFromContent bool) string {
	if extractFromContent && input != "" {
		// extract filename from markdown content
		lines := strings.Split(input, "\n")
		var firstLine string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				firstLine = line
				break
			}
		}

		if firstLine != "" {
			// remove markdown syntax
			firstLine = strings.TrimSpace(firstLine)
			firstLine = strings.TrimLeft(firstLine, "#")
			firstLine = strings.TrimSpace(firstLine)
			firstLine = strings.ReplaceAll(firstLine, "**", "")
			firstLine = strings.ReplaceAll(firstLine, "*", "")
			firstLine = strings.ReplaceAll(firstLine, "__", "")
			firstLine = strings.ReplaceAll(firstLine, "_", "")
			input = firstLine
		}
	}

	// fallback to timestamp if input is empty
	if input == "" {
		return time.Now().Format("2006-01-02-150405")
	}

	// sanitize for filesystem - keep only safe characters
	var sanitized strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ' ' {
			sanitized.WriteRune(r)
		} else if r == '.' && preserveExtensions {
			// preserve dots for file extensions
			sanitized.WriteRune(r)
		}
	}

	result := sanitized.String()
	result = strings.TrimSpace(result)

	// replace spaces with hyphens
	result = strings.ReplaceAll(result, " ", "-")
	// remove multiple consecutive hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	// trim leading/trailing hyphens
	result = strings.Trim(result, "-")

	if result == "" {
		return time.Now().Format("2006-01-02-150405")
	}

	// apply length limit
	if maxLength > 0 && len(result) > maxLength {
		if preserveExtensions {
			// preserve extension if possible
			lastDot := strings.LastIndex(result, ".")
			if lastDot > 0 && lastDot < len(result)-1 {
				ext := result[lastDot:]
				name := result[:lastDot]
				maxNameLength := maxLength - len(ext)
				if maxNameLength > 0 {
					result = name[:maxNameLength] + ext
				} else {
					result = result[:maxLength]
				}
			} else {
				result = result[:maxLength]
			}
		} else {
			result = result[:maxLength]
			result = strings.TrimSuffix(result, "-")
		}
	}

	return result
}

// Normalize standardizes string input by trimming whitespace and converting to lowercase
func Normalize(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

// StripPathPrefix removes a specified prefix from a path to prevent duplication
// Example: StripPathPrefix("docs/ai.md", "docs/") -> "ai.md"
// Example: StripPathPrefix("media/images/photo.jpg", "media/") -> "images/photo.jpg"
func StripPathPrefix(path, prefix string) string {
	if path == "" {
		return path
	}

	// normalize path separators to forward slashes
	normalizedPath := strings.ReplaceAll(path, "\\", "/")

	// ensure prefix ends with slash for proper matching
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// strip the prefix if it exists
	if strings.HasPrefix(normalizedPath, prefix) {
		return strings.TrimPrefix(normalizedPath, prefix)
	}

	return normalizedPath
}

// ResolveFilenameConflicts checks for existing files and appends -1, -2, etc. to avoid conflicts
func ResolveFilenameConflicts(fullPath, relativePath string) string {
	// if file doesn't exist, return original path
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return relativePath
	}

	// extract filename parts
	dir := filepath.Dir(relativePath)
	filename := filepath.Base(relativePath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// try appending numbers until we find a non-existing filename
	for i := 1; i < 1000; i++ { // reasonable limit
		newFilename := fmt.Sprintf("%s-%d%s", nameWithoutExt, i, ext)
		var newPath string
		if dir != "" && dir != "." {
			newPath = filepath.Join(dir, newFilename)
		} else {
			newPath = newFilename
		}

		// construct full path based on the same directory structure as fullPath
		baseDir := filepath.Dir(fullPath)
		fullNewPath := filepath.Join(baseDir, filepath.Base(newPath))
		if _, err := os.Stat(fullNewPath); os.IsNotExist(err) {
			return newPath
		}
	}

	// fallback: use timestamp if we couldn't resolve conflicts
	timestamp := SanitizeFilename("", 20, false, true) // this generates timestamp
	newFilename := fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext)
	if dir != "" && dir != "." {
		return filepath.Join(dir, newFilename)
	}
	return newFilename
}
