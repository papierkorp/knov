package pdfexport

import "github.com/go-pdf/fpdf"

// renderer walks a goldmark AST and draws it onto a PDF document. Block-level
// dispatch lives in blocks.go, inline tokenization in inline.go, paragraph
// word-wrap in layout.go, and tables in table.go.
type renderer struct {
	pdf            *fpdf.Fpdf
	source         []byte
	translate      func(string) string
	indent         float64
	hasContent     bool // true once anything has been drawn on the current page
	opts           Options
	margin         float64
	iconFontLoaded bool
}

func newRenderer(source []byte, opts Options) *renderer {
	format := opts.PageFormat
	if format == "" {
		format = defaultPageFormat
	}
	orientation := opts.Orientation
	if orientation == "" {
		orientation = defaultOrientation
	}
	margin := opts.MarginMM
	if margin <= 0 {
		margin = defaultMarginMM
	}

	pdf := fpdf.New(orientation, "mm", format, ".")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)
	pdf.AddPage()
	r := &renderer{pdf: pdf, source: source, opts: opts, margin: margin}
	r.translate = pdf.UnicodeTranslatorFromDescriptor("")

	if len(iconFontData) > 0 {
		pdf.AddUTF8FontFromBytes(iconFontFamily, "", iconFontData)
		if pdf.Ok() {
			r.iconFontLoaded = true
		} else {
			pdf.ClearError()
		}
	}

	return r
}

func (r *renderer) contentWidth() float64 {
	w, _ := r.pdf.GetPageSize()
	return w - 2*r.margin - r.indent
}

func (r *renderer) applyStyle(st style) {
	if st.isIcon && r.iconFontLoaded {
		r.pdf.SetFont(iconFontFamily, "", st.size)
		r.pdf.SetTextColor(st.iconColor[0], st.iconColor[1], st.iconColor[2])
		return
	}
	r.pdf.SetFont(st.fontFamily(), st.fontStyle(), st.size)
	switch {
	case st.link:
		r.pdf.SetTextColor(30, 100, 200)
	case st.code:
		r.pdf.SetTextColor(120, 40, 40)
	case st.textColor != [3]int{}:
		r.pdf.SetTextColor(st.textColor[0], st.textColor[1], st.textColor[2])
	default:
		r.pdf.SetTextColor(0, 0, 0)
	}
}

// textFor returns the drawable string for tok. Icon glyphs must bypass
// translate(): that mapping targets the Latin-1-ish encoding used by the
// core Arial/Courier fonts, and would corrupt a Private Use Area codepoint
// from the embedded UTF-8 icon font.
func (r *renderer) textFor(tok token) string {
	if tok.style.isIcon {
		return tok.text
	}
	return r.translate(tok.text)
}

// ensureSpace forces a page break first if height won't fit on the rest of
// the current page. Needed before raw Rect-drawn blocks (code blocks, table
// rows), which bypass fpdf's own per-Cell auto page break.
func (r *renderer) ensureSpace(height float64) {
	_, pageH := r.pdf.GetPageSize()
	if r.pdf.GetY()+height > pageH-r.margin {
		r.pdf.AddPage()
	}
}
