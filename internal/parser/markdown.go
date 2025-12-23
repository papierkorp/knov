package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/translation"
	"knov/internal/utils"

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
func (h *MarkdownHandler) Render(content []byte, filePath string) ([]byte, error) {
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

			// Add edit button after table closing tag
			if _, ok := node.(*ast.Table); ok && !entering {
				w.Write([]byte("</table>"))
				w.Write([]byte(`<div class="table-edit-wrapper">
					<a href="/files/edittable/` + filePath + `" class="btn-table-edit">
						<i class="fa fa-edit"></i> ` + translation.SprintfForRequest(configmanager.GetLanguage(), "edit table") + `
					</a>
				</div>`))
				return ast.GoToNext, true
			}

			return ast.GoToNext, false
		},
	}
	renderer := html.NewRenderer(opts)

	htmlOutput := markdown.ToHTML(content, p, renderer)

	// Post-process to add header edit buttons outside the header tags
	processedHTML := h.addHeaderEditButtons(string(htmlOutput), filePath)

	return []byte(processedHTML), nil
}

// addHeaderEditButtons adds edit buttons after header tags using post-processing
func (h *MarkdownHandler) addHeaderEditButtons(htmlContent, filePath string) string {
	// regex to match header tags with IDs
	headerRegex := regexp.MustCompile(`<h([1-6])\s+id="([^"]+)"[^>]*>(.*?)</h[1-6]>`)

	return headerRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := headerRegex.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		level := parts[1]
		headerID := parts[2]
		content := parts[3]

		// create edit button - styled like header anchor, positioned on right
		editButton := fmt.Sprintf(`<a href="/files/edit/%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
			filePath, headerID, translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"))

		// return header with edit button on the right
		return fmt.Sprintf(`<h%s id="%s">%s%s</h%s>`, level, headerID, content, editButton, level)
	})
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

// ExtractSection extracts a markdown section by header ID
func (h *MarkdownHandler) ExtractSection(content []byte, sectionID string) (string, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	var sectionStart, sectionEnd int
	var inSection bool
	usedIDs := make(map[string]int)

	// find section start and end
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			headerText := regexp.MustCompile(`^#+\s*`).ReplaceAllString(line, "")
			headerText = strings.TrimSpace(headerText)
			generatedID := utils.GenerateID(headerText, usedIDs)

			if generatedID == sectionID && !inSection {
				sectionStart = i
				inSection = true
				continue
			}

			if inSection && generatedID != sectionID {
				sectionEnd = i
				break
			}
		}
	}

	if !inSection {
		return "", fmt.Errorf("section not found: %s", sectionID)
	}

	if sectionEnd == 0 {
		sectionEnd = len(lines)
	}

	sectionLines := lines[sectionStart:sectionEnd]
	return strings.Join(sectionLines, "\n"), nil
}

// ReplaceSectionInMarkdown replaces a section in markdown content
func (h *MarkdownHandler) ReplaceSectionInMarkdown(content []byte, sectionID, newSectionContent string) ([]byte, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	var sectionStart, sectionEnd int
	var inSection bool
	usedIDs := make(map[string]int)

	// find section start and end
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			headerText := regexp.MustCompile(`^#+\s*`).ReplaceAllString(line, "")
			headerText = strings.TrimSpace(headerText)
			generatedID := utils.GenerateID(headerText, usedIDs)

			if generatedID == sectionID && !inSection {
				sectionStart = i
				inSection = true
				continue
			}

			if inSection && generatedID != sectionID {
				sectionEnd = i
				break
			}
		}
	}

	if !inSection {
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	if sectionEnd == 0 {
		sectionEnd = len(lines)
	}

	// replace section
	newLines := make([]string, 0, len(lines))
	newLines = append(newLines, lines[:sectionStart]...)
	newLines = append(newLines, strings.Split(newSectionContent, "\n")...)
	newLines = append(newLines, lines[sectionEnd:]...)

	return []byte(strings.Join(newLines, "\n")), nil
}
