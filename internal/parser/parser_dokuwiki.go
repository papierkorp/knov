package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DokuwikiHandler struct{}

func NewDokuwikiHandler() *DokuwikiHandler {
	return &DokuwikiHandler{}
}

func (h *DokuwikiHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == ".dokuwiki" {
		return true
	}

	if ext == ".txt" {
		content, err := os.ReadFile(filename)
		if err == nil {
			s := strings.TrimPrefix(string(content), "\xEF\xBB\xBF") // strip UTF-8 BOM
			for _, line := range strings.Split(s, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "======") || strings.HasPrefix(trimmed, "=====") {
					return true
				}
				break
			}
		}
	}

	return false
}

func (h *DokuwikiHandler) Parse(content []byte) ([]byte, error) {
	// return as html, also change Render function
	// parsed := h.ConvertToHTML(string(content))
	// return []byte(parsed), nil
	// return as txt, also change Render function
	s := string(content)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return []byte(s), nil
}

func (h *DokuwikiHandler) Render(content []byte, filePath string) ([]byte, error) {
	// return as html, based on Parse function
	// return content, nil

	// return as text, based on Parse function
	html := "<pre>" + string(content) + "</pre>"
	return []byte(html), nil
}

func (h *DokuwikiHandler) ExtractLinks(content []byte) []string {
	return nil
	// return h.extractDokuWikiLinks(string(content))
}

func (h *DokuwikiHandler) Name() string {
	return "dokuwiki"
}

// ConvertToMarkdown converts DokuWiki syntax to Markdown using unified processing
func (h *DokuwikiHandler) ConvertToMarkdown(content string) string {
	content, escapes := h.extractEscapes(content)
	content, rawBlocks := h.extractRawCodeBlocks(content)
	content = h.stripLeadingSpaces(content)
	content = h.restoreRawCodeBlocks(content, rawBlocks)
	content = h.handleComplexStructures(content)
	content = h.processDokuWikiSyntax(content, "markdown")
	content = h.restoreEscapes(content, escapes, "markdown")
	return content
}

// ConvertToHTML converts DokuWiki syntax to HTML using unified processing
func (h *DokuwikiHandler) ConvertToHTML(content string) string {
	content, escapes := h.extractEscapes(content)
	content, rawBlocks := h.extractRawCodeBlocks(content)
	content = h.stripLeadingSpaces(content)
	content = h.restoreRawCodeBlocks(content, rawBlocks)
	content = h.handleComplexStructures(content)
	content = h.processDokuWikiSyntax(content, "html")
	content = h.addParagraphTags(content)
	content = h.restoreEscapes(content, escapes, "html")
	return content
}

// extractRawCodeBlocks replaces raw <code>/<file>/<sxh>/<codify> tags with placeholders
// so their indented content is not touched by stripLeadingSpaces.
func (h *DokuwikiHandler) extractRawCodeBlocks(content string) (string, []string) {
	var blocks []string
	re := regexp.MustCompile(`(?s)<(?:code|file|sxh|codify)(?:\s[^>]*)?>.*?</(?:code|file|sxh|codify)>`)
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		placeholder := fmt.Sprintf("\x00RAW%d\x00", len(blocks))
		blocks = append(blocks, match)
		return placeholder
	})
	return result, blocks
}

// restoreRawCodeBlocks replaces placeholders back with the original tag content.
func (h *DokuwikiHandler) restoreRawCodeBlocks(content string, blocks []string) string {
	for i, block := range blocks {
		content = strings.ReplaceAll(content, fmt.Sprintf("\x00RAW%d\x00", i), block)
	}
	return content
}

// stripLeadingSpaces removes leading whitespace from lines, but preserves
// DokuWiki list indentation (lines starting with 2+ spaces followed by * or -).
func (h *DokuwikiHandler) stripLeadingSpaces(content string) string {
	listLineRe := regexp.MustCompile(`^( {2,})(\*|-) `)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if listLineRe.MatchString(line) {
			continue
		}
		lines[i] = strings.TrimLeft(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// extractEscapes replaces %%...%% spans with unique placeholders before any other processing
func (h *DokuwikiHandler) extractEscapes(content string) (string, []string) {
	var escapes []string
	result := regexp.MustCompile(`%%(.*?)%%`).ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2]
		placeholder := fmt.Sprintf("\x00ESC%d\x00", len(escapes))
		escapes = append(escapes, inner)
		return placeholder
	})
	return result, escapes
}

// restoreEscapes replaces placeholders back with their inner content as inline code
func (h *DokuwikiHandler) restoreEscapes(content string, escapes []string, outputFormat string) string {
	for i, inner := range escapes {
		var restored string
		if outputFormat == "html" {
			escaped := strings.ReplaceAll(inner, "&", "&amp;")
			escaped = strings.ReplaceAll(escaped, "<", "&lt;")
			escaped = strings.ReplaceAll(escaped, ">", "&gt;")
			restored = "<code>" + escaped + "</code>"
		} else {
			restored = "`" + inner + "`"
		}
		content = strings.ReplaceAll(content, fmt.Sprintf("\x00ESC%d\x00", i), restored)
	}
	return content
}

