package filetype

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	gomarkdown_parser "github.com/gomarkdown/markdown/parser"
	"knov/internal/logging"
)

type MarkdownHandler struct{}

func NewMarkdownHandler() *MarkdownHandler {
	return &MarkdownHandler{}
}

func (h *MarkdownHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown"
}

func (h *MarkdownHandler) GetContent(filepath string) ([]byte, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filepath, err)
		return nil, err
	}
	return content, nil
}

func (h *MarkdownHandler) Parse(content []byte) ([]byte, error) {
	processed := h.processMarkdownLinks(string(content))
	return []byte(processed), nil
}

func (h *MarkdownHandler) Render(content []byte) ([]byte, error) {
	extensions := gomarkdown_parser.CommonExtensions | gomarkdown_parser.AutoHeadingIDs
	p := gomarkdown_parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	html := markdown.ToHTML(content, p, renderer)
	return html, nil
}

func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	return h.extractMarkdownLinks(string(content))
}

func (h *MarkdownHandler) Name() string {
	return "markdown"
}

// processMarkdownLinks converts markdown-style links to HTML anchors
func (h *MarkdownHandler) processMarkdownLinks(content string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		text := strings.TrimSpace(matches[1])
		url := strings.TrimSpace(matches[2])

		if !strings.Contains(url, "://") && !strings.HasPrefix(url, "#") {
			if !strings.HasSuffix(url, ".md") {
				url += ".md"
			}
			return `<a href="/files/` + url + `">` + text + `</a>`
		}

		return `<a href="` + url + `">` + text + `</a>`
	})

	return content
}

// extractMarkdownLinks extracts all links from markdown content
func (h *MarkdownHandler) extractMarkdownLinks(content string) []string {
	var links []string
	linkSet := make(map[string]bool)

	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 2 {
			url := strings.TrimSpace(match[2])
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
