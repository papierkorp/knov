package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/pathutils"
	"knov/internal/translation"

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

// convertMisrenderedListsInCode detects <pre><code class="language-text"> blocks whose content
// consists entirely of list items and converts them to proper nested HTML lists.
func convertMisrenderedListsInCode(htmlContent string) string {
	re := regexp.MustCompile(`(?s)<pre[^>]*><code[^>]*class="language-text"[^>]*>(.*?)</code></pre>`)
	listLineRe := regexp.MustCompile(`^( *)([-*]) (.*)$`)
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")

	return re.ReplaceAllStringFunc(htmlContent, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		lines := strings.Split(strings.TrimRight(m[1], "\n"), "\n")

		// check all non-empty lines are list items
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !listLineRe.MatchString(line) {
				return match
			}
		}

		var buf strings.Builder
		depth := 0 // current open <ul> count

		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lm := listLineRe.FindStringSubmatch(line)
			if lm == nil {
				continue
			}
			indent := len(lm[1])
			level := indent/2 + 1 // 1-based nesting level

			for depth < level {
				buf.WriteString("<ul>\n")
				depth++
			}
			for depth > level {
				buf.WriteString("</ul>\n")
				depth--
			}

			content := inlineCodeRe.ReplaceAllString(lm[3], "<code>$1</code>")
			fmt.Fprintf(&buf, "<li>%s</li>\n", content)
		}
		for depth > 0 {
			buf.WriteString("</ul>\n")
			depth--
		}
		return buf.String()
	})
}

type codeBlock struct{ lang, content string }

// extractCodeBlocks replaces fenced code blocks with placeholders before markdown parsing
// to prevent misinterpretation (e.g. # comments becoming headers, list items becoming lists).
// Call restoreCodeBlocks with the returned blocks after rendering.
func (h *MarkdownHandler) extractCodeBlocks(content []byte) ([]byte, []codeBlock) {
	var blocks []codeBlock
	fenceRe := regexp.MustCompile("(?s)```([^\n]*)\n(.*?)```")
	result := fenceRe.ReplaceAllStringFunc(string(content), func(match string) string {
		m := fenceRe.FindStringSubmatch(match)
		lang := strings.TrimSpace(m[1])
		if lang == "" {
			lang = "text"
		}
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", len(blocks))
		blocks = append(blocks, codeBlock{lang, m[2]})
		return "\n" + placeholder + "\n"
	})
	return []byte(result), blocks
}

// restoreCodeBlocks replaces placeholders with syntax-highlighted code blocks.
// Iterates in reverse to avoid substring collisions (e.g. KNOVCODEBLOCK1 inside KNOVCODEBLOCK10).
func (h *MarkdownHandler) restoreCodeBlocks(html string, blocks []codeBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", i)
		highlighted := HighlightCodeBlock(blocks[i].content, blocks[i].lang)
		html = strings.ReplaceAll(html, "<p>"+placeholder+"</p>", highlighted)
		html = strings.ReplaceAll(html, placeholder, highlighted)
	}
	return html
}

// resolveMediaPath returns a clean relative media path from a markdown image destination.
// Falls back to bare filename if it has a known image extension (e.g. "photo.png" without prefix).
func resolveMediaPath(dest string) string {
	if pathutils.IsMedia(dest) {
		return pathutils.ToRelative(dest)
	}
	// fallback: bare image filename with no media/ prefix
	if configmanager.IsImageExtension(strings.ToLower(filepath.Ext(dest))) {
		return dest
	}
	return ""
}

