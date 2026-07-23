package pdfexport

import "github.com/go-pdf/fpdf"

// renderer walks a goldmark AST and draws it onto a PDF document. Block-level
// dispatch lives in blocks.go, inline tokenization in inline.go, paragraph
// word-wrap in layout.go, and tables in table.go.
type renderer struct {
	pdf        *fpdf.Fpdf
	source     []byte
	translate  func(string) string
	indent     float64
	hasContent bool // true once anything has been drawn on the current page
}

func newRenderer(source []byte) *renderer {
	pdf := fpdf.New("P", "mm", "A4", ".")
	pdf.SetMargins(marginMM, marginMM, marginMM)
	pdf.SetAutoPageBreak(true, marginMM)
	pdf.AddPage()
	r := &renderer{pdf: pdf, source: source}
	r.translate = pdf.UnicodeTranslatorFromDescriptor("")
	return r
}

func (r *renderer) contentWidth() float64 {
	w, _ := r.pdf.GetPageSize()
	return w - 2*marginMM - r.indent
}

func (r *renderer) applyStyle(st style) {
	r.pdf.SetFont(st.fontFamily(), st.fontStyle(), st.size)
	switch {
	case st.link:
		r.pdf.SetTextColor(30, 100, 200)
	case st.code:
		r.pdf.SetTextColor(120, 40, 40)
	default:
		r.pdf.SetTextColor(0, 0, 0)
	}
}

// ensureSpace forces a page break first if height won't fit on the rest of
// the current page. Needed before raw Rect-drawn blocks (code blocks, table
// rows), which bypass fpdf's own per-Cell auto page break.
func (r *renderer) ensureSpace(height float64) {
	_, pageH := r.pdf.GetPageSize()
	if r.pdf.GetY()+height > pageH-marginMM {
		r.pdf.AddPage()
	}
}
