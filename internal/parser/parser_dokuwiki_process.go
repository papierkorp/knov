package parser

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// processDokuWikiSyntax applies unified detection and rendering for all syntax types
func (h *DokuwikiHandler) processDokuWikiSyntax(content string, outputFormat string) string {
	// Process in order of complexity to avoid conflicts

	// 0. Handle include sections first (needs outputFormat)
	content = h.convertIncludeSections(content, outputFormat)

	// 1. Handle media links (needs outputFormat)
	content = h.processMediaLinks(content, outputFormat)

	// 2. Tables first (so they don't get processed as other syntax)
	content = h.ProcessTables(content, outputFormat)

	// 3. Code blocks (so they don't get processed as other syntax)
	content = h.processCodeBlocks(content, outputFormat)

	// 4. Headers
	content = h.processHeaders(content, outputFormat)

	// 5. Text formatting
	content = h.processTextFormatting(content, outputFormat)

	// 6. Links
	content = h.processLinks(content, outputFormat)

	// 7. Lists (last, so other formatting inside lists works)
	content = h.processLists(content, outputFormat)

	return content
}

// processMediaLinks handles media link conversion for both HTML and Markdown
func (h *DokuwikiHandler) processMediaLinks(content string, outputFormat string) string {
	mediaRegex := regexp.MustCompile(`\{\{:([^}]+)\}\}`)
	content = mediaRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := mediaRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			mediaPath := strings.TrimSpace(matches[1])

			// convert dokuwiki namespace (colons) to filesystem path (slashes)
			mediaPath = strings.ReplaceAll(mediaPath, ":", "/")

			// create media URL
			mediaURL := fmt.Sprintf("/files/media/%s", mediaPath)

			if outputFormat == "html" {
				// for HTML: create img tag
				return fmt.Sprintf(`<img src="%s" alt="%s" />`, mediaURL, filepath.Base(mediaPath))
			} else {
				// for Markdown: create image syntax
				return fmt.Sprintf(`![%s](%s)`, filepath.Base(mediaPath), mediaURL)
			}
		}
		return match
	})

	return content
}

// processCodeBlocks handles all code block syntaxes
func (h *DokuwikiHandler) processCodeBlocks(content string, outputFormat string) string {
	patterns := []struct {
		regex string
		tag   string
	}{
		{`(?s)<sxh\s+([a-zA-Z0-9_-]+)>(.*?)</sxh>`, "sxh"},
		{`(?s)<codify\s+([a-zA-Z0-9_-]+)>(.*?)</codify>`, "codify"},
		{`(?s)<code\s+([a-zA-Z0-9_-]+)>(.*?)</code>`, "code"},
		{`(?s)<file\s+([a-zA-Z0-9_-]+)>(.*?)</file>`, "file"},
		{`(?s)<code>(.*?)</code>`, "simple-code"}, // no language
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern.regex)
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			matches := re.FindStringSubmatch(match)
			if len(matches) < 2 {
				return match
			}

			var element DokuWikiElement
			if pattern.tag == "simple-code" {
				element = DokuWikiElement{
					Type:     "code-block",
					Language: "",
					Content:  strings.TrimSpace(matches[1]),
				}
			} else if len(matches) >= 3 {
				element = DokuWikiElement{
					Type:     "code-block",
					Language: strings.TrimSpace(matches[1]),
					Content:  strings.TrimSpace(matches[2]),
				}
			}

			return h.renderElement(element, outputFormat)
		})
	}

	return content
}

// processHeaders handles header conversion for both HTML and Markdown
func (h *DokuwikiHandler) processHeaders(content string, outputFormat string) string {
	patterns := []struct {
		regex string
		level int
	}{
		{`======\s*(.+?)\s*======`, 1},
		{`=====\s*(.+?)\s*=====`, 2},
		{`====\s*(.+?)\s*====`, 3},
		{`===\s*(.+?)\s*===`, 4},
		{`==\s*(.+?)\s*==`, 5},
		{`=\s*(.+?)\s*=`, 6},
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern.regex)
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			matches := re.FindStringSubmatch(match)
			if len(matches) > 1 {
				element := DokuWikiElement{
					Type:    "header",
					Level:   pattern.level,
					Content: strings.TrimSpace(matches[1]),
				}
				return h.renderElement(element, outputFormat)
			}
			return match
		})
	}

	return content
}

