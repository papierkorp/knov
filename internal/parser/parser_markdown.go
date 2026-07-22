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
	processed := h.wrapRawHTMLBlocks(string(content))
	processed = h.processWikiLinks(processed)
	processed = h.processMarkdownLinks(processed)
	return []byte(processed), nil
}

var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// processWikiLinks converts [[path]] and [[path|display]] to standard markdown links.
func (h *MarkdownHandler) processWikiLinks(content string) string {
	return wikiLinkRe.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2]
		parts := strings.SplitN(inner, "|", 2)
		linkPath := strings.TrimSpace(parts[0])

		// split off anchor before any path manipulation
		anchor := ""
		if idx := strings.Index(linkPath, "#"); idx != -1 {
			anchor = linkPath[idx:]
			linkPath = linkPath[:idx]
		}

		display := strings.TrimSuffix(filepath.Base(linkPath), ".md")
		if len(parts) == 2 {
			display = strings.TrimSpace(parts[1])
		}
		if !strings.Contains(filepath.Base(linkPath), ".") {
			linkPath += ".md"
		}
		if decoded, err := url.PathUnescape(linkPath); err == nil {
			linkPath = decoded
		}
		return "[" + display + "](" + pathutils.ToFileURL(linkPath) + anchor + ")"
	})
}

// wrapRawHTMLBlocks wraps bare HTML blocks in fenced code blocks so goldmark
// renders them as code instead of silently omitting them.
func (h *MarkdownHandler) wrapRawHTMLBlocks(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// detect start of a bare HTML block (line starts with < and a tag name)
		if strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "<!--") &&
			!strings.HasPrefix(trimmed, "<a ") && !strings.HasPrefix(trimmed, "</a") &&
			htmlBlockRe.MatchString(trimmed) {

			// collect all consecutive lines of the HTML block
			var block []string
			for i < len(lines) {
				block = append(block, lines[i])
				if strings.TrimSpace(lines[i]) == "" && len(block) > 1 {
					break
				}
				i++
			}
			// only wrap if it looks like a multi-tag block
			joined := strings.Join(block, "\n")
			if strings.Count(joined, "<") > 1 {
				result = append(result, "```html")
				result = append(result, strings.TrimRight(joined, "\n"))
				result = append(result, "```")
			} else {
				result = append(result, block...)
			}
			continue
		}

		result = append(result, line)
		i++
	}
	return strings.Join(result, "\n")
}

var htmlBlockRe = regexp.MustCompile(`(?i)^<(html|head|body|div|section|article|header|footer|nav|main|aside|meta|script|style|link|table|form|iframe|p|ul|ol|li|h[1-6]|pre|blockquote)[\s>]`)

func (h *MarkdownHandler) Render(content []byte, filePath string) ([]byte, error) {
	content, blocks := h.extractCodeBlocks(content)
	content, detailsBlocks := h.extractHTMLBlocks(content, "details", "summary")
	content = h.preprocessTodoStates(content)

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),

		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
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
	result = h.restoreHTMLBlocks(result, "details", detailsBlocks)
	result = h.postprocessTodoStates(result)
	result = sanitizeHTML(result)
	result = InjectHeaderIDs(result)
	result = h.addHeaderButtons(result, filePath)
	result = h.wrapHeaderSections(result, filePath)
	return []byte(result), nil
}

// RenderInlineMarkdown renders inline markdown (code, bold, italic, links) to HTML.
// It strips the wrapping <p> tag goldmark adds around single-line input.
func RenderInlineMarkdown(s string) string {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return s
	}
	result := strings.TrimSpace(buf.String())
	result = strings.TrimPrefix(result, "<p>")
	result = strings.TrimSuffix(result, "</p>")
	return strings.TrimSpace(result)
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
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)
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
// HTML wrapper-tag extract/restore — renders a chosen tag's wrapper live
// (sanitized) while leaving the enclosed content to be parsed as normal
// markdown by goldmark. Used for tags goldmark would otherwise silently drop
// or safely escape as raw HTML (e.g. <details>).
// ---------------------------------------------------------------------------

