package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"knov/internal/configmanager"
	"knov/internal/markdown"
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

// markdownExtensions are the extensions the markdown handler renders rather than letting
// fall through to the plaintext handler. One list so CanHandle and IsMarkdownExtension agree.
var markdownExtensions = []string{".md", ".markdown", ".index", ".moc", ".list", ".todo", ".book"}

func (h *MarkdownHandler) CanHandle(filename string) bool {
	return IsMarkdownExtension(filename)
}

// IsMarkdownExtension reports whether filename's extension renders as markdown. Save paths
// use it to check a chosen name still renders (else it shows its raw source).
func IsMarkdownExtension(filename string) bool {
	return slices.Contains(markdownExtensions, strings.ToLower(filepath.Ext(filename)))
}

func (h *MarkdownHandler) Parse(content []byte) ([]byte, error) {
	content = StripFrontMatter(content)
	processed := h.wrapRawHTMLBlocks(string(content))
	processed = ResolveWikiLinks(processed)
	processed = ProcessMarkdownLinks(processed)
	return []byte(processed), nil
}

var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// ResolveWikiLinks converts [[path]] and [[path|display]] to standard markdown links.
//
// When there's no explicit "|display", this leaves the markdown link text
// empty rather than deriving a label here - ProcessMarkdownLinks (which
// always runs right after this) owns all fallback-label generation and
// applies it the same way regardless of whether a link started as [[wiki]]
// or was hand-typed as [](...). Don't re-add filename/anchor-humanizing logic
// in this function; that duplication is what previously let the wiki-link
// path drift out of sync with the hand-typed path (unicode anchors
// mis-capitalized, an anchor-only [[#some-header]] rendering as ".").
func ResolveWikiLinks(content string) string {
	return wikiLinkRe.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2]

		display := ""
		if parts := strings.SplitN(inner, "|", 2); len(parts) == 2 {
			display = strings.TrimSpace(parts[1])
		}

		linkPath, anchor := ResolveWikiTarget(inner)

		// pure same-page anchor (e.g. "[[#some-header]]") - keep it a real
		// same-page link instead of routing it through /files/. Leaving
		// display empty lets ProcessMarkdownLinks fill in the header text,
		// same as it does for a hand-typed "[](#some-header)".
		if linkPath == "" {
			return "[" + display + "](" + anchor + ")"
		}
		return "[" + display + "](" + pathutils.ToFileURL(linkPath) + anchor + ")"
	})
}

// ResolveWikiTarget normalizes a wikilink body ("path", "path#anchor", "path|text", ...)
// into a docs-relative path and a leading "#anchor" (empty when absent): drop the "|alias",
// split off the "#anchor", default a missing extension to ".md", URL-decode the path. A
// bare "#anchor" yields an empty path. Shared with internal/book so a book entry resolves
// its reference the same way the link renderer does.
func ResolveWikiTarget(inner string) (linkPath, anchor string) {
	if i := strings.Index(inner, "|"); i != -1 {
		inner = inner[:i]
	}
	linkPath = strings.TrimSpace(inner)

	if idx := strings.Index(linkPath, "#"); idx != -1 {
		anchor = linkPath[idx:]
		linkPath = linkPath[:idx]
	}
	if linkPath == "" {
		return "", anchor
	}

	if !strings.Contains(filepath.Base(linkPath), ".") {
		linkPath += ".md"
	}
	if decoded, err := url.PathUnescape(linkPath); err == nil {
		linkPath = decoded
	}
	return linkPath, anchor
}

// wrapRawHTMLBlocks wraps bare HTML blocks in fenced code blocks so goldmark
// renders them as code instead of silently omitting them.
func (h *MarkdownHandler) wrapRawHTMLBlocks(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	i := 0
	inFence := false
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// don't touch content already inside a fenced code block
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			result = append(result, line)
			i++
			continue
		}
		if inFence {
			result = append(result, line)
			i++
			continue
		}

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
	content = PreprocessTodoStates(content)

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
	result = h.wrapHeaderSections(result, filePath)
	return []byte(result), nil
}

// inlineMD is the shared goldmark instance for RenderInlineMarkdown and
// RenderHeadingInline. goldmark is safe for concurrent use, so one instance
// avoids rebuilding the parser on every call (RenderHeadingInline runs once per
// heading, both in parser.Headings and in the render-time heading renderer).
var inlineMD = goldmark.New(goldmark.WithExtensions(extension.GFM))

// RenderInlineMarkdown renders inline markdown (code, bold, italic, links) to HTML.
// It strips the wrapping <p> tag goldmark adds around single-line input.
func RenderInlineMarkdown(s string) string {
	var buf bytes.Buffer
	if err := inlineMD.Convert([]byte(s), &buf); err != nil {
		return s
	}
	result := strings.TrimSpace(buf.String())
	result = strings.TrimPrefix(result, "<p>")
	result = strings.TrimSuffix(result, "</p>")
	return strings.TrimSpace(result)
}

