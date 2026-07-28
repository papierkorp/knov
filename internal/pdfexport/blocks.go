package pdfexport

import (
	"fmt"
	"strings"

	"knov/internal/parser"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// ---------------------------------------------------------------------------
// block-level rendering
// ---------------------------------------------------------------------------

func (r *renderer) renderChildren(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		r.renderBlock(c)
	}
}

// renderBlock dispatches a single block-level node to its drawing routine.
// Add a case here for any new block-level markdown/HTML construct — each one
// is self-contained and doesn't touch the others.
func (r *renderer) renderBlock(n ast.Node) {
	switch v := n.(type) {
	case *ast.Heading:
		if v.Level == 1 && r.hasContent && r.opts.PageBreakBeforeHeadings {
			r.pdf.AddPage()
			r.hasContent = false
		}
		r.writeParagraph(r.collectTokens(n, headingStyle(v.Level)), 3)
	case *ast.Paragraph:
		if img, ok := soleImage(v); ok {
			r.renderImage(img)
		} else {
			r.writeParagraph(r.collectTokens(n, normalStyle()), 3)
		}
	case *ast.TextBlock:
		r.writeParagraph(r.collectTokens(n, normalStyle()), 1)
	case *ast.Blockquote:
		// margin top
		if r.hasContent {
			r.pdf.Ln(3)
		}
		x := r.margin + r.indent
		y0 := r.pdf.GetY()
		r.indent += 8
		r.renderChildren(n)
		r.indent -= 8
		if y1 := r.pdf.GetY(); y1 > y0 {
			r.pdf.SetFillColor(150, 150, 150)
			r.pdf.Rect(x, y0, 1.5, y1-y0, "F")
		}
	case *ast.List:
		r.renderList(v)
	case *ast.CodeBlock:
		r.writeCodeBlock(string(v.Text(r.source)), "")
	case *ast.FencedCodeBlock:
		lang := ""
		if info := v.Info; info != nil {
			lang = strings.TrimSpace(string(info.Segment.Value(r.source)))
		}
		r.writeCodeBlock(string(v.Text(r.source)), lang)
	case *ast.ThematicBreak:
		r.horizontalRule()
	case *extast.Table:
		r.renderTable(v)
	default:
		r.renderChildren(n)
	}
	r.hasContent = true
}

// soleImage reports whether n's only child is an image, i.e. an image on its
// own line — the common case that should be drawn as a real embedded image
// rather than inline placeholder text.
func soleImage(n ast.Node) (*ast.Image, bool) {
	img, ok := n.FirstChild().(*ast.Image)
	if ok && img == n.LastChild() {
		return img, true
	}
	return nil, false
}

func (r *renderer) horizontalRule() {
	y := r.pdf.GetY() + 2
	x0 := r.margin + r.indent
	x1 := r.margin + r.contentWidth()
	r.pdf.SetDrawColor(150, 150, 150)
	r.pdf.Line(x0, y, x1, y)
	r.pdf.Ln(6)
}

// codeRun is a single colored, monospace fragment of a wrapped code line —
// the drawing-time counterpart of parser.HighlightToken once split across
// line breaks and the hard character wrap.
type codeRun struct {
	text         string
	color        [3]int
	bold, italic bool
}

