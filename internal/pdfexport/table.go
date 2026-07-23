package pdfexport

import (
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// ---------------------------------------------------------------------------
// tables — column widths are computed from actual content instead of a fixed
// header-only width (the bug that originally motivated dropping mdtopdf):
// each column gets its natural (single-line) width and its minimum (longest
// unbreakable word) width, then shrinks proportionally to fit the page
// without ever going below a column's minimum.
// ---------------------------------------------------------------------------

type tableCell struct {
	tokens []token
}

func sumWidths(widths []float64) float64 {
	sum := 0.0
	for _, w := range widths {
		sum += w
	}
	return sum
}

func (r *renderer) renderTable(t *extast.Table) {
	var header *extast.TableHeader
	var rows []*extast.TableRow
	for c := t.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *extast.TableHeader:
			header = v
		case *extast.TableRow:
			rows = append(rows, v)
		}
	}
	if header == nil {
		return
	}

	headerCells := r.tableCellsOf(header)
	numCols := len(headerCells)
	if numCols == 0 {
		return
	}
	bodyCells := make([][]tableCell, len(rows))
	for i, row := range rows {
		bodyCells[i] = r.tableCellsOf(row)
	}

	colWidths := make([]float64, numCols)
	minWidths := make([]float64, numCols)
	for i, c := range headerCells {
		colWidths[i] = r.naturalWidth(c.tokens, true)
		minWidths[i] = r.minColumnWidth(c.tokens, true)
	}
	for _, row := range bodyCells {
		for i, c := range row {
			if i >= numCols {
				continue
			}
			if w := r.naturalWidth(c.tokens, false); w > colWidths[i] {
				colWidths[i] = w
			}
			if w := r.minColumnWidth(c.tokens, false); w > minWidths[i] {
				minWidths[i] = w
			}
		}
	}

	avail := r.contentWidth()
	if sumWidths(colWidths) > avail {
		// shrink flexible (wrappable) space first so no column drops below the
		// width its longest unbreakable word needs. Iterative because clamping
		// a column to its minimum changes how much the remaining columns must
		// absorb; repeat until it fits or every column is already at its floor.
		for range colWidths {
			deficit := sumWidths(colWidths) - avail
			if deficit <= 0 {
				break
			}
			flexTotal := 0.0
			for i := range colWidths {
				if flex := colWidths[i] - minWidths[i]; flex > 0 {
					flexTotal += flex
				}
			}
			if flexTotal <= 0 {
				break
			}
			for i := range colWidths {
				flex := colWidths[i] - minWidths[i]
				if flex <= 0 {
					continue
				}
				colWidths[i] -= deficit * flex / flexTotal
				if colWidths[i] < minWidths[i] {
					colWidths[i] = minWidths[i]
				}
			}
		}
	} else if total := sumWidths(colWidths); total > 0 {
		extra := (avail - total) / float64(numCols)
		for i := range colWidths {
			colWidths[i] += extra
		}
	}

	r.drawTableRow(headerCells, colWidths, true)
	for _, row := range bodyCells {
		r.drawTableRow(row, colWidths, false)
	}
	r.pdf.Ln(4)
}

func (r *renderer) tableCellsOf(row ast.Node) []tableCell {
	var cells []tableCell
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		cells = append(cells, tableCell{tokens: r.collectTokens(c, normalStyle())})
	}
	return cells
}

// naturalWidth returns the width (mm) needed to display tokens on a single
// line, clamped to a sane min/max so one long cell can't blow out the table.
func (r *renderer) naturalWidth(tokens []token, header bool) float64 {
	w := 0.0
	for i, tok := range tokens {
		st := tok.style
		if header {
			st.bold = true
		}
		r.applyStyle(st)
		if i > 0 {
			w += r.pdf.GetStringWidth(" ")
		}
		w += r.pdf.GetStringWidth(r.translate(tok.text))
	}
	w += 2 * cellPadMM
	if w < 18 {
		w = 18
	}
	if w > 70 {
		w = 70
	}
	return w
}

// minColumnWidth returns the width (mm) needed for the single longest word in
// tokens — a column can never be shrunk below this without the word overflowing
// into the next column.
func (r *renderer) minColumnWidth(tokens []token, header bool) float64 {
	w := 0.0
	for _, tok := range tokens {
		st := tok.style
		if header {
			st.bold = true
		}
		r.applyStyle(st)
		if tw := r.pdf.GetStringWidth(r.translate(tok.text)); tw > w {
			w = tw
		}
	}
	w += 2 * cellPadMM
	if w < 18 {
		w = 18
	}
	return w
}

func (r *renderer) drawTableRow(cells []tableCell, widths []float64, header bool) {
	lineH := normalStyle().lineHeight()
	linesPerCell := make([][][]token, len(widths))
	maxLines := 1
	for i := range widths {
		var toks []token
		if i < len(cells) {
			toks = cells[i].tokens
		}
		if header {
			bolded := make([]token, len(toks))
			for j, t := range toks {
				t.style.bold = true
				bolded[j] = t
			}
			toks = bolded
		}
		linesPerCell[i] = r.wrapTokens(toks, widths[i]-2*cellPadMM)
		if len(linesPerCell[i]) > maxLines {
			maxLines = len(linesPerCell[i])
		}
	}

	rowHeight := float64(maxLines)*lineH + 2*cellPadMM
	r.ensureSpace(rowHeight)

	x0, y0 := marginMM+r.indent, r.pdf.GetY()
	if header {
		r.pdf.SetFillColor(220, 220, 220)
	} else {
		r.pdf.SetFillColor(255, 255, 255)
	}
	r.pdf.SetDrawColor(160, 160, 160)

	x := x0
	for i, w := range widths {
		r.pdf.Rect(x, y0, w, rowHeight, "DF")
		ty := y0 + cellPadMM
		for _, line := range linesPerCell[i] {
			tx := x + cellPadMM
			for j, tok := range line {
				r.applyStyle(tok.style)
				text := r.translate(tok.text)
				tw := r.pdf.GetStringWidth(text)
				if j > 0 {
					tx += r.pdf.GetStringWidth(" ")
				}
				r.pdf.SetXY(tx, ty)
				r.pdf.CellFormat(tw, lineH, text, "", 0, "L", false, 0, "")
				tx += tw
			}
			ty += lineH
		}
		x += w
	}
	r.pdf.SetXY(x0, y0+rowHeight)
}