// processTextFormatting handles text formatting for both HTML and Markdown
func (h *DokuwikiHandler) processTextFormatting(content string, outputFormat string) string {
	patterns := []struct {
		regex      string
		formatType string
	}{
		{`\*\*(.+?)\*\*`, "bold"},
		{`//(.+?)//`, "italic"},
		{`__(.+?)__`, "underline"},
		{`''(.+?)''`, "code"},
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern.regex)
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			matches := re.FindStringSubmatch(match)
			if len(matches) > 1 {
				element := DokuWikiElement{
					Type:    pattern.formatType,
					Content: matches[1],
				}
				return h.renderElement(element, outputFormat)
			}
			return match
		})
	}

	return content
}

// processLinks handles link conversion for both HTML and Markdown
func (h *DokuwikiHandler) processLinks(content string, outputFormat string) string {
	linkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	content = linkRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := linkRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			originalURL := strings.TrimSpace(matches[1])
			text := originalURL
			if len(matches) > 2 && matches[2] != "" {
				text = strings.TrimSpace(matches[2])
			}

			// convert URL for internal links (not external http/https links)
			convertedURL := originalURL
			if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
				// handle dokuwiki namespaces and anchors
				url := originalURL
				var anchor string
				hasAnchor := strings.Contains(url, "#")
				if hasAnchor {
					parts := strings.Split(url, "#")
					url = parts[0]
					// convert underscores to dashes in anchor
					anchor = "#" + strings.ReplaceAll(parts[1], "_", "-")
				}

				// convert dokuwiki namespace (colons) to filesystem path (slashes)
				url = strings.ReplaceAll(url, ":", "/")

				// for internal links, create web path
				if url != "" { // not just an anchor
					// don't add extension when anchor is present
					if !hasAnchor && !strings.HasSuffix(url, ".md") && !strings.HasSuffix(url, ".txt") {
						url += ".txt"
					}
					// use /files/docs/ prefix consistently
					convertedURL = fmt.Sprintf("/files/docs/%s%s", url, anchor)
				} else {
					// just an anchor link
					convertedURL = anchor
				}
			}

			element := DokuWikiElement{
				Type: "link",
				URL:  convertedURL,
				Text: text,
			}
			return h.renderElement(element, outputFormat)
		}
		return match
	})

	return content
}

// processLists handles list detection for both formats
func (h *DokuwikiHandler) processLists(content string, outputFormat string) string {
	if outputFormat == "html" {
		return h.processListsHTML(content)
	}

	return h.processListsMarkdown(content)
}

// processListsMarkdown handles markdown list conversion
func (h *DokuwikiHandler) processListsMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		// DokuWiki list pattern: exactly 2+ spaces + asterisk/dash + space + content
		listMatch := regexp.MustCompile(`^( {2,})(\*|-) (.+)$`).FindStringSubmatch(line)
		if listMatch != nil {
			spaces := listMatch[1]
			marker := listMatch[2]
			item := listMatch[3]

			// Calculate nesting level (DokuWiki: 2 spaces = level 1, 4 spaces = level 2, etc.)
			level := len(spaces) / 2

			element := DokuWikiElement{
				Type:    "list-item",
				Level:   level,
				Content: item,
				Text:    marker, // store marker type in Text field
			}

			result = append(result, h.renderElement(element, "markdown"))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// processListsHTML handles HTML list conversion with proper nesting
func (h *DokuwikiHandler) processListsHTML(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var listStack []string // tracks open list tags by level

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")

		// Detect list item and its level
		var isListItem bool
		var listType string
		var level int
		var item string

		if match := regexp.MustCompile(`^(  +)(\*|-) (.+)$`).FindStringSubmatch(trimmed); match != nil {
			level = len(match[1]) / 2
			if match[2] == "*" {
				listType = "ul"
			} else {
				listType = "ol"
			}
			item = match[3]
			isListItem = true
		}

		if isListItem {
			// Close lists deeper than current level
			for len(listStack) > level {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}

			// Check if we need to change list type at current level
			if len(listStack) == level && listStack[level-1] != listType {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}

			// Open new lists up to current level
			for len(listStack) < level {
				result = append(result, "<"+listType+">")
				listStack = append(listStack, listType)
			}

			result = append(result, "<li>"+item+"</li>")
		} else {
			// Close all open lists
			for len(listStack) > 0 {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}
			result = append(result, line)
		}
	}

	// Close any remaining open lists
	for len(listStack) > 0 {
		result = append(result, "</"+listStack[len(listStack)-1]+">")
		listStack = listStack[:len(listStack)-1]
	}

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------

// Helper functions for complex structures

// removeCatlistTags removes catlist tags completely
func (h *DokuwikiHandler) removeCatlistTags(content string) string {
	return regexp.MustCompile(`<catlist[^>]*>\s*`).ReplaceAllString(content, "")
}

// convertIncludeSections converts {{section> include plugin syntax
func (h *DokuwikiHandler) convertIncludeSections(content string, outputFormat string) string {
	sectionRegex := regexp.MustCompile(`\{\{section>([^&}]+)(?:[&}][^}]*)?\}\}`)
	content = sectionRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := sectionRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		pathSection := strings.TrimSpace(matches[1])

		// handle dokuwiki namespaces and anchors
		url := pathSection
		var anchor string
		hasAnchor := strings.Contains(url, "#")
		if hasAnchor {
			parts := strings.Split(url, "#")
			url = parts[0]
			// convert underscores to dashes in anchor
			anchor = "#" + strings.ReplaceAll(parts[1], "_", "-")
		}

		// convert dokuwiki namespace (colons) to filesystem path (slashes)
		url = strings.ReplaceAll(url, ":", "/")

		// don't add extension when anchor is present
		if !hasAnchor && !strings.HasSuffix(url, ".md") && !strings.HasSuffix(url, ".txt") {
			url += ".txt"
		}

		// use /files/docs/ prefix consistently
		webPath := fmt.Sprintf("/files/docs/%s%s", url, anchor)

		if outputFormat == "html" {
			return fmt.Sprintf(`<a href="%s">%s</a>`, webPath, pathSection)
		} else {
			// markdown format
			return fmt.Sprintf("[%s](%s)", pathSection, webPath)
		}
	})

	return content
}

