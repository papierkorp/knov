package dokuwikiconverter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// processDokuWikiSyntax applies unified detection and rendering for all syntax types
func (h *Converter) processDokuWikiSyntax(content string, outputFormat string) string {
	// Process in order of complexity to avoid conflicts

	// 0. Strip <nowiki> tags — keep their content but prevent further processing
	content = regexp.MustCompile(`(?s)<nowiki>(.*?)</nowiki>`).ReplaceAllString(content, "$1")

	// 0b. Extract inline code ''..'' before anything else so their content is never processed
	content, inlineCodes := h.extractInlineCodes(content)

	// 1. Handle include sections first (needs outputFormat)
	content = h.convertIncludeSections(content, outputFormat)

	// 2. Handle media links (needs outputFormat)
	content = h.processMediaLinks(content, outputFormat)

	// 3. Tables first (so they don't get processed as other syntax)
	content = h.ProcessTables(content, outputFormat)

	// 4. Code blocks (convert tags, then protect content from further processing)
	content = h.processCodeBlocks(content, outputFormat)
	content, codeBlocks := h.extractCodeBlocks(content)

	// 4b. Folded sections (++ title | content ++) — must run after processCodeBlocks so
	// fence markers (```lang) are already in place when indentation is calculated
	if outputFormat == "markdown" {
		content = h.convertFoldedSections(content)
	}

	// 4c. Replace catlist (after code blocks are extracted so codeblocks are protected)
	content = h.replaceCatlistTags(content, outputFormat)

	// 6. Headers
	content = h.processHeaders(content, outputFormat)

	// 7. Links (before text formatting to protect :// in URLs)
	content = h.processLinks(content, outputFormat)

	// 8. Text formatting - protect URLs first so // italic regex doesn't mangle them
	content, urls := h.extractURLs(content)
	content = h.processTextFormatting(content, outputFormat)
	content = h.restoreURLs(content, urls)

	// 9. Lists (last, so other formatting inside lists works)
	content = h.processLists(content, outputFormat)

	// 10. Restore protected code block content
	content = h.restoreCodeBlocks(content, codeBlocks)

	// 11. Restore inline codes (rendered, content untouched)
	content = h.restoreInlineCodes(content, inlineCodes, outputFormat)

	return content
}

// extractInlineCodes extracts ”..” inline code spans before any other processing
func (h *Converter) extractInlineCodes(content string) (string, []string) {
	var codes []string
	re := regexp.MustCompile(`''(.+?)''`)
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		m := re.FindStringSubmatch(match)
		placeholder := fmt.Sprintf("\x00INLINE%d\x00", len(codes))
		codes = append(codes, m[1])
		return placeholder
	})
	return result, codes
}

// restoreInlineCodes renders and restores extracted inline code placeholders
func (h *Converter) restoreInlineCodes(content string, codes []string, outputFormat string) string {
	for i, code := range codes {
		var rendered string
		if outputFormat == "html" {
			rendered = fmt.Sprintf("<code>%s</code>", code)
		} else {
			rendered = fmt.Sprintf("`%s`", code)
		}
		content = strings.ReplaceAll(content, fmt.Sprintf("\x00INLINE%d\x00", i), rendered)
	}
	return content
}

// processMediaLinks handles media link conversion for both HTML and Markdown
func (h *Converter) processMediaLinks(content string, outputFormat string) string {
	// matches {{ :path/to/file | optional alt }} or {{ namespace:path | alt }} with optional spaces
	mediaRegex := regexp.MustCompile(`\{\{\s*:?([^|}]+)(?:\|([^}]*))?\}\}`)
	content = mediaRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := mediaRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// convert dokuwiki namespace (colons) to filesystem path (slashes)
		mediaPath := strings.TrimSpace(matches[1])
		mediaPath = strings.ReplaceAll(mediaPath, ":", "/")
		// strip query params (e.g. ?direct, ?400)
		if qi := strings.Index(mediaPath, "?"); qi != -1 {
			mediaPath = mediaPath[:qi]
		}
		// dokuwiki stores all media filenames in lowercase
		mediaPath = strings.ToLower(mediaPath)

		// if the path has no directory component it is relative to the source file's folder
		if h.fileDir != "" && !strings.Contains(mediaPath, "/") {
			mediaPath = h.fileDir + "/" + mediaPath
		}

		altText := strings.TrimSpace(matches[2])
		if altText == "" {
			altText = filepath.Base(mediaPath)
		}

		mediaURL := fmt.Sprintf("/media/%s", mediaPath)

		// PDFs render as links, everything else as images
		if strings.ToLower(filepath.Ext(mediaPath)) == ".pdf" {
			if outputFormat == "html" {
				return fmt.Sprintf(`<a href="%s">%s</a>`, mediaURL, altText)
			}
			return fmt.Sprintf("[%s](%s)", altText, mediaURL)
		}

		if outputFormat == "html" {
			return fmt.Sprintf(`<img src="%s" alt="%s" />`, mediaURL, altText)
		}
		return fmt.Sprintf("![%s](%s)", altText, mediaURL)
	})

	return content
}

