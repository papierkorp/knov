package parser

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/logging"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	gomarkdown_parser "github.com/gomarkdown/markdown/parser"
)

type MarkdownHandler struct{}

func NewMarkdownHandler() *MarkdownHandler {
	return &MarkdownHandler{}
}

func (h *MarkdownHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".index" || ext == ".moc" || ext == ".list"
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

type customRenderer struct {
	*html.Renderer
}

func (r *customRenderer) RenderNode(w io.Writer, node ast.Node, entering bool) ast.WalkStatus {
	if code, ok := node.(*ast.CodeBlock); ok && entering {
		lang := string(code.Info)
		if lang == "" {
			lang = "text"
		}
		highlighted := HighlightCodeBlock(string(code.Literal), lang)
		w.Write([]byte(highlighted))
		return ast.GoToNext
	}
	return r.Renderer.RenderNode(w, node, entering)
}

// update the Render function to use custom renderer:
func (h *MarkdownHandler) Render(content []byte) ([]byte, error) {
	extensions := gomarkdown_parser.CommonExtensions | gomarkdown_parser.AutoHeadingIDs
	p := gomarkdown_parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{
		Flags: htmlFlags,
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			if code, ok := node.(*ast.CodeBlock); ok && entering {
				lang := string(code.Info)
				if lang == "" {
					lang = "text"
				}
				highlighted := HighlightCodeBlock(string(code.Literal), lang)
				w.Write([]byte(highlighted))
				return ast.GoToNext, true
			}
			return ast.GoToNext, false
		},
	}
	renderer := html.NewRenderer(opts)

	htmlOutput := markdown.ToHTML(content, p, renderer)
	return htmlOutput, nil
}

func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	var links []string
	text := string(content)

	// remove code blocks to avoid extracting links from code
	text = removeCodeBlocks(text)

	// extract wiki-style links [[path]]
	wikiLinkRegex := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	wikiMatches := wikiLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range wikiMatches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			if link != "" {
				links = append(links, link)
			}
		}
	}

	// extract standard markdown links [text](url)
	mdLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	mdMatches := mdLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range mdMatches {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			// skip external urls and anchors
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "#") {
				links = append(links, link)
			}
		}
	}

	return links
}

// removeCodeBlocks removes fenced code blocks and inline code from markdown text
func removeCodeBlocks(text string) string {
	// remove fenced code blocks (```...```)
	fencedCodeRegex := regexp.MustCompile("(?s)```[^`]*```")
	text = fencedCodeRegex.ReplaceAllString(text, "")

	// remove inline code (`...`)
	inlineCodeRegex := regexp.MustCompile("`[^`]*`")
	text = inlineCodeRegex.ReplaceAllString(text, "")

	return text
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
