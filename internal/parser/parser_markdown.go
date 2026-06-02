package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/pathutils"
	"knov/internal/translation"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

type MarkdownHandler struct{}

func NewMarkdownHandler() *MarkdownHandler {
	return &MarkdownHandler{}
}

func (h *MarkdownHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".index" || ext == ".moc" || ext == ".list" || ext == ".todo"
}

func (h *MarkdownHandler) Parse(content []byte) ([]byte, error) {
	content = StripFrontMatter(content)
	processed := h.processMarkdownLinks(string(content))
	return []byte(processed), nil
}

func (h *MarkdownHandler) Render(content []byte, filePath string) ([]byte, error) {
	content, blocks := h.extractCodeBlocks(content)

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(newKnovNodeRenderer(filePath, blocks), 1),
			),
		),
	)

	var buf bytes.Buffer
	source := []byte(content)
	if err := md.Convert(source, &buf); err != nil {
		return nil, err
	}

	result := buf.String()
	result = h.restoreOrphanCodeBlocks(result, blocks)
	result = h.addHeaderButtons(result, filePath)
	result = h.wrapHeaderSections(result)
	return []byte(result), nil
}

// ---------------------------------------------------------------------------
// Custom node renderer — handles code blocks (chroma), tables (HTMX), images
// ---------------------------------------------------------------------------

type knovNodeRenderer struct {
	filePath string
	blocks   []codeBlock
	tableIdx int
	html.Config
}

func newKnovNodeRenderer(filePath string, blocks []codeBlock) renderer.NodeRenderer {
	return &knovNodeRenderer{
		filePath: filePath,
		blocks:   blocks,
		Config:   html.NewConfig(),
	}
}

func (r *knovNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(extast.KindTable, r.renderTable)
	reg.Register(ast.KindImage, r.renderImage)
}

func (r *knovNodeRenderer) renderFencedCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.FencedCodeBlock)
	lang := "text"
	if info := n.Info; info != nil {
		tag := strings.TrimSpace(string(info.Segment.Value(source)))
		if tag != "" {
			lang = tag
		}
	}

	// check if this is a placeholder — restore from blocks slice
	var content string
	var buf bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	raw := buf.String()

	placeholder := strings.TrimSpace(raw)
	if strings.HasPrefix(placeholder, "KNOVCODEBLOCK") {
		idx := 0
		fmt.Sscanf(placeholder[len("KNOVCODEBLOCK"):], "%d", &idx)
		if idx < len(r.blocks) {
			r.blocks[idx].rendered = true
			fmt.Fprintf(w, "%s", HighlightCodeBlock(r.blocks[idx].content, r.blocks[idx].lang))
			return ast.WalkSkipChildren, nil
		}
	}

	content = raw
	fmt.Fprintf(w, "%s", HighlightCodeBlock(content, lang))
	return ast.WalkSkipChildren, nil
}

func (r *knovNodeRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	var buf bytes.Buffer
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	fmt.Fprintf(w, "%s", HighlightCodeBlock(buf.String(), "text"))
	return ast.WalkSkipChildren, nil
}

func (r *knovNodeRenderer) renderTable(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	relPath := pathutils.ToRelative(r.filePath)
	fmt.Fprintf(w,
		`<div id="table-component-%d" hx-get="/api/components/table?filepath=%s&tableindex=%d" hx-trigger="load" hx-swap="outerHTML"></div>`,
		r.tableIdx, url.QueryEscape(relPath), r.tableIdx,
	)
	r.tableIdx++
	return ast.WalkSkipChildren, nil
}