// processCodeBlocks handles all code block syntaxes
func (h *Converter) processCodeBlocks(content string, outputFormat string) string {
	patterns := []struct {
		regex string
		tag   string
	}{
		{`(?s)<sxh\s+([a-zA-Z0-9_-]+)(?:\s+[^\s>]+)?\s*>(.*?)</sxh>`, "sxh"},
		{`(?s)<sxh>(.*?)</sxh>`, "simple-code"},
		{`(?s)<codify\s+([a-zA-Z0-9_-]+)(?:\s+[^\s>]+)?\s*>(.*?)</codify>`, "codify"},
		{`(?s)<codify>(.*?)</codify>`, "simple-code"},
		{`(?s)<code\s+([a-zA-Z0-9_-]+)(?:\s+[^\s>]+)?\s*>(.*?)</code>`, "code"},
		{`(?s)<file\s+([a-zA-Z0-9_-]+)(?:\s+[^\s>]+)?\s*>(.*?)</file>`, "file"},
		{`(?s)<file>(.*?)</file>`, "simple-code"},
		{`(?s)<code>(.*?)</code>`, "simple-code"},
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
					Content:  strings.Trim(matches[1], "\n\r"),
				}
			} else if len(matches) >= 3 {
				element = DokuWikiElement{
					Type:     "code-block",
					Language: strings.TrimSpace(matches[1]),
					Content:  strings.Trim(matches[2], "\n\r"),
				}
			}

			return h.renderElement(element, outputFormat)
		})
	}

	return content
}

