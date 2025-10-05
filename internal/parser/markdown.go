// Package parser handles markdown parsing
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseMarkdown converts Markdown to HTML-ready format
func ParseMarkdown(content string) string {
	// process code blocks first (preserve them)
	content = processMarkdownCodeBlocks(content)

	// process links
	content = ProcessMarkdownLinks(content)

	return content
}

// processMarkdownCodeBlocks marks code blocks with language classes
func processMarkdownCodeBlocks(content string) string {
	// ```language ... ```
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)\n(.*?)```")

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		language := strings.TrimSpace(matches[1])
		if language == "" {
			language = "plaintext"
		}
		code := matches[2]

		// return with language class - markdown parser will wrap in pre
		return fmt.Sprintf("```%s\n%s```", language, code)
	})

	return content
}

// ProcessMarkdownLinks converts markdown-style links to HTML anchors
func ProcessMarkdownLinks(content string) string {
	// standard markdown links: [text](url)
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		text := strings.TrimSpace(matches[1])
		url := strings.TrimSpace(matches[2])

		// internal links (no protocol)
		if !strings.Contains(url, "://") && !strings.HasPrefix(url, "#") {
			// ensure .md extension for file links
			if !strings.HasSuffix(url, ".md") {
				url += ".md"
			}
			return `<a href="/files/` + url + `">` + text + `</a>`
		}

		// external links
		return `<a href="` + url + `">` + text + `</a>`
	})

	return content
}

// ExtractMarkdownLinks extracts all links from markdown content
func ExtractMarkdownLinks(content string) []string {
	var links []string
	linkSet := make(map[string]bool)

	// [text](url) pattern
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 2 {
			url := strings.TrimSpace(match[2])
			// only internal file links
			if !strings.Contains(url, "://") && !strings.HasPrefix(url, "#") {
				if !linkSet[url] {
					linkSet[url] = true
					links = append(links, url)
				}
			}
		}
	}

	return links
}
