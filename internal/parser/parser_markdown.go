package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
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
	"knov/internal/utils"

	"github.com/microcosm-cc/bluemonday"
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

// markdownExtensions are the extensions the markdown handler renders rather than letting
// fall through to the plaintext handler. One list so CanHandle and IsMarkdownExtension agree.
var markdownExtensions = []string{".md", ".markdown", ".index", ".moc", ".list", ".todo", ".book", ".tracker"}

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

// ResolveWikiLinks converts [[path]] and [[path|display]] outside code to standard markdown links.
//
// When there's no explicit "|display", this leaves the markdown link text
// empty rather than deriving a label here - ProcessMarkdownLinks (which
// always runs right after this) owns all fallback-label generation and
// applies it the same way regardless of whether a link started as [[wiki]]
// or was hand-typed as [](...). Don't re-add filename/anchor-humanizing logic
// in this function; that duplication is what previously let the wiki-link
// path drift out of sync with the hand-typed path (unicode anchors
// mis-capitalized, an anchor-only [[#some-header]] rendering as ".").
// The destination is written as a decoded /files/ path, not an app url yet -
// ProcessMarkdownLinks turns it into one, so always run it on the result.
func ResolveWikiLinks(content string) string {
	return replaceOutsideCode(content, func(part string, _ bool) string {
		return wikiLinkRe.ReplaceAllStringFunc(part, func(match string) string {
			inner := match[2 : len(match)-2]
			wl := ParseLink(inner, LinkWiki)
			display := strings.TrimSpace(strings.TrimPrefix(wl.Alias, "|"))

			// the anchor is written as markdown too, encoded so a quote or ")" in it stays
			// part of the anchor - ProcessMarkdownLinks decodes it (AnchorText)
			l := Link{Kind: LinkMarkdown}
			if text := wl.AnchorText(); text != "" {
				l.Anchor = "#" + encodeLinkPath(text, LinkMarkdown)
			}
			// a pure same-page anchor (e.g. "[[#some-header]]") has no path and stays a
			// real same-page link instead of routing it through /files/. Leaving
			// display empty lets ProcessMarkdownLinks fill in the header text,
			// same as it does for a hand-typed "[](#some-header)".
			if linkPath := ResolveWikiTarget(inner); linkPath != "" {
				l.Path = "/files/" + linkPath
			}
			return "[" + display + "](" + l.Dest() + ")"
		})
	})
}

// ResolveWikiTarget normalizes a wikilink body ("path", "path#anchor", "path|text", ...)
// into a docs-relative path: read with ParseLink, default a missing extension to ".md". A bare
// "#anchor" yields an empty path. Shared with internal/book so a book entry resolves its
// reference the same way the link renderer does.
func ResolveWikiTarget(inner string) string {
	return utils.WithDefaultLinkExt(ParseLink(inner, LinkWiki).Path)
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

func (h *MarkdownHandler) Render(content []byte, filePath string, editableSections bool) ([]byte, error) {
	html, _, err := h.RenderWithUsedIDs(content, filePath, editableSections, make(map[string]int))
	return html, err
}

// RenderWithUsedIDs is Render, but heading-id collision counts are read from and
// written back to the caller's usedIDs map instead of a fresh one, and it also
// returns the headings (with the exact ids it assigned them) it found in
// content. Callers rendering several documents onto one page (e.g.
// renderDocsMarkdown) pass the same map across calls so headings sharing text
// across documents still dedupe (e.g. two changelog files each with "##
// Added"), and build their TOC from the returned headings (see
// parser.HeadingsToTOC) instead of re-scanning content themselves - so the TOC
// ids can't drift from the ids this render actually used, even across several
// calls sharing one map.
func (h *MarkdownHandler) RenderWithUsedIDs(content []byte, filePath string, editableSections bool, usedIDs map[string]int) ([]byte, []Heading, error) {
	content, blocks := h.extractCodeBlocks(content)
	content, detailsBlocks := h.extractHTMLBlocks(content, "details", "summary")
	content = PreprocessTodoStates(content)

	// scanned after the extraction above, from the exact content goldmark converts
	// below - so a heading-like line inside a <details> block (extracted to raw HTML,
	// never reaching the AST as a real heading) can't desync this scan from the AST
	// walk's heading count/order the way scanning the original content would.
	headings := HeadingsWithIDs(strings.Split(string(content), "\n"), usedIDs)

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),

		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(todoStateTransformer{}, 100),
			),
		),

		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			renderer.WithNodeRenderers(
				util.Prioritized(newKnovNodeRenderer(filePath, blocks, editableSections, headings, usedIDs), 1),
			),
		),
	)

	sw := newSectionWriter(filePath, editableSections)
	source := []byte(content)
	if err := md.Convert(source, sw); err != nil {
		return nil, nil, err
	}

	result := sw.out.String()
	result = h.restoreOrphanCodeBlocks(result, blocks)
	result = h.restoreHTMLBlocks(result, "details", detailsBlocks)
	result = sanitizeHTML(result)
	return []byte(result), headings, nil
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
	filePath         string
	relPath          string // pathutils.ToRelative(filePath); PathlessRender when there is no source file
	blocks           []codeBlock
	tableIdx         int
	headings         []Heading      // ids already assigned by the pre-render Headings scan (see RenderWithUsedIDs), matched to AST headings by line number
	headingIdx       int            // headings[headingIdx] is the next unconsumed scanned heading, advanced past by line number as renderHeading runs
	usedIDs          map[string]int // heading-id collision counts; only touched by the headings-exhausted fallback below, to stay consistent with the scan's counts
	headingID        string         // id of the heading currently being rendered (set on enter, used on exit)
	editableSections bool           // false suppresses the per-heading/per-section edit buttons (list, todo, tracker, filter, index, book)
	html.Config
}