// processHeaders handles header conversion for both HTML and Markdown
func (h *Converter) processHeaders(content string, outputFormat string) string {
	patterns := []struct {
		regex string
		level int
	}{
		{`(?m)^======\s*(.+?)\s*======\s*$`, 1},
		{`(?m)^=====\s*(.+?)\s*=====\s*$`, 2},
		{`(?m)^====\s*(.+?)\s*====\s*$`, 3},
		{`(?m)^===\s*(.+?)\s*===\s*$`, 4},
		{`(?m)^==\s*(.+?)\s*==\s*$`, 5},
		{`(?m)^=\s*(.+?)\s*=\s*$`, 6},
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
func (h *Converter) processTextFormatting(content string, outputFormat string) string {
	patterns := []struct {
		regex      string
		formatType string
	}{
		{`\*\*(.+?)\*\*`, "bold"},
		{`//(.+?)//`, "italic"},
		{`__(.+?)__`, "underline"},
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
func (h *Converter) processLinks(content string, outputFormat string) string {
	linkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	content = linkRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := linkRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			originalURL := strings.TrimSpace(matches[1])
			text := originalURL
			if len(matches) > 2 && matches[2] != "" {
				text = strings.TrimSpace(matches[2])
			}

			// convert URL for internal links (not external http/https/www links)
			convertedURL := originalURL
			if strings.HasPrefix(originalURL, "www.") {
				convertedURL = "https://" + originalURL
			}
			isExternal := strings.HasPrefix(originalURL, "http://") ||
				strings.HasPrefix(originalURL, "https://") ||
				strings.HasPrefix(originalURL, "www.")
			if !isExternal {
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

				// convert dokuwiki namespace separators (colons and >) to filesystem path (slashes)
				url = strings.ReplaceAll(url, ":", "/")
				url = strings.ReplaceAll(url, ">", "/")

				// for internal links, create web path
				if url != "" { // not just an anchor
					ext := strings.ToLower(filepath.Ext(url))
					// links pointing at binary/media files go to /media/, not /files/docs/
					isMediaFile := ext != "" && ext != ".md" && ext != ".txt"
					if isMediaFile {
						convertedURL = fmt.Sprintf("/media/%s%s", url, anchor)
					} else {
						if !strings.HasSuffix(url, ".md") {
							url += ".md"
						}
						convertedURL = fmt.Sprintf("/files/docs/%s%s", url, anchor)
					}
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
func (h *Converter) processLists(content string, outputFormat string) string {
	if outputFormat == "html" {
		return h.processListsHTML(content)
	}

	return h.processListsMarkdown(content)
}

// processListsMarkdown handles markdown list conversion
func (h *Converter) processListsMarkdown(content string) string {
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
func (h *Converter) processListsHTML(content string) string {
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

// replaceCatlistTags replaces catlist tags with a link to the browse/folders page.
// It extracts the p:namespace argument (e.g. "p:it" → "/browse/folders/it").
func (h *Converter) replaceCatlistTags(content string, outputFormat string) string {
	return regexp.MustCompile(`<catlist([^>]*)>\s*`).ReplaceAllStringFunc(content, func(match string) string {
		args := regexp.MustCompile(`<catlist([^>]*)>`).FindStringSubmatch(match)
		path := ""
		if len(args) > 1 {
			// extract p:namespace argument
			if m := regexp.MustCompile(`p:([^\s>]+)`).FindStringSubmatch(args[1]); len(m) > 1 {
				// convert dokuwiki colons to path separators
				path = "/" + strings.ReplaceAll(m[1], ":", "/")
			}
		}
		url := "/browse/folders" + path
		if outputFormat == "html" {
			return fmt.Sprintf("<a href=\"%s\">%s</a>\n", url, url)
		}
		return fmt.Sprintf("[%s](%s)\n", url, url)
	})
}

// convertIncludeSections converts {{section> include plugin syntax
func (h *Converter) convertIncludeSections(content string, outputFormat string) string {
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
		if !strings.HasSuffix(url, ".md") {
			url += ".md"
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
func (h *Converter) convertFoldedSections(content string) string {
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
	var foldedIndent string // leading whitespace of the list item containing the folded section

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if line contains ++ start pattern (including in list items)
		if !inFoldedSection && strings.Contains(line, "++") && strings.Contains(line, "|") {
			parts := strings.Split(line, "++")
			if len(parts) >= 2 {
				inFoldedSection = true
				foldedContent = []string{}

				// extract only the leading whitespace (not the list marker * or -)
				listPrefix := parts[0]
				foldedIndent = listPrefix
				if trimmed := strings.TrimLeft(listPrefix, " \t"); len(trimmed) > 0 {
					foldedIndent = listPrefix[:len(listPrefix)-len(trimmed)]
				}

				// extract title (between ++ and |)
				afterOpen := strings.Join(parts[1:], "++")
				titleAndRest := strings.SplitN(afterOpen, "|", 2)
				title := strings.TrimSpace(titleAndRest[0])

				// emit title as a list item at the same indent level
				if title != "" {
					result = append(result, foldedIndent+"* "+title)
				}

				// check if there's inline content after the |
				if len(titleAndRest) > 1 && strings.TrimSpace(titleAndRest[1]) != "" {
					foldedContent = append(foldedContent, strings.TrimSpace(titleAndRest[1]))
				}
				continue
			}
		}

		// Check if we're in a folded section
		if inFoldedSection {
			// closing ++ can be standalone or at the end of a line (e.g. </code>++)
			trimmed := strings.TrimSpace(line)
			isClosing := trimmed == "++" || strings.HasSuffix(trimmed, "++")
			if isClosing {
				// add any content before the closing ++
				beforeClose := strings.TrimSpace(strings.TrimSuffix(trimmed, "++"))
				if beforeClose != "" {
					foldedContent = append(foldedContent, beforeClose)
				}

				innerContent := strings.TrimSpace(strings.Join(foldedContent, "\n"))
				if innerContent != "" {
					// indent non-fence lines so they stay nested in the list;
					// fence markers (```lang / ```) get childIndent, content inside stays untouched.
					// foldedIndent is raw DokuWiki whitespace (2 spaces per level); markdown renders
					// level N as (N-1)*2 spaces, so child content sits at N*2 spaces.
					childIndent := strings.Repeat("  ", len(foldedIndent)/2)
					indentedLines := strings.Split(innerContent, "\n")
					inFence := false
					for j, l := range indentedLines {
						trimmed := strings.TrimSpace(l)
						if strings.HasPrefix(trimmed, "```") {
							inFence = !inFence
							indentedLines[j] = childIndent + trimmed
						} else if inFence {
							indentedLines[j] = l // fence content: leave untouched
						} else if l != "" {
							indentedLines[j] = childIndent + l
						}
					}
					result = append(result, strings.Join(indentedLines, "\n"))
				}
				inFoldedSection = false
				foldedContent = []string{}
				foldedIndent = ""
				continue
			} else {
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
func (h *Converter) addParagraphTags(content string) string {
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
