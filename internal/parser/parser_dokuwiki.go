package parser

import (
	"fmt"
	"os"
	"path/filepath"
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
			lines := strings.Split(string(content), "\n")
			if len(lines) > 0 {
				firstLine := strings.TrimSpace(lines[0])
				if strings.HasPrefix(firstLine, "======") || strings.HasPrefix(firstLine, "=====") {
					return true
				}
			}
		}
	}

	return false
}

func (h *DokuwikiHandler) Parse(content []byte) ([]byte, error) {
	parsed := h.ConvertToHTML(string(content))
	return []byte(parsed), nil
}

func (h *DokuwikiHandler) Render(content []byte, filePath string) ([]byte, error) {
	return content, nil
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
	// Handle special DokuWiki plugins and complex structures first
	content = h.handleComplexStructures(content)

	// Use unified syntax processing
	content = h.processDokuWikiSyntax(content, "markdown")

	return content
}

// ConvertToHTML converts DokuWiki syntax to HTML using unified processing
func (h *DokuwikiHandler) ConvertToHTML(content string) string {
	// Handle special DokuWiki plugins and complex structures first
	content = h.handleComplexStructures(content)

	// Use unified syntax processing
	content = h.processDokuWikiSyntax(content, "html")

	// Add paragraph tags for HTML
	content = h.addParagraphTags(content)

	return content
}

// handleComplexStructures processes complex DokuWiki syntax that needs special handling
func (h *DokuwikiHandler) handleComplexStructures(content string) string {
	// Remove catlist tags completely
	content = h.removeCatlistTags(content)

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
		return fmt.Sprintf("<u>%s</u>", element.Content) // no markdown equivalent
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