func newKnovNodeRenderer(filePath string, blocks []codeBlock, editableSections bool, headings []Heading, usedIDs map[string]int) renderer.NodeRenderer {
	return &knovNodeRenderer{
		filePath:         filePath,
		relPath:          pathutils.ToRelative(filePath),
		blocks:           blocks,
		headings:         headings,
		usedIDs:          usedIDs,
		editableSections: editableSections,
		Config:           html.NewConfig(),
	}
}

func (r *knovNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(ast.KindListItem, r.renderListItem)
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)
	reg.Register(kindTodoDate, r.renderTodoDate)

	// renderTable emits the live, file-backed table editor; with no source file to load,
	// leave KindTable to goldmark's default GFM renderer (a plain static <table>).
	if r.filePath != PathlessRender {
		reg.Register(extast.KindTable, r.renderTable)
	}
}

// renderHeading writes the <hN id> wrapper and the trailing anchor link, but
// lets goldmark render the heading's children itself (WalkContinue) so the
// visible markup matches the rest of the document (typographer, hard wraps).
// The id comes off r.headings, the same pre-render Headings scan a caller uses
// to build its TOC (see RenderWithUsedIDs and parser.HeadingsToTOC) - so a
// rendered anchor and its TOC entry are the same id by construction, not by two
// independent computations that happen to agree. headings never nest, so one
// headingID field is enough to carry the id from the enter call to the exit
// call.
//
// The scan and this walk are matched up by line number (Heading.Line), not
// raw position: ATXHeading is a documented approximation of CommonMark (e.g.
// it doesn't stop at 3 spaces of indentation), so the scan can find a
// heading-like line goldmark's real parser doesn't treat as one. Skipping any
// scanned heading whose line comes before the node being rendered keeps such
// a phantom from being mistaken for - and shifting the id of - the next real
// heading. If goldmark's walk still runs past the end of the scan, or never
// finds a scanned heading at the node's line, that one falls back to slugging
// on the spot against the same usedIDs the scan already advanced, so dedup
// counts stay consistent either way.
//
// It also drives sectionWriter's section boundaries: startHeading before the
// opening tag closes out the content-section for everything since the last
// heading, endHeading after the closing tag reopens buffering for the next one.
// w is always the *sectionWriter Render constructs, never a plain writer.
//
// On exit it also emits the per-heading section buttons (see headerButtons),
// keyed off r.headingID rather than the tag text. This is the only place heading
// markup is produced, so a heading goldmark never parses as one (raw HTML) gets
// no id and no buttons.
func (r *knovNodeRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	sw, _ := w.(*sectionWriter)
	if !entering {
		fmt.Fprintf(w, `<a href="#%s" class="header-anchor" aria-hidden="true">#</a>%s</h%d>`, r.headingID, r.headerButtons(), n.Level)
		if sw != nil {
			sw.endHeading(r.headingID)
		}
		return ast.WalkContinue, nil
	}

	lines := n.Lines()
	var lineNo int
	if lines.Len() > 0 {
		lineNo = bytes.Count(source[:lines.At(0).Start], []byte("\n"))
	}
	for r.headingIdx < len(r.headings) && r.headings[r.headingIdx].Line < lineNo {
		r.headingIdx++ // scanned heading the AST never produced - skip so it can't shift the next real one's id
	}

	if r.headingIdx < len(r.headings) && r.headings[r.headingIdx].Line == lineNo {
		r.headingID = r.headings[r.headingIdx].ID
		r.headingIdx++
	} else {
		var raw bytes.Buffer
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			raw.Write(seg.Value(source))
		}
		r.headingID = SlugHeading(RenderHeadingInline(raw.String()), r.usedIDs)
	}

	if sw != nil {
		sw.startHeading()
	}
	fmt.Fprintf(w, `<h%d id="%s">`, n.Level, r.headingID)
	return ast.WalkContinue, nil
}