// extractCodeBlocks replaces fenced code blocks and HTML pre/code blocks with placeholders
// to protect their content from further processing (e.g. catlist replacement)
func (h *DokuwikiHandler) extractCodeBlocks(content string) (string, []string) {
	var blocks []string
	re := regexp.MustCompile("(?s)```[^\n]*\n.*?```|<pre><code>.*?</code></pre>")
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		placeholder := fmt.Sprintf("\x00CODE%d\x00", len(blocks))
		blocks = append(blocks, match)
		return placeholder
	})
	return result, blocks
}

// restoreCodeBlocks replaces code block placeholders back with their original content
func (h *DokuwikiHandler) restoreCodeBlocks(content string, blocks []string) string {
	for i, block := range blocks {
		content = strings.ReplaceAll(content, fmt.Sprintf("\x00CODE%d\x00", i), block)
	}
	return content
}

// extractURLs replaces URLs (http/https) with placeholders to protect them from text formatting
func (h *DokuwikiHandler) extractURLs(content string) (string, []string) {
	var urls []string
	result := regexp.MustCompile(`https?://[^\s)\]"<]+`).ReplaceAllStringFunc(content, func(match string) string {
		placeholder := fmt.Sprintf("\x00URL%d\x00", len(urls))
		urls = append(urls, match)
		return placeholder
	})
	return result, urls
}

// restoreURLs replaces URL placeholders back with the original URLs
func (h *DokuwikiHandler) restoreURLs(content string, urls []string) string {
	for i, url := range urls {
		content = strings.ReplaceAll(content, fmt.Sprintf("\x00URL%d\x00", i), url)
	}
	return content
}

func (h *DokuwikiHandler) handleComplexStructures(content string) string {
	// Remove tablelayout plugin syntax
	content = regexp.MustCompile(`\{\{[^}]*tablelayout[^}]*\}\}`).ReplaceAllString(content, "")

	// Handle folded sections (++ title | content ++)
	content = h.convertFoldedSections(content)

	return content
}

// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------

// DokuWikiElement represents a detected DokuWiki syntax element
type DokuWikiElement struct {
	Type     string
	Level    int
	Content  string
	URL      string
	Text     string
	Language string     // for code blocks
	Headers  []string   // for tables
	Rows     [][]string // for tables
}

// renderElement renders a DokuWiki element in the specified format
func (h *DokuwikiHandler) renderElement(element DokuWikiElement, outputFormat string) string {
	if outputFormat == "markdown" {
		return h.renderAsMarkdown(element)
	}
	return h.renderAsHTML(element)
}

// renderAsHTML renders DokuWiki elements as HTML
func (h *DokuwikiHandler) renderAsHTML(element DokuWikiElement) string {
	switch element.Type {
	case "header":
		return fmt.Sprintf("<h%d>%s</h%d>", element.Level, element.Content, element.Level)
	case "bold":
		return fmt.Sprintf("<strong>%s</strong>", element.Content)
	case "italic":
		return fmt.Sprintf("<em>%s</em>", element.Content)
	case "underline":
		return fmt.Sprintf("<u>%s</u>", element.Content)
	case "code":
		return fmt.Sprintf("<code>%s</code>", element.Content)
	case "link":
		if strings.HasPrefix(element.URL, "http://") || strings.HasPrefix(element.URL, "https://") {
			return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, element.URL, element.Text)
		}
		// use URL directly since it already has correct /files/docs/ prefix
		return fmt.Sprintf(`<a href="%s">%s</a>`, element.URL, element.Text)
	case "code-block":
		// if element.Language != "" codehighlight
		return fmt.Sprintf("<pre><code>%s</code></pre>", element.Content)
	case "list-item":
		// Note: The actual list tag nesting is handled in processListsHTML
		return fmt.Sprintf("<li>%s</li>", element.Content)
	case "table":
		return h.renderTableAsHTML(element)
	}
	return element.Content
}

// renderAsMarkdown renders DokuWiki elements as Markdown
func (h *DokuwikiHandler) renderAsMarkdown(element DokuWikiElement) string {
	switch element.Type {
	case "header":
		prefix := strings.Repeat("#", element.Level)
		return fmt.Sprintf("%s %s", prefix, element.Content)
	case "bold":
		return fmt.Sprintf("**%s**", element.Content)
	case "italic":
		return fmt.Sprintf("*%s*", element.Content)
	case "underline":
		return fmt.Sprintf("_%s_", element.Content)
	case "code":
		return fmt.Sprintf("`%s`", element.Content)
	case "link":
		return fmt.Sprintf("[%s](%s)", element.Text, element.URL)
	case "code-block":
		if element.Language != "" {
			return fmt.Sprintf("```%s\n%s\n```", element.Language, element.Content)
		}
		return fmt.Sprintf("```\n%s\n```", element.Content)
	case "list-item":
		indent := strings.Repeat("  ", element.Level-1)
		if element.Text == "*" {
			return fmt.Sprintf("%s- %s", indent, element.Content)
		} else {
			return fmt.Sprintf("%s1. %s", indent, element.Content)
		}
	case "table":
		return h.renderTableAsMarkdown(element)
	}
	return element.Content
}