func (r *knovNodeRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.Image)
	dest := string(n.Destination)
	isExternal := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	var altBuf bytes.Buffer
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			altBuf.Write(t.Segment.Value(source))
		}
	}
	alt := altBuf.String()

	if configmanager.GetPreviewsEnabled() && !isExternal {
		previewPath := resolveMediaPath(dest)
		if previewPath == "" {
			return ast.WalkSkipChildren, nil
		}
		size := configmanager.GetDefaultPreviewSize()
		containerTag, containerClass := "div", "media-preview-container"
		if configmanager.GetDisplayMode() == "inline" {
			containerTag, containerClass = "span", containerClass+" inline-container"
		}
		fmt.Fprintf(w, `<%s class="%s" hx-get="/api/media/preview?path=%s&size=%d" hx-trigger="load" hx-swap="innerHTML">%s...</%s>`,
			containerTag, containerClass, url.QueryEscape(previewPath), size,
			translation.SprintfForRequest(configmanager.GetLanguage(), "loading media"), containerTag)
		return ast.WalkSkipChildren, nil
	}

	if isExternal {
		fmt.Fprintf(w, `<img src="%s" alt="%s" />`, dest, alt)
	} else {
		previewPath := resolveMediaPath(dest)
		if previewPath != "" {
			fmt.Fprintf(w, `<img src="/media/%s" alt="%s" />`, previewPath, alt)
		} else {
			fmt.Fprintf(w, `<img src="%s" alt="%s" />`, dest, alt)
		}
	}
	return ast.WalkSkipChildren, nil
}

// ---------------------------------------------------------------------------
// Code block extract/restore (still needed for chroma inside lists)
// ---------------------------------------------------------------------------

type codeBlock struct {
	lang     string
	content  string
	rendered bool
}

// extractCodeBlocks replaces fenced code blocks with KNOVCODEBLOCK<n> placeholders
// so chroma handles highlighting and goldmark never sees the raw code content.
func (h *MarkdownHandler) extractCodeBlocks(content []byte) ([]byte, []codeBlock) {
	var blocks []codeBlock
	fenceRe := regexp.MustCompile("(?m)^([ \t]*)```([^\n]*)\n")
	lines := strings.Split(string(content), "\n")
	var result []string
	i := 0
	for i < len(lines) {
		m := fenceRe.FindStringSubmatch(lines[i] + "\n")
		if m == nil {
			result = append(result, lines[i])
			i++
			continue
		}
		indent := m[1]
		lang := strings.TrimSpace(m[2])
		if lang == "" {
			lang = "text"
		}
		i++
		var contentLines []string
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "```" {
				i++
				break
			}
			contentLines = append(contentLines, lines[i])
			i++
		}
		rawContent := strings.Join(contentLines, "\n") + "\n"
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", len(blocks))
		blocks = append(blocks, codeBlock{lang: lang, content: rawContent})
		// emit with blank lines so goldmark sees a proper fenced block
		result = append(result, "", indent+"```", indent+placeholder, indent+"```", "")
	}
	return []byte(strings.Join(result, "\n")), blocks
}

// restoreOrphanCodeBlocks replaces any KNOVCODEBLOCK placeholder that the node renderer
// did not handle (e.g. inside a <p> tag due to unusual nesting) with highlighted HTML.
func (h *MarkdownHandler) restoreOrphanCodeBlocks(html string, blocks []codeBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].rendered {
			continue
		}
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", i)
		highlighted := HighlightCodeBlock(blocks[i].content, blocks[i].lang)
		html = strings.ReplaceAll(html, "<p>"+placeholder+"</p>", highlighted)
		html = strings.ReplaceAll(html, placeholder, highlighted)
	}
	return html
}

// ---------------------------------------------------------------------------
// Post-processing (kept as-is — parser-agnostic custom features)
// ---------------------------------------------------------------------------

// resolveMediaPath returns a clean relative media path from a markdown image destination.
func resolveMediaPath(dest string) string {
	if pathutils.IsMedia(dest) {
		return pathutils.ToRelative(dest)
	}
	if configmanager.IsImageExtension(strings.ToLower(filepath.Ext(dest))) {
		return dest
	}
	return ""
}