var headingInlineRe = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)

// RenderHeadingInline renders s as a heading's inline content, used only to feed
// SlugHeading: renderHeading calls it for the render-time id and HeadingID calls
// it for the pre-render scan, so both slug off the same HTML (the visible heading
// markup comes from goldmark's normal child rendering, not from here). Rendering s
// on its own would let a leading block marker ("1. ", "- ", "> ") turn into a
// list or blockquote and drop the marker text; wrapping it in "# " keeps
// goldmark on the inline path.
func RenderHeadingInline(s string) string {
	var buf bytes.Buffer
	if err := inlineMD.Convert([]byte("# "+s), &buf); err != nil {
		return s
	}
	m := headingInlineRe.FindStringSubmatch(buf.String())
	if m == nil {
		return s
	}
	return strings.TrimSpace(m[1])
}

// ---------------------------------------------------------------------------
// Custom node renderer — handles code blocks (chroma), tables (HTMX), images
// ---------------------------------------------------------------------------

type knovNodeRenderer struct {
	filePath  string
	relPath   string // pathutils.ToRelative(filePath); PathlessRender when there is no source file
	blocks    []codeBlock
	tableIdx  int
	usedIDs   map[string]int // heading-id collision counts for this document
	headingID string         // id of the heading currently being rendered (set on enter, used on exit)
	html.Config
}

func newKnovNodeRenderer(filePath string, blocks []codeBlock) renderer.NodeRenderer {
	return &knovNodeRenderer{
		filePath: filePath,
		relPath:  pathutils.ToRelative(filePath),
		blocks:   blocks,
		usedIDs:  make(map[string]int),
		Config:   html.NewConfig(),
	}
}

func (r *knovNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)

	// renderTable emits the live, file-backed table editor; with no source file to load,
	// leave KindTable to goldmark's default GFM renderer (a plain static <table>).
	if r.filePath != PathlessRender {
		reg.Register(extast.KindTable, r.renderTable)
	}
}

// renderHeading writes the <hN id> wrapper and the trailing anchor link, but
// lets goldmark render the heading's children itself (WalkContinue) so the
// visible markup matches the rest of the document (typographer, hard wraps).
// The id is slugged through the shared SlugHeading from the raw heading text so
// it matches parser.Headings (section editing / autocomplete) without a second,
// drift-prone computation. usedIDs lives on the renderer, so collisions dedupe
// in document order; headings never nest, so one headingID field is enough to
// carry the id from the enter call to the exit call.
//
// Emitted format contract: `<hN id="slug">` - id is the first and only attribute
// on the tag. GenerateTOC and wrapHeaderSections both read the id positionally
// with a regex that expects it right after the level, so adding another attribute
// here (or moving id) silently blanks the TOC and the section wrappers. Change
// those regexes too if this format changes.
//
// On exit it also emits the per-heading section buttons (see headerButtons),
// keyed off r.headingID rather than the tag text. This is the only place heading
// markup is produced, so a heading goldmark never parses as one (raw HTML) gets
// no id and no buttons.
func (r *knovNodeRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if !entering {
		fmt.Fprintf(w, `<a href="#%s" class="header-anchor" aria-hidden="true">#</a>%s</h%d>`, r.headingID, r.headerButtons(), n.Level)
		return ast.WalkContinue, nil
	}

	var raw bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		raw.Write(seg.Value(source))
	}

	r.headingID = SlugHeading(RenderHeadingInline(raw.String()), r.usedIDs)
	fmt.Fprintf(w, `<h%d id="%s">`, n.Level, r.headingID)
	return ast.WalkContinue, nil
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

// extractCodeBlocks replaces every ``` / ~~~ fenced block with a KNOVCODEBLOCK<n>
// placeholder (kept in a clean, blank-line-padded fence so goldmark still parses
// it as one) so chroma handles highlighting and goldmark never sees the raw code.
func (h *MarkdownHandler) extractCodeBlocks(content []byte) ([]byte, []codeBlock) {
	lines := strings.Split(string(content), "\n")
	found := markdown.CodeBlocks(lines)
	if len(found) == 0 {
		return content, nil
	}

	var blocks []codeBlock
	var result []string
	prev := 0
	for _, cb := range found {
		result = append(result, lines[prev:cb.Start]...)

		lang := cb.Lang
		if lang == "" {
			lang = "text"
		}
		placeholder := fmt.Sprintf("KNOVCODEBLOCK%d", len(blocks))
		blocks = append(blocks, codeBlock{lang: lang, content: cb.Body + "\n"})
		result = append(result, "", cb.Indent+"```", cb.Indent+placeholder, cb.Indent+"```", "")

		prev = cb.End + 1 // cb.End is always in range, so prev is at most len(lines)
	}
	result = append(result, lines[prev:]...)
	return []byte(strings.Join(result, "\n")), blocks
}

