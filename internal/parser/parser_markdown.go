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
	processed = h.fixDeeplyIndentedLists(processed)
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

	// Pre-process content to handle lists with DokuWiki-style behavior (blank lines break lists)
	content = h.preprocessListsForDokuWikiStyle(content)

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

// fixDeeplyIndentedLists converts lines with 4+ spaces + - to proper markdown list format
// and ensures code blocks are properly indented within lists
func (h *MarkdownHandler) fixDeeplyIndentedLists(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inCodeBlock bool
	var codeBlockIndent string

	for _, line := range lines {
		// check if we're dealing with a code block fence
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				// entering code block - check previous lines for list context
				inCodeBlock = true
				for j := len(result) - 1; j >= 0; j-- {
					prevLine := result[j]
					if strings.TrimSpace(prevLine) == "" {
						continue // skip empty lines
					}
					// find the last list item to determine indentation
					if match := regexp.MustCompile(`^( *)- (.+)$`).FindStringSubmatch(prevLine); match != nil {
						// for a list item like "    - content", content underneath should be indented "      " (list indent + 2)
						listIndent := match[1]
						codeBlockIndent = listIndent + "  "
						break
					}
					break // stop if we hit non-list content
				}

				if codeBlockIndent != "" {
					result = append(result, codeBlockIndent+strings.TrimSpace(line))
				} else {
					result = append(result, line)
				}
			} else {
				// exiting code block
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

		// if we're in a code block, maintain the indentation
		if inCodeBlock && codeBlockIndent != "" {
			if strings.TrimSpace(line) == "" {
				result = append(result, "")
			} else {
				result = append(result, codeBlockIndent+strings.TrimSpace(line))
			}
			continue
		}

		// check if line has spaces followed by - (list item)
		if match := regexp.MustCompile(`^( *)- (.+)$`).FindStringSubmatch(line); match != nil {
			spaces := match[1]
			listContent := match[2]
			spaceCount := len(spaces)

			// convert DokuWiki indentation to proper markdown indentation
			// 8 spaces → 6 spaces (level 4)
			// 6 spaces → 4 spaces (level 3)
			// 4 spaces → 2 spaces (level 2)
			// 2 spaces → 0 spaces (level 1)
			var newIndent string
			if spaceCount >= 8 {
				newIndent = "      " // 6 spaces for level 3
			} else if spaceCount >= 6 {
				newIndent = "    " // 4 spaces for level 2
			} else if spaceCount >= 4 {
				newIndent = "  " // 2 spaces for level 1
			} else if spaceCount >= 2 {
				newIndent = "  " // keep 2 spaces for level 1 (don't convert to 0)
			}

			result = append(result, fmt.Sprintf("%s- %s", newIndent, listContent))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
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
			return fmt.Sprintf(`<a href="/%s">%s</a>`, link, filepath.Base(link))
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
			return `<a href="/` + url + `">` + text + `</a>`
		}

		if !strings.Contains(url, "://") && !strings.HasPrefix(url, "#") {
			// convert /files/media/ links to /media/
			if strings.HasPrefix(url, "/files/media/") {
				return `<a href="/media/` + url[len("/files/media/"):] + `">` + text + `</a>`
			}
			// check if URL already has /files/ prefix to avoid duplicates
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

// preprocessListsForDokuWikiStyle modifies markdown to force list separation on blank lines
// This uses the DokuWiki logic: any blank line should break lists
func (h *MarkdownHandler) preprocessListsForDokuWikiStyle(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string

	for i, line := range lines {
		result = append(result, line)

		// Check if current line is a list item at any level
		if match := regexp.MustCompile(`^( *)- (.+)$`).FindStringSubmatch(line); match != nil {
			// Look ahead: current list item -> blank line -> next line
			if i+2 < len(lines) {
				nextLine := lines[i+1]
				// lineAfterNext := lines[i+2]

				// If next line is blank, and line after that is also a list item or any content
				if strings.TrimSpace(nextLine) == "" {
					// Insert an HTML comment to force separation
					// This will cause gomarkdown to end the current list
					result = append(result, "")
					result = append(result, "<!-- -->")
					result = append(result, "")
				}
			}
		}
	}

	return []byte(strings.Join(result, "\n"))
}