func (r *knovNodeRenderer) renderFencedCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.FencedCodeBlock)
	info := ""
	if n.Info != nil {
		info = strings.TrimSpace(string(n.Info.Segment.Value(source)))
	}
	lang := "text"
	if info != "" {
		lang = info
	}

	// check if this is a placeholder — restore from blocks slice. Usually the marker
	// is its own content line, but a too-short block (see placeholderLines) has none,
	// so it rides in the info string instead.
	var content string
	var buf bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	raw := buf.String()

	placeholder := strings.TrimSpace(raw)
	if placeholder == "" {
		placeholder = info
	}
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
			fmt.Fprintf(w, `<img src="%s" alt="%s" />`, pathutils.ToMediaURL(previewPath), alt)
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
// placeholder (kept in a clean fence so goldmark still parses it as one) so
// chroma handles highlighting and goldmark never sees the raw code.
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
		result = append(result, placeholderLines(cb, placeholder)...)

		prev = cb.End + 1 // cb.End is always in range, so prev is at most len(lines)
	}
	result = append(result, lines[prev:]...)
	return []byte(strings.Join(result, "\n")), blocks
}

// placeholderLines builds the lines that replace a fenced block, always spanning
// exactly as many lines as cb did in the original (blank padding around the fence,
// same as before, then inside it once that's used up). Anything else desyncs every
// line number computed from here on (e.g. a todo checkbox's data-line) from the raw
// file's real line numbers.
//
// A marker on its own content line needs 3 lines minimum (open fence, marker,
// close fence), which is already more than an empty block (```` ``` ```` / ```` ``` ````,
// 2 lines) or an unterminated one-line fence spans. Padding those up would desync
// just the same, so instead the marker rides in the opening fence's info string
// (renderFencedCode checks there too), needing no line of its own.
func placeholderLines(cb markdown.CodeBlock, placeholder string) []string {
	want := cb.End - cb.Start + 1
	if want < 3 {
		block := []string{cb.Indent + "```" + placeholder}
		for i := 1; i < want; i++ {
			block = append(block, cb.Indent+"```")
		}
		return block
	}
	extra := want - 3
	lead, trail := 0, 0
	if extra > 0 {
		lead, extra = 1, extra-1
	}
	if extra > 0 {
		trail, extra = 1, extra-1
	}

	block := make([]string, 0, want)
	for i := 0; i < lead; i++ {
		block = append(block, "")
	}
	block = append(block, cb.Indent+"```", cb.Indent+placeholder)
	for i := 0; i < extra; i++ {
		block = append(block, "")
	}
	block = append(block, cb.Indent+"```")
	for i := 0; i < trail; i++ {
		block = append(block, "")
	}
	return block
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

// resolveMediaPath returns a clean relative media path from a markdown image destination
// (goldmark already resolved its escapes and entities, so it's only percent-decoded).
func resolveMediaPath(dest string) string {
	dest = unescapePath(dest)
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
	if !r.editableSections {
		return pdfBtn
	}
	editBtn := fmt.Sprintf(
		`<a href="%s?section=%s" class="header-edit-btn" title="%s"><i class="fa fa-edit"></i></a>`,
		pathutils.ToFileEditURL(r.relPath), url.QueryEscape(r.headingID),
		translation.SprintfForRequest(lang, "edit section"),
	)
	return pdfBtn + editBtn
}

// sectionWriter is the target Render hands to goldmark: it implements util.BufWriter
// itself, so goldmark uses it directly as the render output instead of wrapping a
// plain buffer in its own bufio.Writer (see renderer.Renderer.Render). That gives it
// first look at every byte any node renderer writes, built-in or knov's own, so it can
// wrap content between headings in <div class="content-section"> as the document
// renders instead of re-parsing the finished HTML string for <hN> tags afterward.
//
// Content is buffered per section (in segment) and only committed to out - trimmed,
// and dropped if empty - at the next heading boundary or at end of document (Flush,
// which goldmark calls once after the walk completes). renderHeading calls
// startHeading/endHeading around a heading's own tag and children so that markup lands
// in out directly, outside the wrapping divs.
type sectionWriter struct {
	out              bytes.Buffer
	segment          bytes.Buffer
	passthrough      bool
	editID           string // id of the most recently closed heading; keys the edit button for the section that follows ("" before the first heading)
	relPath          string
	editableSections bool
}

func newSectionWriter(filePath string, editableSections bool) *sectionWriter {
	return &sectionWriter{relPath: pathutils.ToRelative(filePath), editableSections: editableSections}
}

func (s *sectionWriter) dst() *bytes.Buffer {
	if s.passthrough {
		return &s.out
	}
	return &s.segment
}

func (s *sectionWriter) Write(p []byte) (int, error)         { return s.dst().Write(p) }
func (s *sectionWriter) WriteByte(c byte) error              { return s.dst().WriteByte(c) }
func (s *sectionWriter) WriteRune(r rune) (int, error)       { return s.dst().WriteRune(r) }
func (s *sectionWriter) WriteString(str string) (int, error) { return s.dst().WriteString(str) }
func (s *sectionWriter) Available() int                      { return 0 } // unused by goldmark; only here to satisfy util.BufWriter
func (s *sectionWriter) Buffered() int                       { return s.segment.Len() }
func (s *sectionWriter) Flush() error                        { s.closeSection(); return nil }

// startHeading closes out the section preceding a heading and switches to passthrough
// so the heading's own tag and children write straight to out.
func (s *sectionWriter) startHeading() {
	s.closeSection()
	s.passthrough = true
}

// endHeading switches back to buffering into the next section, keyed to id for that
// section's edit button.
func (s *sectionWriter) endHeading(id string) {
	s.passthrough = false
	s.editID = id
}

func (s *sectionWriter) closeSection() {
	section := strings.TrimSpace(s.segment.String())
	s.segment.Reset()
	if section == "" {
		return
	}
	editBtn := ""
	if s.editID != "" && s.relPath != PathlessRender && s.editableSections {
		editBtn = fmt.Sprintf(
			`<a href="%s?section=%s" class="section-edit-btn" title="%s"><i class="fa fa-pen"></i> %s</a>`,
			pathutils.ToFileEditURL(s.relPath), url.QueryEscape(s.editID),
			translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "edit section"),
		)
	}
	fmt.Fprintf(&s.out, `<div class="content-section">%s%s</div>`, section, editBtn)
}

