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
				if strings.HasPrefix(dest, "media/") {
					mediaPath = dest[6:] // remove "media/" prefix
				} else if strings.HasPrefix(dest, "/media/") {
					mediaPath = dest[7:] // remove "/media/" prefix
				} else {
					// Check if it's a common image extension that should be treated as media
					ext := strings.ToLower(filepath.Ext(dest))
					if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
						// Assume it's a media file if it's an image extension
						mediaPath = dest
					}
				}

				// If we identified it as a media file, handle it
				if mediaPath != "" {
					if entering {
						// Check if previews are enabled
						if configmanager.GetPreviewsEnabled() {
							// Get default preview size from settings
							size := configmanager.GetDefaultPreviewSize()

							// Add inline-container class for inline display mode
							containerClass := "media-preview-container"
							if configmanager.GetDisplayMode() == "inline" {
								containerClass += " inline-container"
							}

							// Create HTMX preview element instead of regular image
							previewHTML := fmt.Sprintf(`<div class="%s" hx-get="/api/media/preview?path=%s&size=%d" hx-trigger="load" hx-swap="innerHTML">%s...</div>`,
								containerClass, mediaPath, size, translation.SprintfForRequest(configmanager.GetLanguage(), "loading media"))

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
			if !strings.HasSuffix(url, ".md") {
				url += ".md"
			}
			return `<a href="/files/` + url + `">` + text + `</a>`
		}

		return `<a href="` + url + `">` + text + `</a>`
	})

	return content
}
