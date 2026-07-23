package pdfexport

import (
	"fmt"
	"strings"

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
		if v.Level == 1 && r.hasContent {
			r.pdf.AddPage()
			r.hasContent = false
		}
		r.writeParagraph(r.collectTokens(n, headingStyle(v.Level)), 3)
	case *ast.Paragraph:
		r.writeParagraph(r.collectTokens(n, normalStyle()), 3)
	case *ast.TextBlock:
		r.writeParagraph(r.collectTokens(n, normalStyle()), 1)
	case *ast.Blockquote:
		r.indent += 8
		r.renderChildren(n)
		r.indent -= 8
	case *ast.List:
		r.renderList(v)
	case *ast.CodeBlock:
		r.writeCodeBlock(string(v.Text(r.source)))
	case *ast.FencedCodeBlock:
		r.writeCodeBlock(string(v.Text(r.source)))
	case *ast.ThematicBreak:
		r.horizontalRule()
	case *extast.Table:
		r.renderTable(v)
	default:
		r.renderChildren(n)
	}
	r.hasContent = true
}

func (r *renderer) horizontalRule() {
	y := r.pdf.GetY() + 2
	x0 := marginMM + r.indent
	x1 := marginMM + r.contentWidth()
	r.pdf.SetDrawColor(150, 150, 150)
	r.pdf.Line(x0, y, x1, y)
	r.pdf.Ln(6)
}

func (r *renderer) writeCodeBlock(text string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	st := codeBlockStyle()
	lh := st.lineHeight()
	leftX := marginMM + r.indent
	width := r.contentWidth()

	r.applyStyle(st)
	innerWidth := width - 2*cellPadMM
	charW := r.pdf.GetStringWidth("M") // Courier is monospace: every char is this wide
	maxChars := int(innerWidth / charW)
	if maxChars < 1 {
		maxChars = 1
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		runes := []rune(line)
		for len(runes) > maxChars {
			lines = append(lines, string(runes[:maxChars]))
			runes = runes[maxChars:]
		}
		lines = append(lines, string(runes))
	}

	// Draw in page-sized chunks: a code block taller than one page can't get a
	// single Rect for its background, and the cursor position afterwards must
	// come from the real, current page — not arithmetic from a stale starting
	// point that pagination has since moved past.
	_, pageH := r.pdf.GetPageSize()
	i := 0
	for i < len(lines) {
		avail := pageH - marginMM - r.pdf.GetY() - 2*cellPadMM
		fit := int(avail / lh)
		if fit < 1 {
			r.pdf.AddPage()
			fit = int((pageH - 2*marginMM - 2*cellPadMM) / lh)
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
		r.pdf.SetFillColor(240, 240, 240)
		r.pdf.Rect(leftX, y0, width, chunkHeight, "F")

		for _, line := range chunk {
			r.applyStyle(st)
			r.pdf.SetX(leftX + cellPadMM)
			r.pdf.CellFormat(innerWidth, lh, r.translate(line), "", 2, "L", false, 0, "")
		}
		r.pdf.SetXY(leftX, y0+chunkHeight)
		i = end
	}
	r.pdf.Ln(3)
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
			r.pdf.SetX(marginMM + r.indent)
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