// ---------------------------------------------------------------------------
// Link processing and helpers
// ---------------------------------------------------------------------------

// [text](dest) or ![alt](dest) (the text may hold escaped brackets), the destination read like
// RewriteLinks does. The text may span lines and hold code but no unescaped "[", so a nested
// ![img](src) or one after a stray "[" matches on its own; the outer link of nested brackets
// matches as "](dest)" alone.
var processMdLinkRe = regexp.MustCompile(`(!?\[(?:[^\[\]\\]|\\.)*)?\]\(` + mdLinkDestPattern + `\)`)

// ProcessMarkdownLinks rewrites every internal [text](url) link outside code to its app url
// (/files/, /media/), read with ParseLink and the metadata extension rule, an anchor to the
// heading id it names (see AnchorID), and a reference definition to a docs file the same way.
// This is also where every empty-text "[](url)" link (including ones
// ResolveWikiLinks produced from a [[wiki link]] with no "|display") gets its
// fallback label - keep that logic here only, see the note on ResolveWikiLinks.
func ProcessMarkdownLinks(content string) string {
	// links are matched on the whole content with code blanked out, so a link text spanning
	// lines or holding `code` keeps its "![" and brackets / destinations in code are never seen
	masked := maskCode(content)
	var b strings.Builder
	last := 0
	for _, m := range processMdLinkRe.FindAllStringSubmatchIndex(masked, -1) {
		if strings.Contains(masked[m[4]:m[5]], "\x00") {
			continue
		}
		open := ""
		if m[2] != -1 {
			open = content[m[2]:m[3]]
		}
		b.WriteString(content[last:m[0]])
		b.WriteString(processMarkdownLink(content[m[0]:m[1]], open, content[m[4]:m[5]]))
		last = m[1]
	}
	b.WriteString(content[last:])
	return processRefDefs(b.String())
}