func (r *renderer) writeCodeBlock(text, language string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	st := codeBlockStyle()
	lh := st.lineHeight()
	leftX := r.margin + r.indent
	width := r.contentWidth()

	r.applyStyle(st)
	innerWidth := width - 2*cellPadMM
	charW := r.pdf.GetStringWidth("M") // code fonts are monospace-only (see the fonts manifest): every char is this wide
	maxChars := int(innerWidth / charW)
	if maxChars < 1 {
		maxChars = 1
	}

	toks := []parser.HighlightToken{{Text: text}}
	bg := [3]int{240, 240, 240}
	if r.opts.SyntaxHighlighting {
		toks = parser.HighlightTokens(text, language)
		bg = parser.HighlightBackground()
	}
	lines := wrapHighlightedCode(toks, maxChars)

	// Draw in page-sized chunks: a code block taller than one page can't get a
	// single Rect for its background, and the cursor position afterwards must
	// come from the real, current page — not arithmetic from a stale starting
	// point that pagination has since moved past.
	_, pageH := r.pdf.GetPageSize()
	i := 0
	for i < len(lines) {
		avail := pageH - r.margin - r.pdf.GetY() - 2*cellPadMM
		fit := int(avail / lh)
		if fit < 1 {
			r.pdf.AddPage()
			fit = int((pageH - 2*r.margin - 2*cellPadMM) / lh)
			if fit < 1 {
				fit = 1
			}
		}
		end := i + fit
		if end > len(lines) {
			end = len(lines)
		}
		chunk := lines[i:end]

		chunkHeight := float64(len(chunk))*lh + 2*cellPadMM
		y0 := r.pdf.GetY()
		r.pdf.SetFillColor(bg[0], bg[1], bg[2])
		r.pdf.Rect(leftX, y0, width, chunkHeight, "F")

		y := y0 + cellPadMM
		for _, line := range chunk {
			x := leftX + cellPadMM
			for _, run := range line {
				rst := st
				rst.bold, rst.italic, rst.textColor = run.bold, run.italic, run.color
				r.applyStyle(rst)
				w := r.pdf.GetStringWidth(r.transformText(run.text, rst))
				r.pdf.SetXY(x, y)
				r.pdf.CellFormat(w, lh, r.transformText(run.text, rst), "", 0, "L", false, 0, "")
				x += w
			}
			y += lh
		}
		r.pdf.SetXY(leftX, y0+chunkHeight)
		i = end
	}
	r.pdf.Ln(3)
}

// wrapHighlightedCode splits highlighted tokens into physical lines (at "\n"
// boundaries in the token text, then again at maxChars), preserving each
// fragment's color/weight as a codeRun.
func wrapHighlightedCode(toks []parser.HighlightToken, maxChars int) [][]codeRun {
	var lines [][]codeRun
	var cur []codeRun
	count := 0

	emit := func(text string, tok parser.HighlightToken) {
		for len(text) > 0 {
			if count >= maxChars {
				lines = append(lines, cur)
				cur = nil
				count = 0
			}
			runes := []rune(text)
			n := maxChars - count
			if n > len(runes) {
				n = len(runes)
			}
			cur = append(cur, codeRun{text: string(runes[:n]), color: tok.Color, bold: tok.Bold, italic: tok.Italic})
			count += n
			text = string(runes[n:])
		}
	}

	for _, tok := range toks {
		parts := strings.Split(tok.Text, "\n")
		for i, part := range parts {
			if part != "" {
				emit(part, tok)
			}
			if i < len(parts)-1 {
				lines = append(lines, cur)
				cur = nil
				count = 0
			}
		}
	}
	lines = append(lines, cur)
	return lines
}

func (r *renderer) renderList(list *ast.List) {
	i := list.Start
	if i == 0 {
		i = 1
	}
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		item, ok := c.(*ast.ListItem)
		if !ok {
			continue
		}
		marker := "-"
		if list.IsOrdered() {
			marker = fmt.Sprintf("%d.", i)
			i++
		}
		r.renderListItem(item, marker)
	}
}

func (r *renderer) renderListItem(item *ast.ListItem, marker string) {
	st := normalStyle()
	r.applyStyle(st)
	markerWidth := r.pdf.GetStringWidth(marker) + 3

	first := true
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		if nested, ok := c.(*ast.List); ok {
			r.indent += markerWidth + 2
			r.renderList(nested)
			r.indent -= markerWidth + 2
			continue
		}

		tokens := r.collectTokens(c, normalStyle())
		if first {
			r.applyStyle(st)
			r.pdf.SetX(r.margin + r.indent)
			r.pdf.CellFormat(markerWidth, st.lineHeight(), marker, "", 0, "L", false, 0, "")
			r.indent += markerWidth
			r.writeParagraph(tokens, 1)
			r.indent -= markerWidth
			first = false
		} else {
			r.indent += markerWidth
			r.writeParagraph(tokens, 1)
			r.indent -= markerWidth
		}
	}
}
