package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
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

			// Handle media images - convert media/ paths to /static/media/
			if img, ok := node.(*ast.Image); ok && entering {
				dest := string(img.Destination)
				if strings.HasPrefix(dest, "media/") {
					// Convert media/ to /static/media/ for proper serving
					img.Destination = []byte("/static/" + dest)
				}
			}

			return ast.GoToNext, false
		},
	}
	renderer := html.NewRenderer(opts)

	htmlOutput := markdown.ToHTML(content, p, renderer)

	// Post-process to add header edit buttons outside the header tags
	processedHTML := h.addHeaderButtons(string(htmlOutput), filePath)

	return []byte(processedHTML), nil
}

// addHeaderButtons adds edit buttons after header tags using post-processing
func (h *MarkdownHandler) addHeaderButtons(htmlContent, filePath string) string {
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

	// extract wiki-style links [[path|text]] or [[path]]
	wikiLinkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	wikiMatches := wikiLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range wikiMatches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			// skip external urls
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.Contains(link, "://") {
				links = append(links, link)
			}
		}
	}

	// extract media links {{path}}
	mediaLinkRegex := regexp.MustCompile(`\{\{([^\}]+)\}\}`)
	mediaMatches := mediaLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range mediaMatches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
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
	// first, convert wiki-style links [[link|text]] and [[link]] to markdown format
	wikiLinkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	content = wikiLinkRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := wikiLinkRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		link := strings.TrimSpace(matches[1])
		text := link
		if len(matches) > 2 && matches[2] != "" {
			text = strings.TrimSpace(matches[2])
		}

		// external links
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.Contains(link, "://") {
			return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, link, text)
		}

		// internal links
		if !strings.HasSuffix(link, ".md") && !strings.HasSuffix(link, ".txt") {
			link += ".md"
		}
		return fmt.Sprintf(`<a href="/files/%s">%s</a>`, link, text)
	})

	// convert media links {{link}} to markdown image syntax
	mediaLinkRegex := regexp.MustCompile(`\{\{([^\}]+)\}\}`)
	content = mediaLinkRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := mediaLinkRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		link := strings.TrimSpace(matches[1])

		// if it starts with media/, it's a media file
		if strings.HasPrefix(link, "media/") {
			return fmt.Sprintf(`<a href="/static/%s">%s</a>`, link, filepath.Base(link))
		}

		// otherwise treat as file link
		if !strings.HasSuffix(link, ".md") && !strings.HasSuffix(link, ".txt") {
			link += ".md"
		}
		return fmt.Sprintf(`<a href="/files/%s">%s</a>`, link, filepath.Base(link))
	})

	// process regular markdown links [text](url)
	re := regexp.MustCompile(`(!)?\[([^\]]+)\]\(([^)]+)\)`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 4 {
			return match
		}

		isImage := matches[1] == "!"
		text := strings.TrimSpace(matches[2])
		url := strings.TrimSpace(matches[3])

		// if it's an image link, don't process it - return as-is
		if isImage {
			return match
		}

		// if it's a media link, convert to media route
		if strings.HasPrefix(url, "media/") {
			return `<a href="/static/` + url + `">` + text + `</a>`
		}

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

// ExtractSectionContent extracts content of a specific section from a markdown file
func ExtractSectionContent(filePath, sectionID string) (string, error) {
	fullPath := contentStorage.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return extractSectionFromMarkdown(string(content), sectionID)
}

// SaveSectionContent saves content to a specific section in a markdown file
func SaveSectionContent(filePath, sectionID, sectionContent string) error {
	fullPath := contentStorage.ToDocsPath(filePath)
	originalContent, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return err
	}

	updatedContent, err := replaceSectionInMarkdown(string(originalContent), sectionID, sectionContent)
	if err != nil {
		return err
	}

	return contentStorage.WriteFile(fullPath, []byte(updatedContent), 0644)
}

// extractSectionFromMarkdown extracts content between headers including the header itself
func extractSectionFromMarkdown(content, sectionID string) (string, error) {
	lines := strings.Split(content, "\n")

	var sectionStart, sectionEnd int
	var inSection bool
	var inCodeBlock bool
	usedIDs := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inSection {
				sectionStart = i
				inSection = true
				continue
			}

			if inSection && headerID != sectionID {
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

// replaceSectionInMarkdown replaces content of a specific section including the header
func replaceSectionInMarkdown(content, sectionID, newContent string) (string, error) {
	lines := strings.Split(content, "\n")

	var result []string
	var inTargetSection bool
	var inCodeBlock bool
	var headerLevel int
	usedIDs := make(map[string]int)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inTargetSection {
				// start of target section - replace with new content
				inTargetSection = true
				headerLevel = level
				if strings.TrimSpace(newContent) != "" {
					result = append(result, strings.Split(newContent, "\n")...)
				}
				continue
			} else if inTargetSection && level <= headerLevel {
				// reached next section of same or higher level
				inTargetSection = false
				result = append(result, line)
				continue
			}
		}

		if !inTargetSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), nil
}
