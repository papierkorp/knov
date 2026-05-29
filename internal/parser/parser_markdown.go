package parser

import (
	"fmt"
	"io"
	"net/url"
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

func (h *MarkdownHandler) Name() string { return "markdown" }

func (h *MarkdownHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".index" || ext == ".moc" || ext == ".list" || ext == ".todo"
}

func (h *MarkdownHandler) Parse(content []byte) ([]byte, error) {
	processed := h.processMarkdownLinks(string(content))
	return []byte(processed), nil
}

// Render converts parsed markdown bytes to HTML using a three-stage pipeline:
//
//  1. Pre-processing  — transform the markdown source before the parser sees it
//  2. Parsing         — gomarkdown converts markdown to raw HTML
//  3. Post-processing — transform the raw HTML into the final output
func (h *MarkdownHandler) Render(content []byte, filePath string) ([]byte, error) {
	// --- pre-processing ---
	content, codeBlocks := h.preExtractCodeBlocks(content) // must be first: protects code from all later steps
	content = h.preSeparateLists(content)                  // fix gomarkdown list-merging and ol-after-paragraph

	// --- parsing ---
	extensions := gomarkdown_parser.CommonExtensions | gomarkdown_parser.AutoHeadingIDs | gomarkdown_parser.HardLineBreak
	extensions &^= gomarkdown_parser.MathJax
	p := gomarkdown_parser.NewWithExtensions(extensions)
	raw := string(markdown.ToHTML(content, p, h.buildRenderer(filePath)))

	// --- post-processing ---
	result := h.postRestoreCodeBlocks(raw, codeBlocks) // must be first: restores code before other steps inspect HTML
	result = h.postAddHeaderButtons(result, filePath)  // inject edit + anchor buttons into <hN> tags
	result = postFixMisrenderedListsInCode(result)     // convert gomarkdown code blocks that are actually lists
	result = h.postCleanupListParagraphs(result)       // strip spurious <p> inside <li>
	result = h.postWrapHeaderSections(result)          // wrap content between headers in <div class="content-section">

	return []byte(result), nil
}

// ---------------------------------------------------------------------------
// Renderer
// ---------------------------------------------------------------------------

type codeBlock struct{ lang, content string }

// buildRenderer creates the gomarkdown HTML renderer with hooks for:
//   - code block syntax highlighting
//   - table → HTMX component replacement
//   - image → media preview or direct <img>
func (h *MarkdownHandler) buildRenderer(filePath string) *html.Renderer {
	tableIdx := 0
	relPath := pathutils.ToRelative(filePath)

	opts := html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			// code blocks — syntax highlight via chroma
			if code, ok := node.(*ast.CodeBlock); ok && entering {
				lang := string(code.Info)
				if lang == "" {
					lang = "text"
				}
				fmt.Fprintf(w, "%s", HighlightCodeBlock(string(code.Literal), lang))
				return ast.GoToNext, true
			}

			// tables — replace with HTMX-powered interactive component
			if _, ok := node.(*ast.Table); ok {
				if entering {
					fmt.Fprintf(w,
						`<div id="table-component-%d" hx-get="/api/components/table?filepath=%s&tableindex=%d" hx-trigger="load" hx-swap="outerHTML"></div>`,
						tableIdx, url.QueryEscape(relPath), tableIdx)
					tableIdx++
					return ast.SkipChildren, true
				}
				return ast.GoToNext, true
			}

			// images — media preview or direct img tag
			if img, ok := node.(*ast.Image); ok {
				dest := string(img.Destination)
				isExternal := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

				previewPath := dest
				if !isExternal {
					previewPath = resolveMediaPath(dest)
					if previewPath == "" {
						return ast.GoToNext, false
					}
				}

				if entering {
					if configmanager.GetPreviewsEnabled() {
						size := configmanager.GetDefaultPreviewSize()
						containerTag, containerClass := "div", "media-preview-container"
						if configmanager.GetDisplayMode() == "inline" {
							containerTag, containerClass = "span", containerClass+" inline-container"
						}
						fmt.Fprintf(w,
							`<%s class="%s" hx-get="/api/media/preview?path=%s&size=%d" hx-trigger="load" hx-swap="innerHTML">%s...</%s>`,
							containerTag, containerClass, previewPath, size,
							translation.SprintfForRequest(configmanager.GetLanguage(), "loading media"),
							containerTag)
					} else {
						if isExternal {
							fmt.Fprintf(w, `<img src="%s" alt="%s" />`, dest, filepath.Base(dest))
						} else {
							fmt.Fprintf(w, `<img src="/media/%s" alt="%s" />`, previewPath, filepath.Base(previewPath))
						}
					}
					return ast.SkipChildren, true
				}
				return ast.GoToNext, true
			}

			return ast.GoToNext, false
		},
	}
	return html.NewRenderer(opts)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveMediaPath returns a clean relative media path from a markdown image
// destination. Falls back to bare filename for known image extensions.
func resolveMediaPath(dest string) string {
	if pathutils.IsMedia(dest) {
		return pathutils.ToRelative(dest)
	}
	if configmanager.IsImageExtension(strings.ToLower(filepath.Ext(dest))) {
		return dest
	}
	return ""
}

// processMarkdownLinks converts internal markdown [text](url) links to HTML
// anchors with the correct /files/ prefix. External links and images are left
// unchanged (images are handled by the renderer hook).
func (h *MarkdownHandler) processMarkdownLinks(content string) string {
	re := regexp.MustCompile(`(!)?\[([^\]]+)\]\(([^)]+)\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 4 {
			return match
		}
		if matches[1] == "!" { // image — renderer hook handles it
			return match
		}
		text := strings.TrimSpace(matches[2])
		u := strings.TrimSpace(matches[3])

		if strings.Contains(u, "://") || strings.HasPrefix(u, "#") {
			return `<a href="` + u + `">` + text + `</a>`
		}
		if strings.HasPrefix(u, "/files/media/") {
			return `<a href="/media/` + u[len("/files/media/"):] + `">` + text + `</a>`
		}
		if strings.HasPrefix(u, "/files/") {
			return `<a href="` + u + `">` + text + `</a>`
		}
		if !strings.HasSuffix(u, ".md") {
			u += ".md"
		}
		return `<a href="/files/` + u + `">` + text + `</a>`
	})
}

// ExtractLinks returns all internal (non-http) links found in the markdown content.
func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	text := removeCodeBlocks(string(content))
	var links []string

	mdRe := regexp.MustCompile(`[^!]\[([^\]]+)\]\(([^\)]+)\)`)
	for _, match := range mdRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "#") {
				links = append(links, link)
			}
		}
	}

	imgRe := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`)
	for _, match := range imgRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
				links = append(links, link)
			}
		}
	}
	return links
}

// removeCodeBlocks strips fenced and inline code from markdown before link extraction.
func removeCodeBlocks(text string) string {
	text = regexp.MustCompile("(?s)```[^`]*```").ReplaceAllString(text, "")
	text = regexp.MustCompile("`[^`]*`").ReplaceAllString(text, "")
	return text
}