// processMarkdownLink rewrites one processMdLinkRe match with its opening "[text" (or "") and
// destination, see ProcessMarkdownLinks.
func processMarkdownLink(match, open, dest string) string {
	l := ParseLink(dest, LinkMarkdown)

	// external links (a scheme like https:/mailto: or a //host, same check as ExtractLinks) — leave as-is,
	// a javascript: link is removed by sanitizeHTML
	if l.External {
		return match
	}
	// an image is rendered from its destination by goldmark, written back normalized
	if strings.HasPrefix(open, "!") {
		return open + "](" + l.Dest() + ")"
	}

	text := strings.TrimSpace(strings.TrimPrefix(open, "["))
	fallback := open != "" && text == ""
	link := func(target string) string {
		if open == "" {
			return "](" + target + ")"
		}
		return "[" + text + "](" + target + ")"
	}

	anchor := linkAnchor(l)

	// pure anchor (same-page link) — fall back to the header text if empty
	if l.Path == "" {
		if fallback {
			text = humanizeSlug(l.AnchorText())
		}
		return link(l.Query + anchor + l.Title)
	}

	// empty link text (e.g. "[](path.md#anchor)") — fall back to filename + header
	if fallback {
		text = autoLinkText(l.Path, l.AnchorText())
	}

	// the path is decoded, the app url encodes it so goldmark accepts spaces/unicode; query,
	// anchor and title are kept for every target
	if rel, ok := strings.CutPrefix(l.Path, "media/"); ok {
		query := "?mode=detail"
		if l.Query != "" {
			query = l.Query + "&mode=detail"
		}
		return link(pathutils.ToMediaURL(rel) + query + anchor + l.Title)
	}
	if rel, ok := strings.CutPrefix(strings.TrimPrefix(l.Path, "/files"), "/media/"); ok {
		return link(pathutils.ToMediaURL(rel) + l.Query + anchor + l.Title)
	}
	return link(docLinkDest(l))
}

// docLinkDest writes the app url destination of a link to a docs file (bare or /files/ path).
func docLinkDest(l Link) string {
	return pathutils.ToFileURL(utils.WithDefaultLinkExt(strings.TrimPrefix(l.Path, "/files/"))) + l.Query + linkAnchor(l) + l.Title
}

// linkAnchor is l's anchor as the heading id it names: "#My Section" -> "#my-section", so a
// visible heading text (with spaces) works as anchor; an anchor that already is an id stays as
// written (e.g. percent-encoded). only for markdown targets, other fragments (doc.pdf#page=3)
// stay as written
func linkAnchor(l Link) string {
	if ext := path.Ext(l.Path); ext == "" || ext == ".md" {
		if id := AnchorID(l.AnchorText()); id != "" && id != l.AnchorText() {
			return "#" + id
		}
	}
	return l.Anchor
}