type htmlWrapperBlock struct {
	openHTML  string
	closeHTML string
}

// extractHTMLBlocks pulls the opening/closing wrapper for the given tag out of bare
// HTML blocks and replaces them with placeholders, so they survive goldmark's
// HTML-block handling and can be reinserted as sanitized, live HTML after rendering.
// If innerTag is non-empty and immediately follows the opening tag on its own line
// (e.g. <summary> right after <details>), its content is rendered as inline markdown
// and kept as part of the open wrapper; everything else in between is left untouched.
func (h *MarkdownHandler) extractHTMLBlocks(content []byte, tag, innerTag string) ([]byte, []htmlWrapperBlock) {
	openRe := regexp.MustCompile(`(?i)^<` + tag + `[\s>]`)
	var innerRe *regexp.Regexp
	if innerTag != "" {
		innerRe = regexp.MustCompile(`(?is)^<` + innerTag + `[^>]*>(.*)</` + innerTag + `>\s*$`)
	}

	var blocks []htmlWrapperBlock
	lines := strings.Split(string(content), "\n")
	var result []string
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !openRe.MatchString(trimmed) {
			result = append(result, lines[i])
			i++
			continue
		}

		// collect the whole block, tracking tag depth since the content commonly
		// contains blank lines
		var block []string
		depth := 0
		for i < len(lines) {
			block = append(block, lines[i])
			low := strings.ToLower(lines[i])
			depth += strings.Count(low, "<"+tag)
			depth -= strings.Count(low, "</"+tag)
			i++
			if depth <= 0 {
				break
			}
		}
		if len(block) < 2 {
			result = append(result, block...)
			continue
		}

		openLines := []string{block[0]}
		bodyStart := 1
		if innerRe != nil && len(block) > 2 {
			if m := innerRe.FindStringSubmatch(strings.TrimSpace(block[1])); m != nil {
				openLines = append(openLines, fmt.Sprintf("<%s>%s</%s>", innerTag, RenderInlineMarkdown(m[1]), innerTag))
				bodyStart = 2
			}
		}

		idx := len(blocks)
		blocks = append(blocks, htmlWrapperBlock{
			openHTML:  strings.Join(openLines, "\n"),
			closeHTML: block[len(block)-1],
		})

		placeholder := fmt.Sprintf("KNOVHTML%s%d", strings.ToUpper(tag), idx)
		result = append(result, "", placeholder+"OPEN", "")
		result = append(result, block[bodyStart:len(block)-1]...)
		result = append(result, "", placeholder+"CLOSE", "")
	}
	return []byte(strings.Join(result, "\n")), blocks
}

