package parser

import (
	"fmt"
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/translation"
)

// ---------------------------------------------------------------------------
// Pre-processing steps (markdown source → markdown source)
// Each step receives []byte and returns []byte.
// Steps run in order before the markdown parser sees the content.
// ---------------------------------------------------------------------------

// preExtractCodeBlocks replaces fenced code blocks with opaque placeholders so
// the markdown parser never sees their content (prevents # comments becoming
// headers, list items in code becoming lists, etc.).
// Preserves leading indentation so placeholders inside lists stay nested.
func (h *MarkdownHandler) preExtractCodeBlocks(content []byte) ([]byte, []codeBlock) {
	var blocks []codeBlock
	fenceRe := regexp.MustCompile("(?m)^([ \t]*)```([^\n]*)\n")
	lines := strings.Split(string(content), "\n")
	var result []string
	i := 0
	for i < len(lines) {
		m := fenceRe.FindStringSubmatch(lines[i] + "\n")
		if m == nil {
			result = append(result, lines[i])
			i++
			continue
		}
		indent := m[1]
		lang := strings.TrimSpace(m[2])
		if lang == "" {
			lang = "text"
		}
		i++
		var contentLines []string
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "```" {
				i++ // skip closing fence
				break
			}
			contentLines = append(contentLines, lines[i])
			i++
		}
		rawContent := strings.Join(contentLines, "\n") + "\n"
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", len(blocks))
		blocks = append(blocks, codeBlock{lang, rawContent})
		// emit with same indent so the placeholder stays inside any surrounding list item
		result = append(result, "", indent+placeholder, "")
	}
	return []byte(strings.Join(result, "\n")), blocks
}

// preSeparateLists injects HTML comment separators between top-level lists
// divided by blank lines, working around gomarkdown merging them into one list.
// Also ensures a blank line before ordered list items that follow a paragraph,
// so gomarkdown starts a proper <ol> instead of treating them as paragraph text.
// Only top-level items are affected — nested items must never have their context
// reset or gomarkdown interprets them as indented code blocks.
func (h *MarkdownHandler) preSeparateLists(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	topLevelRe := regexp.MustCompile(`^[-*] `)
	orderedRe := regexp.MustCompile(`^\d+\.\s`)
	inCodeBlock := false

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
		}

		// ensure blank line before an ordered list item that follows a paragraph
		if !inCodeBlock && orderedRe.MatchString(line) && i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			if prev != "" && !topLevelRe.MatchString(prev) && !orderedRe.MatchString(prev) {
				result = append(result, "")
			}
		}

		result = append(result, line)

		// inject separator after a top-level list item followed by a blank line
		if !inCodeBlock && topLevelRe.MatchString(line) && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			result = append(result, "", "<!-- -->", "")
		}
	}
	return []byte(strings.Join(result, "\n"))
}

// ---------------------------------------------------------------------------
// Post-processing steps (HTML → HTML)
// Each step receives a string and returns a string.
// Steps run in order after the markdown parser has produced HTML.
// ---------------------------------------------------------------------------

// postRestoreCodeBlocks replaces placeholders with syntax-highlighted <pre> blocks.
// Iterates in reverse to avoid substring collisions (e.g. KNOVCODEBLOCK1 inside KNOVCODEBLOCK10).
func (h *MarkdownHandler) postRestoreCodeBlocks(html string, blocks []codeBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", i)
		highlighted := HighlightCodeBlock(blocks[i].content, blocks[i].lang)
		html = strings.ReplaceAll(html, "<p>"+placeholder+"</p>", highlighted)
		html = strings.ReplaceAll(html, placeholder, highlighted)
	}
	return html
}

// postAddHeaderButtons injects edit-section anchor buttons into every header tag.
func (h *MarkdownHandler) postAddHeaderButtons(htmlContent, filePath string) string {
	headerRe := regexp.MustCompile(`<h([1-6])\s+id="([^"]+)"[^>]*>(.*?)</h[1-6]>`)
	return headerRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := headerRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		editBtn := fmt.Sprintf(
			`<a href="/files/edit/%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
			filePath, parts[2],
			translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
		)
		anchor := fmt.Sprintf(`<a href="#%s" class="header-anchor" aria-hidden="true">#</a>`, parts[2])
		return fmt.Sprintf(`<h%s id="%s">%s%s%s</h%s>`, parts[1], parts[2], parts[3], editBtn, anchor, parts[1])
	})
}

// postFixMisrenderedListsInCode detects <pre><code class="language-text"> blocks
// whose content consists entirely of list items and converts them to proper nested
// HTML lists. This catches cases where gomarkdown misparses indented list items as
// code blocks.
func postFixMisrenderedListsInCode(htmlContent string) string {
	preRe := regexp.MustCompile(`(?s)<pre[^>]*><code[^>]*class="language-text"[^>]*>(.*?)</code></pre>`)
	listLineRe := regexp.MustCompile(`^( *)([-*]) (.*)$`)
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")

	return preRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		m := preRe.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		lines := strings.Split(strings.TrimRight(m[1], "\n"), "\n")

		// only convert if every non-empty line is a list item
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !listLineRe.MatchString(line) {
				return match
			}
		}

		var buf strings.Builder
		depth := 0
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lm := listLineRe.FindStringSubmatch(line)
			if lm == nil {
				continue
			}
			level := len(lm[1])/2 + 1
			for depth < level {
				buf.WriteString("<ul>\n")
				depth++
			}
			for depth > level {
				buf.WriteString("</ul>\n")
				depth--
			}
			itemContent := inlineCodeRe.ReplaceAllString(lm[3], "<code>$1</code>")
			fmt.Fprintf(&buf, "<li>%s</li>\n", itemContent)
		}
		for depth > 0 {
			buf.WriteString("</ul>\n")
			depth--
		}
		return buf.String()
	})
}

// postCleanupListParagraphs removes unnecessary <p> tags that gomarkdown wraps
// around list item content in loose lists.
func (h *MarkdownHandler) postCleanupListParagraphs(htmlContent string) string {
	htmlContent = regexp.MustCompile(`<br>\s*</p>\s*</li>`).ReplaceAllString(htmlContent, `<br></li>`)
	htmlContent = regexp.MustCompile(`</p>\s*</li>`).ReplaceAllString(htmlContent, `</li>`)
	htmlContent = regexp.MustCompile(`</p>(\s*<ul>)`).ReplaceAllString(htmlContent, `$1`)
	htmlContent = regexp.MustCompile(`<li><p>`).ReplaceAllString(htmlContent, `<li>`)
	return htmlContent
}

// postWrapHeaderSections wraps the content between each header in a
// <div class="content-section"> so CSS and JS can target per-section areas.
func (h *MarkdownHandler) postWrapHeaderSections(htmlContent string) string {
	headerRe := regexp.MustCompile(`<h([1-6])[^>]*>.*?</h[1-6]>`)
	matches := headerRe.FindAllStringIndex(htmlContent, -1)

	if len(matches) == 0 {
		return fmt.Sprintf(`<div class="content-section">%s</div>`, htmlContent)
	}

	var out strings.Builder

	if matches[0][0] > 0 {
		before := strings.TrimSpace(htmlContent[:matches[0][0]])
		if before != "" {
			fmt.Fprintf(&out, `<div class="content-section">%s</div>`, before)
		}
	}

	for i, match := range matches {
		out.WriteString(htmlContent[match[0]:match[1]])

		start := match[1]
		end := len(htmlContent)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := strings.TrimSpace(htmlContent[start:end])
		if section != "" {
			fmt.Fprintf(&out, `<div class="content-section">%s</div>`, section)
		}
	}
	return out.String()
}
