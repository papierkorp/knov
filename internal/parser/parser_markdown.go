package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/configmanager"
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
	processed = h.fixIndentedListsInCodeBlocks(processed)
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
	extensions := gomarkdown_parser.CommonExtensions | gomarkdown_parser.AutoHeadingIDs | gomarkdown_parser.HardLineBreak
	extensions &^= gomarkdown_parser.MathJax // prevent $ signs from being treated as math delimiters
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

			// Handle media images - convert to preview API calls
			if img, ok := node.(*ast.Image); ok {
				dest := string(img.Destination)
				var mediaPath string

				// Determine if this is a media reference and extract the path
				if strings.HasPrefix(dest, "/files/media/") {
					mediaPath = dest[len("/files/media/"):] // remove "/files/media/" prefix
				} else if strings.HasPrefix(dest, "media/") {
					mediaPath = dest[6:] // remove "media/" prefix
				} else if strings.HasPrefix(dest, "/media/") {
					mediaPath = dest[7:] // remove "/media/" prefix
				} else if !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
					ext := strings.ToLower(filepath.Ext(dest))
					if configmanager.IsImageExtension(ext) {
						mediaPath = dest
					}
				}

				// If we identified it as a media file, handle it
				if mediaPath != "" {
					if entering {
						if configmanager.GetPreviewsEnabled() {
							// Get default preview size from settings
							size := configmanager.GetDefaultPreviewSize()
							displayMode := configmanager.GetDisplayMode()

							// Use span for inline mode to keep text flow, div for other modes
							containerTag := "div"
							containerClass := "media-preview-container"
							if displayMode == "inline" {
								containerTag = "span"
								containerClass += " inline-container"
							}

							// Create HTMX preview element instead of regular image
							previewHTML := fmt.Sprintf(`<%s class="%s" hx-get="/api/media/preview?path=%s&size=%d" hx-trigger="load" hx-swap="innerHTML">%s...</%s>`,
								containerTag, containerClass, mediaPath, size, translation.SprintfForRequest(configmanager.GetLanguage(), "loading media"), containerTag)

							w.Write([]byte(previewHTML))
						} else {
							// Previews disabled, render direct image link
							fmt.Fprintf(w, `<img src="/media/%s" alt="%s" />`, mediaPath, filepath.Base(mediaPath))
						}
					}
					// Skip children to prevent alt text from being rendered separately
					return ast.SkipChildren, true
				}
			}

			return ast.GoToNext, false
		},
	}
	renderer := html.NewRenderer(opts)

	// gomarkdown merges lists separated by blank lines - inject comment separators to prevent this
	content = h.preprocessBlankLineLists(content)

	htmlOutput := markdown.ToHTML(content, p, renderer)

	// Post-process to add header edit buttons outside the header tags
	processedHTML := h.addHeaderButtons(string(htmlOutput), filePath)

	// Remove unnecessary p tags from list items
	processedHTML = h.cleanupListParagraphs(processedHTML)

	// Wrap content between headers in sections
	processedHTML = h.wrapHeaderSections(processedHTML)

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

// fixIndentedListsInCodeBlocks ensures code blocks nested inside lists maintain correct indentation,
// working around gomarkdown's behaviour of losing list context after a fenced code block
func (h *MarkdownHandler) fixIndentedListsInCodeBlocks(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inCodeBlock bool
	var codeBlockIndent string

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				inCodeBlock = true
				for j := len(result) - 1; j >= 0; j-- {
					prev := result[j]
					if strings.TrimSpace(prev) == "" {
						continue
					}
					if match := regexp.MustCompile(`^( *)- (.+)$`).FindStringSubmatch(prev); match != nil {
						codeBlockIndent = match[1] + "  "
					}
					break
				}
				if codeBlockIndent != "" {
					result = append(result, codeBlockIndent+strings.TrimSpace(line))
				} else {
					result = append(result, line)
				}
			} else {
				if codeBlockIndent != "" {
					result = append(result, codeBlockIndent+strings.TrimSpace(line))
				} else {
					result = append(result, line)
				}
				inCodeBlock = false
				codeBlockIndent = ""
			}
			continue
		}
		if inCodeBlock && codeBlockIndent != "" {
			if strings.TrimSpace(line) == "" {
				result = append(result, "")
			} else {
				result = append(result, codeBlockIndent+strings.TrimSpace(line))
			}
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// preprocessBlankLineLists injects HTML comment separators between lists divided by blank lines,
// working around gomarkdown merging them into a single list
func (h *MarkdownHandler) preprocessBlankLineLists(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	listItemRe := regexp.MustCompile(`^( *)- (.+)$`)

	for i, line := range lines {
		result = append(result, line)
		if listItemRe.MatchString(line) && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
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
