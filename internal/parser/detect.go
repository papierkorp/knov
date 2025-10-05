// Package parser handles parsing of various file formats
package parser

import (
	"path/filepath"
	"strings"
)

// DetectFileType determines the file type based on extension and content
func DetectFileType(filePath string, content string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	// markdown files
	if ext == ".md" || ext == ".markdown" {
		return "markdown"
	}

	// explicit dokuwiki extension
	if ext == ".dokuwiki" {
		return "dokuwiki"
	}

	// .txt files need content inspection
	if ext == ".txt" {
		// check first line for dokuwiki header syntax
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			// dokuwiki uses ====== for headers
			if strings.HasPrefix(firstLine, "======") || strings.HasPrefix(firstLine, "=====") {
				return "dokuwiki"
			}
		}
		// default .txt to plain text
		return "plaintext"
	}

	// unknown types
	return "plaintext"
}