// convertFoldedSections converts ++ folded sections
func (h *DokuwikiHandler) convertFoldedSections(content string) string {
	// Handle ++++ folded sections (four plus signs) - convert to details/summary
	quadFoldedRegex := regexp.MustCompile(`(?s)\+\+\+\+\s*([^|]+?)\s*\|\s*(.*?)\s*\+\+\+\+`)
	content = quadFoldedRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := quadFoldedRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		title := strings.TrimSpace(matches[1])
		innerContent := strings.TrimSpace(matches[2])
		return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>", title, innerContent)
	})

	// Handle ++ folded sections - process line by line to handle list items properly
	lines := strings.Split(content, "\n")
	var result []string
	var inFoldedSection bool
	var foldedContent []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if line contains ++ start pattern (including in list items)
		if !inFoldedSection && strings.Contains(line, "++") && strings.Contains(line, "|") {
			parts := strings.Split(line, "++")
			if len(parts) >= 2 {
				inFoldedSection = true
				foldedContent = []string{}
				// Check if there's content after the | on the same line
				afterPipe := strings.SplitN(line, "|", 2)
				if len(afterPipe) > 1 && strings.TrimSpace(afterPipe[1]) != "" {
					foldedContent = append(foldedContent, strings.TrimSpace(afterPipe[1]))
				}
				continue
			}
		}

		// Check if we're in a folded section
		if inFoldedSection {
			// Check if line ends the folded section
			if strings.Contains(line, "++") {
				// Add any content before ++ on this line
				parts := strings.Split(line, "++")
				if strings.TrimSpace(parts[0]) != "" {
					foldedContent = append(foldedContent, strings.TrimSpace(parts[0]))
				}

				// Process the folded content
				innerContent := strings.Join(foldedContent, "\n")
				innerContent = strings.TrimSpace(innerContent)

				// Just add the content directly - let markdown renderer handle it
				if innerContent != "" {
					result = append(result, innerContent)
				}

				inFoldedSection = false
				foldedContent = []string{}
				continue
			} else {
				// Add line to folded content
				foldedContent = append(foldedContent, line)
				continue
			}
		}

		// Regular line - add as is
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// addParagraphTags adds paragraph tags for HTML output
func (h *DokuwikiHandler) addParagraphTags(content string) string {
	// Handle line breaks
	content = strings.ReplaceAll(content, "\\\\", "<br>")

	// Add paragraph tags
	lines := strings.Split(content, "\n\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "<") {
			lines[i] = "<p>" + line + "</p>"
		}
	}
	content = strings.Join(lines, "\n")

	return content
}