// restoreOrphanCodeBlocks replaces any KNOVCODEBLOCK placeholder that the node renderer
// did not handle (e.g. inside a <p> tag due to unusual nesting) with highlighted HTML.
// The replacement is chroma output (<pre>/<code>/<span>, code text HTML-escaped), so it
// can never reintroduce a live <h1-6> after the renderer has assigned heading ids.
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

// headingTagRe matches an opening or closing <h1>-<h6> tag. Used to keep raw HTML
// that bypasses goldmark (restored wrapper blocks) from smuggling in a heading
// the renderer never saw and so never gave an id.
var headingTagRe = regexp.MustCompile(`(?i)</?h[1-6][^>]*>`)

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
			// strip any <h1-6> from the wrapper lines: restoreHTMLBlocks reinserts
			// these verbatim after the renderer has already given every real heading
			// its id, so a stray heading tag here would be an id-less phantom that no
			// anchor, TOC entry or section edit can reach.
			openHTML:  headingTagRe.ReplaceAllString(strings.Join(openLines, "\n"), ""),
			closeHTML: headingTagRe.ReplaceAllString(block[len(block)-1], ""),
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

// headerButtons returns the per-heading section buttons (pdf export + edit section)
// that renderHeading appends inside every <hN>, keyed to the heading currently being
// rendered. Empty for a pathless render (no source file to target).
func (r *knovNodeRenderer) headerButtons() string {
	if r.relPath == PathlessRender {
		return ""
	}
	lang := configmanager.GetLanguage()
	pdfBtn := ""
	if configmanager.PDFShowHeaderButton.Get() {
		pdfBtn = fmt.Sprintf(
			`<a href="/api/files/export/pdf?filepath=%s&section=%s" class="header-pdf-btn" title="%s"><i class="fa fa-file-pdf"></i></a>`,
			url.QueryEscape(r.relPath), url.QueryEscape(r.headingID),
			translation.SprintfForRequest(lang, "export section to pdf"),
		)
	}
	editBtn := fmt.Sprintf(
		`<a href="/files/edit/%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
		r.relPath, r.headingID,
		translation.SprintfForRequest(lang, "edit section"),
	)
	return pdfBtn + editBtn
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
			if relPath != PathlessRender {
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

// ProcessMarkdownLinks rewrites internal [text](url) links to /files/ routes.
// This is also where every empty-text "[](url)" link (including ones
// ResolveWikiLinks produced from a [[wiki link]] with no "|display") gets its
// fallback label - keep that logic here only, see the note on ResolveWikiLinks.
func ProcessMarkdownLinks(content string) string {
	re := regexp.MustCompile(`(!)?\[([^\]]*)\]\(([^)]+)\)`)
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

		// external links — leave as-is
		if strings.Contains(u, "://") {
			return match
		}

		// pure anchor (same-page link) — fall back to the header text if empty
		if strings.HasPrefix(u, "#") {
			if text == "" {
				slug := strings.TrimPrefix(u, "#")
				if decoded, err := url.PathUnescape(slug); err == nil {
					slug = decoded
				}
				text = humanizeSlug(slug)
			}
			return "[" + text + "](" + u + ")"
		}

		// split off anchor fragment before any path processing
		anchor := ""
		if idx := strings.Index(u, "#"); idx != -1 {
			anchor = u[idx:]
			u = u[:idx]
		}

		// empty link text (e.g. "[](path.md#anchor)") — fall back to filename + header
		if text == "" {
			decodedU, decodedAnchor := u, anchor
			if decoded, err := url.PathUnescape(u); err == nil {
				decodedU = decoded
			}
			if decoded, err := url.PathUnescape(anchor); err == nil {
				decodedAnchor = decoded
			}
			text = autoLinkText(decodedU, decodedAnchor)
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

// autoLinkText builds a display label for a link left without one, e.g.
// "escrow.md" + "#todo-vorlage" -> "escrow - Todo Vorlage".
func autoLinkText(u, anchor string) string {
	name := strings.TrimSuffix(filepath.Base(u), filepath.Ext(u))
	if anchor == "" {
		return name
	}
	return name + " - " + humanizeSlug(strings.TrimPrefix(anchor, "#"))
}

// humanizeSlug turns a header-anchor slug like "todo-vorlage" into "Todo Vorlage".
func humanizeSlug(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	return strings.Join(words, " ")
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
	// drop ``` / ~~~ fenced blocks, then any remaining inline `code` spans, so links
	// inside code are never extracted
	text = markdown.StripFencedBlocks(strings.Split(text, "\n"))
	return regexp.MustCompile("`[^`\n]+`").ReplaceAllString(text, "")
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