// addHeaderButtons injects edit-section anchor buttons into every header tag.
func (h *MarkdownHandler) addHeaderButtons(htmlContent, filePath string) string {
	headerRe := regexp.MustCompile(`<h([1-6])\s+id="([^"]+)"[^>]*>(.*?)</h[1-6]>`)
	return headerRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := headerRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		editBtn := fmt.Sprintf(
			`<a href="/files/edit/%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
			filePath, parts[2],
			translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
		)
		return fmt.Sprintf(`<h%s id="%s">%s%s</h%s>`, parts[1], parts[2], parts[3], editBtn, parts[1])
	})
}

// wrapHeaderSections wraps content between headers in <div class="content-section">.
func (h *MarkdownHandler) wrapHeaderSections(htmlContent string) string {
	headerRe := regexp.MustCompile(`<h([1-6])[^>]*>.*?</h[1-6]>`)
	matches := headerRe.FindAllStringIndex(htmlContent, -1)

	if len(matches) == 0 {
		return fmt.Sprintf(`<div class="content-section">%s</div>`, htmlContent)
	}

	var out strings.Builder
	if matches[0][0] > 0 {
		before := strings.TrimSpace(htmlContent[:matches[0][0]])
		if before != "" {
			fmt.Fprintf(&out, `<div class="content-section">%s</div>`, before)
		}
	}
	for i, match := range matches {
		out.WriteString(htmlContent[match[0]:match[1]])
		start := match[1]
		end := len(htmlContent)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := strings.TrimSpace(htmlContent[start:end])
		if section != "" {
			fmt.Fprintf(&out, `<div class="content-section">%s</div>`, section)
		}
	}
	return out.String()
}

// ---------------------------------------------------------------------------
// Link processing and helpers
// ---------------------------------------------------------------------------

// processMarkdownLinks rewrites internal [text](url) links to /files/ routes.
func (h *MarkdownHandler) processMarkdownLinks(content string) string {
	re := regexp.MustCompile(`(!)?\[([^\]]+)\]\(([^)]+)\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 4 {
			return match
		}
		isImage := matches[1] == "!"
		text := strings.TrimSpace(matches[2])
		u := strings.TrimSpace(matches[3])

		if isImage {
			return matches[1] + "[" + text + "](" + strings.ReplaceAll(u, "\\", "/") + ")"
		}
		if !strings.Contains(u, "://") && !strings.HasPrefix(u, "#") {
			if strings.HasPrefix(u, "/files/media/") {
				return `<a href="/media/` + u[len("/files/media/"):] + `">` + text + `</a>`
			}
			if strings.HasPrefix(u, "/files/") {
				return `<a href="` + u + `">` + text + `</a>`
			}
			if strings.HasPrefix(u, "media/") {
				return `<a href="/` + u + `?mode=detail">` + text + `</a>`
			}
			if !strings.HasSuffix(u, ".md") {
				u += ".md"
			}
			return `<a href="/files/` + u + `">` + text + `</a>`
		}
		return `<a href="` + u + `">` + text + `</a>`
	})
}

func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	var links []string
	text := string(content)
	text = removeCodeBlocks(text)

	mdLinkRegex := regexp.MustCompile(`[^!]\[([^\]]+)\]\(([^\)]+)\)`)
	for _, match := range mdLinkRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "#") {
				links = append(links, link)
			}
		}
	}

	imgLinkRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`)
	for _, match := range imgLinkRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			link := strings.TrimSpace(match[2])
			if link != "" && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
				links = append(links, link)
			}
		}
	}
	return links
}

func removeCodeBlocks(text string) string {
	// split on ``` fence markers so each block is removed independently (no greedy cross-block matching)
	parts := strings.Split(text, "```")
	var result strings.Builder
	for i, part := range parts {
		if i%2 == 0 {
			// outside a code block — strip inline code then keep
			result.WriteString(regexp.MustCompile("`[^`\n]+`").ReplaceAllString(part, ""))
		}
		// odd-indexed parts are inside fenced code blocks — discard
	}
	return result.String()
}

func (h *MarkdownHandler) Name() string {
	return "markdown"
}