// restoreHTMLBlocks reinserts the extracted wrapper tags as live HTML.
func (h *MarkdownHandler) restoreHTMLBlocks(html, tag string, blocks []htmlWrapperBlock) string {
	for i, b := range blocks {
		placeholder := fmt.Sprintf("KNOVHTML%s%d", strings.ToUpper(tag), i)
		html = strings.ReplaceAll(html, "<p>"+placeholder+"OPEN</p>", b.openHTML)
		html = strings.ReplaceAll(html, "<p>"+placeholder+"CLOSE</p>", b.closeHTML)
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
	relPath := pathutils.ToRelative(filePath)
	headerRe := regexp.MustCompile(`<h([1-6])\s+id="([^"]+)"[^>]*>(.*?)</h[1-6]>`)
	return headerRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := headerRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		editBtn := fmt.Sprintf(
			`<a href="/files/edit/%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
			relPath, parts[2],
			translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
		)
		return fmt.Sprintf(`<h%s id="%s">%s%s</h%s>`, parts[1], parts[2], parts[3], editBtn, parts[1])
	})
}

// wrapHeaderSections wraps content between headers in <div class="content-section">
// and appends a section-edit button at the bottom-right of each section.
func (h *MarkdownHandler) wrapHeaderSections(htmlContent, filePath string) string {
	headerRe := regexp.MustCompile(`<h([1-6])[^>]*>.*?</h[1-6]>`)
	idRe := regexp.MustCompile(`id="([^"]+)"`)
	relPath := pathutils.ToRelative(filePath)
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
		headerHTML := htmlContent[match[0]:match[1]]
		out.WriteString(headerHTML)
		start := match[1]
		end := len(htmlContent)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := strings.TrimSpace(htmlContent[start:end])
		if section != "" {
			editBtn := ""
			if relPath != "" {
				if idParts := idRe.FindStringSubmatch(headerHTML); len(idParts) >= 2 {
					editBtn = fmt.Sprintf(
						`<a href="/files/edit/%s?section=%s" class="section-edit-btn" title="%s"><i class="fa fa-pen"></i> %s</a>`,
						relPath, idParts[1],
						translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
						translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
					)
				}
			}
			fmt.Fprintf(&out, `<div class="content-section">%s%s</div>`, section, editBtn)
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

		// strip CommonMark angle-bracket link destination syntax: <url with spaces>
		if strings.HasPrefix(u, "<") && strings.HasSuffix(u, ">") {
			u = u[1 : len(u)-1]
		}

		if isImage {
			return matches[1] + "[" + text + "](" + strings.ReplaceAll(u, "\\", "/") + ")"
		}

		// external links and pure anchors — leave as-is
		if strings.Contains(u, "://") || strings.HasPrefix(u, "#") {
			return match
		}

		// split off anchor fragment before any path processing
		anchor := ""
		if idx := strings.Index(u, "#"); idx != -1 {
			anchor = u[idx:]
			u = u[:idx]
		}

		// media links
		if strings.HasPrefix(u, "/files/media/") {
			return "[" + text + "](/media/" + u[len("/files/media/"):] + ")"
		}
		if strings.HasPrefix(u, "/media/") {
			// already an absolute, directly-servable media route — leave untouched
			return match
		}
		if strings.HasPrefix(u, "media/") {
			return "[" + text + "](/" + u + "?mode=detail)"
		}

		// already routed /files/ links — re-encode so goldmark accepts spaces/unicode
		if strings.HasPrefix(u, "/files/") {
			rel := u[len("/files/"):]
			if decoded, err := url.PathUnescape(rel); err == nil {
				rel = decoded
			}
			return "[" + text + "](" + pathutils.ToFileURL(rel) + anchor + ")"
		}

		// internal doc links — route to /files/
		if decoded, err := url.PathUnescape(u); err == nil {
			u = decoded
		}
		if !strings.HasSuffix(u, ".md") {
			u += ".md"
		}
		return "[" + text + "](" + pathutils.ToFileURL(u) + anchor + ")"
	})
}

var wikiExtractRe = regexp.MustCompile(`\[\[([^\[\]|]+)`)

func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	var links []string
	text := string(content)
	text = removeCodeBlocks(text)

	// extract [[wiki links]] (path before | if any)
	for _, match := range wikiExtractRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			if link := strings.TrimSpace(match[1]); link != "" {
				links = append(links, link)
			}
		}
	}

	// match [text](url) but exclude image links ![]()
	// prepend a space so links at position 0 (start of file/line) still have a preceding char
	mdLinkRegex := regexp.MustCompile(`[^!]\[([^\]]+)\]\(([^\)]+)\)`)
	for _, match := range mdLinkRegex.FindAllStringSubmatch(" "+text, -1) {
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

// sanitizeHTML strips on* event attributes and javascript: hrefs from rendered HTML
// to prevent content files from executing JavaScript in the browser.
func sanitizeHTML(html string) string {
	// strip on* event handlers (onclick, onload, onerror, etc.)
	html = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]*)`).ReplaceAllString(html, "")
	// strip javascript: URLs
	html = regexp.MustCompile(`(?i)(href|src|action)\s*=\s*"javascript:[^"]*"`).ReplaceAllString(html, `$1="#"`)
	html = regexp.MustCompile(`(?i)(href|src|action)\s*=\s*'javascript:[^']*'`).ReplaceAllString(html, `$1="#"`)
	// strip <script> tags and their content
	html = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(html, "")
	return html
}