// buildRenderer creates the HTML renderer with hooks for code highlighting,
// table edit buttons, and media image previews.
func (h *MarkdownHandler) buildRenderer(filePath string) *html.Renderer {
	opts := html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			if code, ok := node.(*ast.CodeBlock); ok && entering {
				lang := string(code.Info)
				if lang == "" {
					lang = "text"
				}
				fmt.Fprintf(w, "%s", HighlightCodeBlock(string(code.Literal), lang))
				return ast.GoToNext, true
			}

			if _, ok := node.(*ast.Table); ok && !entering {
				fmt.Fprintf(w, `</table><div class="table-edit-wrapper"><a href="/files/edittable/%s" class="btn-table-edit"><i class="fa fa-edit"></i> %s</a></div>`,
					filePath, translation.SprintfForRequest(configmanager.GetLanguage(), "edit table"))
				return ast.GoToNext, true
			}

			if img, ok := node.(*ast.Image); ok && entering {
				mediaPath := resolveMediaPath(string(img.Destination))
				if mediaPath == "" {
					return ast.GoToNext, false
				}
				if configmanager.GetPreviewsEnabled() {
					size := configmanager.GetDefaultPreviewSize()
					containerTag, containerClass := "div", "media-preview-container"
					if configmanager.GetDisplayMode() == "inline" {
						containerTag, containerClass = "span", containerClass+" inline-container"
					}
					fmt.Fprintf(w, `<%s class="%s" hx-get="/api/media/preview?path=%s&size=%d" hx-trigger="load" hx-swap="innerHTML">%s...</%s>`,
						containerTag, containerClass, mediaPath, size,
						translation.SprintfForRequest(configmanager.GetLanguage(), "loading media"), containerTag)
				} else {
					fmt.Fprintf(w, `<img src="/media/%s" alt="%s" />`, mediaPath, filepath.Base(mediaPath))
				}
				return ast.SkipChildren, true
			}

			return ast.GoToNext, false
		},
	}
	return html.NewRenderer(opts)
}

// postProcessHTML applies all HTML transformations after markdown rendering:
// header edit buttons, misrendered list fixes, list paragraph cleanup, and section wrapping.
func (h *MarkdownHandler) postProcessHTML(html, filePath string) string {
	html = h.addHeaderButtons(html, filePath)
	html = convertMisrenderedListsInCode(html)
	html = h.cleanupListParagraphs(html)
	html = h.wrapHeaderSections(html)
	return html
}