// processRefDefs rewrites the destination of every reference definition [id]: dest outside code
// to a docs file to its /files/ url, like processMarkdownLink - goldmark would resolve it against
// the page. media and images stay as written, an image may use them (rendered from the
// destination like an inline image).
func processRefDefs(content string) string {
	return replaceOutsideCode(content, func(part string, wholeLine bool) string {
		sub := rewriteRefDefRe.FindStringSubmatch(part)
		if !wholeLine || sub == nil {
			return part
		}
		l := ParseLink(sub[2], LinkMarkdown)
		if l.External || l.Path == "" || pathutils.IsMedia(l.Path) || configmanager.IsImageExtension(strings.ToLower(path.Ext(l.Path))) {
			return part
		}
		return sub[1] + docLinkDest(l)
	})
}

// autoLinkText builds a display label for a link left without one, e.g.
// "escrow.md" + "todo-vorlage" -> "escrow - Todo Vorlage".
func autoLinkText(u, anchor string) string {
	name := strings.TrimSuffix(filepath.Base(u), filepath.Ext(u))
	if anchor == "" {
		return name
	}
	return name + " - " + humanizeSlug(anchor)
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

// ExtractLinks returns the path of every link RewriteLinks passes on (it skips external ones: with a
// scheme like http:/mailto:/data: or a host like //cdn...), pure anchors and html root links to
// app routes (/dashboard, /search?q=...) - for html only /media/ and /files/ are files.
// A wiki link is only external with "//" or a leading unc "\\" ([[ns:page]] is a path), a one-letter scheme is a
// windows drive (C:\x.png), so both are still reported as (broken) links.
func (h *MarkdownHandler) ExtractLinks(content []byte) []string {
	var links []string
	// RewriteLinks is only used as the link walker here: the callback collects every path and
	// never replaces one, so the (unchanged) content it returns is discarded
	RewriteLinks(string(content), func(l Link) (string, bool) {
		if l.Path != "" && !IsAppRouteLink(l.Path, l.Kind) {
			links = append(links, l.Path)
		}
		return "", false
	})
	return links
}

// externalLinkRe matches a scheme (more than one letter, a single one is a windows drive) or a
// //host or a windows \\server unc path - not url.Parse, which fails on an external url with a
// bad escape like "100%"
var externalLinkRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]+:|//|\\\\)`)

func isExternalLink(p string, kind LinkKind) bool {
	if kind == LinkWiki {
		return strings.Contains(p, "//") || strings.HasPrefix(p, `\\`)
	}
	return externalLinkRe.MatchString(p)
}

func (h *MarkdownHandler) Name() string {
	return "markdown"
}

// htmlSanitizePolicy is the allowlist sanitizeHTML applies to rendered HTML, so
// content files can't execute JavaScript in the browser via the
// <details>/<summary> wrapper tags that bypass goldmark's own raw-HTML escaping
// (see extractHTMLBlocks). bluemonday.UGCPolicy covers standard prose markup
// (headings, lists, tables, details/summary, images); the rest allows the
// app's own generated markup - todo/edit icons, htmx loaders, section ids -
// that the node renderer emits inline alongside it.
var htmlSanitizePolicy = newHTMLSanitizePolicy()

func newHTMLSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(false) // these are the user's own internal/external links, not UGC spam to discourage
	p.AllowStyling()                // class="..." on every element
	p.AllowAttrs("type").Matching(regexp.MustCompile(`(?i)^(button|submit|reset)$`)).OnElements("button")
	p.AllowAttrs("aria-hidden").Matching(regexp.MustCompile(`(?i)^(true|false)$`)).OnElements("a")
	p.AllowAttrs("hx-get", "hx-trigger", "hx-swap").OnElements("div", "span")
	p.AllowAttrs("data-line").Matching(bluemonday.Integer).OnElements("span")
	p.AllowAttrs("alt").OnElements("img") // UGCPolicy's alt regex is too strict for arbitrary alt text
	// heading/section ids may be non-ASCII (e.g. a CJK-only heading slug), wider than UGCPolicy's default id regex
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[\p{L}\p{N}:_.-]+$`)).Globally()
	return p
}

func sanitizeHTML(html string) string {
	return htmlSanitizePolicy.Sanitize(html)
}