func (h *MarkdownHandler) Render(content []byte, filePath string) ([]byte, error) {
	extensions := gomarkdown_parser.CommonExtensions | gomarkdown_parser.AutoHeadingIDs | gomarkdown_parser.HardLineBreak
	extensions &^= gomarkdown_parser.MathJax // prevent $ from being treated as math delimiters
	p := gomarkdown_parser.NewWithExtensions(extensions)

	content, blocks := h.extractCodeBlocks(content)
	content = h.preprocessBlankLineLists(content)

	result := h.restoreCodeBlocks(string(markdown.ToHTML(content, p, h.buildRenderer(filePath))), blocks)

	return []byte(h.postProcessHTML(result, filePath)), nil
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

// wrapHeaderSections wraps content between headers in div elements
func (h *MarkdownHandler) wrapHeaderSections(htmlContent string) string {
	// find all headers and their positions
	headerRegex := regexp.MustCompile(`<h([1-6])[^>]*>.*?</h[1-6]>`)
	headerMatches := headerRegex.FindAllStringIndex(htmlContent, -1)

	if len(headerMatches) == 0 {
		// no headers found, wrap entire content
		return fmt.Sprintf(`<div class="content-section">%s</div>`, htmlContent)
	}

	var result strings.Builder

	// add content before first header if any
	if headerMatches[0][0] > 0 {
		beforeFirstHeader := strings.TrimSpace(htmlContent[0:headerMatches[0][0]])
		if beforeFirstHeader != "" {
			result.WriteString(fmt.Sprintf(`<div class="content-section">%s</div>`, beforeFirstHeader))
		}
	}

	// process each header and its content
	for i, match := range headerMatches {
		// add the header itself
		header := htmlContent[match[0]:match[1]]
		result.WriteString(header)

		// find content after this header (until next header or end)
		contentStart := match[1]
		var contentEnd int
		if i+1 < len(headerMatches) {
			contentEnd = headerMatches[i+1][0] // until next header
		} else {
			contentEnd = len(htmlContent) // until end of content
		}

		// extract and wrap the content section
		sectionContent := strings.TrimSpace(htmlContent[contentStart:contentEnd])
		if sectionContent != "" {
			result.WriteString(fmt.Sprintf(`<div class="content-section">%s</div>`, sectionContent))
		}
	}

	return result.String()
}

// cleanupListParagraphs removes unnecessary paragraph tags from list items
// Converts <li><p>content</p></li> to <li>content</li> for cleaner list styling
func (h *MarkdownHandler) cleanupListParagraphs(htmlContent string) string {
	// Handle replacements in the right order: most specific to most general

	// Step 1: Handle </p> that comes after <br> and whitespace before </li>
	htmlContent = regexp.MustCompile(`<br>\s*</p>\s*</li>`).ReplaceAllString(htmlContent, `<br></li>`)

	// Step 2: Handle </p> followed by whitespace before </li>
	htmlContent = regexp.MustCompile(`</p>\s*</li>`).ReplaceAllString(htmlContent, `</li>`)

	// Step 3: Handle </p> when followed by nested content like <ul>
	htmlContent = regexp.MustCompile(`</p>(\s*<ul>)`).ReplaceAllString(htmlContent, `$1`)

	// Step 4: Handle opening <li><p> pattern
	htmlContent = regexp.MustCompile(`<li><p>`).ReplaceAllString(htmlContent, `<li>`)

	return htmlContent
}

func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	var links []string
	text := string(content)

	// remove code blocks to avoid extracting links from code
	text = removeCodeBlocks(text)

	// extract standard markdown links [text](url) - skip images (handled separately)
	mdLinkRegex := regexp.MustCompile(`[^!]\[([^\]]+)\]\(([^\)]+)\)`)
	mdMatches := mdLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range mdMatches {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "#") {
				links = append(links, link)
			}
		}
	}

	// extract image links ![alt](url)
	imgLinkRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`)
	imgMatches := imgLinkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range imgMatches {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
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

// preprocessBlankLineLists injects HTML comment separators between top-level lists divided by blank lines,
// working around gomarkdown merging them into a single list.
// Only top-level items (no leading spaces) get separators — nested items at 4+ spaces must
// never have their context reset or gomarkdown interprets them as indented code blocks.
func (h *MarkdownHandler) preprocessBlankLineLists(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	topLevelItemRe := regexp.MustCompile(`^[-*] `)
	var inCodeBlock bool

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
		}
		result = append(result, line)
		if !inCodeBlock && topLevelItemRe.MatchString(line) && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			result = append(result, "", "<!-- -->", "")
		}
	}

	return []byte(strings.Join(result, "\n"))
}

// processMarkdownLinks converts markdown [text](url) links to HTML anchors
func (h *MarkdownHandler) processMarkdownLinks(content string) string {
	re := regexp.MustCompile(`(!)?\[([^\]]+)\]\(([^)]+)\)`)
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 4 {
			return match
		}

		isImage := matches[1] == "!"
		text := strings.TrimSpace(matches[2])
		url := strings.TrimSpace(matches[3])

		// images are handled by the render hook - return as-is
		if isImage {
			return match
		}

		if !strings.Contains(url, "://") && !strings.HasPrefix(url, "#") {
			// convert /files/media/ links to /media/
			if strings.HasPrefix(url, "/files/media/") {
				return `<a href="/media/` + url[len("/files/media/"):] + `">` + text + `</a>`
			}
			// already has /files/ prefix
			if strings.HasPrefix(url, "/files/") {
				return `<a href="` + url + `">` + text + `</a>`
			}
			if !strings.HasSuffix(url, ".md") {
				url += ".md"
			}
			return `<a href="/files/` + url + `">` + text + `</a>`
		}

		return `<a href="` + url + `">` + text + `</a>`
	})

	return content
}
