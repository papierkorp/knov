package pdfexport

import (
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

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
	r := &renderer{pdf: pdf, source: source, opts: opts, margin: margin}
	r.translate = pdf.UnicodeTranslatorFromDescriptor("")

	if opts.FooterLeft != "" || opts.FooterCenter != "" || opts.FooterRight != "" {
		pdf.AliasNbPages("")
		pdf.SetFooterFunc(r.drawFooter)
	}

	pdf.AddPage()

	if len(iconFontData) > 0 {
		pdf.AddUTF8FontFromBytes(iconFontFamily, "", iconFontData)
		if pdf.Ok() {
			r.iconFontLoaded = true
		} else {
			pdf.ClearError()
		}
	}

	for _, family := range []string{
		opts.FontOverall, opts.FontCodeBlock, opts.FontHeadings, opts.FontH1,
		opts.FooterLeftStyle.Font, opts.FooterCenterStyle.Font, opts.FooterRightStyle.Font,
	} {
		registerCustomFonts(pdf, family)
	}

	return r
}

// fontFamilyFor resolves the fpdf font family for st from the configured
// Options, falling back level by level: h1 -> general headings -> overall ->
// the built-in default for that kind of text. Code (blocks and inline spans)
// falls back to Courier rather than the overall font, since it must stay
// monospace for the code-block word-wrap in blocks.go to line up.
func (r *renderer) fontFamilyFor(st style) string {
	if st.code {
		return firstNonEmpty(r.opts.FontCodeBlock, "Courier")
	}
	if st.heading == 1 {
		return firstNonEmpty(r.opts.FontH1, r.opts.FontHeadings, r.opts.FontOverall, "Arial")
	}
	if st.heading > 0 {
		return firstNonEmpty(r.opts.FontHeadings, r.opts.FontOverall, "Arial")
	}
	return firstNonEmpty(r.opts.FontOverall, "Arial")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
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
	family := r.fontFamilyFor(st)
	r.pdf.SetFont(family, resolveFontStyle(family, st.fontStyle()), st.size)
	switch {
	case st.link:
		r.pdf.SetTextColor(30, 100, 200)
	case st.textColor != [3]int{}:
		r.pdf.SetTextColor(st.textColor[0], st.textColor[1], st.textColor[2])
	case st.code:
		r.pdf.SetTextColor(120, 40, 40)
	default:
		r.pdf.SetTextColor(0, 0, 0)
	}
}

// textFor returns the drawable string for tok. See transformText.
func (r *renderer) textFor(tok token) string {
	return r.transformText(tok.text, tok.style)
}

// transformText returns the drawable form of text for style st. Icon glyphs
// and any style whose resolved font is a registered custom TrueType family
// must bypass translate(): that mapping targets the Latin-1-ish encoding
// used by the core Arial/Times/Courier fonts, and would corrupt both a
// Private Use Area codepoint from the embedded icon font and any non-ASCII
// character in a genuine UTF-8 custom font.
func (r *renderer) transformText(text string, st style) string {
	if st.isIcon {
		return text
	}
	return r.transformTextForFamily(text, r.fontFamilyFor(st))
}

// transformTextForFamily is the family-keyed core of transformText, used
// directly by drawFooter since footer zones pick their font independently of
// the block-level style/heading rules in fontFamilyFor.
func (r *renderer) transformTextForFamily(text, family string) string {
	if _, ok := customFonts[family]; ok {
		return text
	}
	return r.translate(text)
}

// drawFooter renders the configured footer zones, resolving {{page}},
// {{pages}} and the tokens in opts.FooterTokens. {{pages}} is replaced with
// fpdf's own page count alias, which fpdf substitutes document-wide when the
// pdf is closed.
func (r *renderer) drawFooter() {
	pairs := []string{"{{page}}", strconv.Itoa(r.pdf.PageNo()), "{{pages}}", "{nb}"}
	for token, val := range r.opts.FooterTokens {
		pairs = append(pairs, "{{"+token+"}}", val)
	}
	replacer := strings.NewReplacer(pairs...)
	pageW, pageH := r.pdf.GetPageSize()
	zoneWidth := (pageW - 2*r.margin) / 3

	if r.opts.FooterRule {
		y := pageH - 15
		r.pdf.SetDrawColor(150, 150, 150)
		r.pdf.Line(r.margin, y, pageW-r.margin, y)
	}

	r.pdf.SetY(-15)
	family := r.applyFooterStyle(r.opts.FooterLeftStyle)
	r.pdf.CellFormat(zoneWidth, 10, r.transformTextForFamily(replacer.Replace(r.opts.FooterLeft), family), "", 0, "L", false, 0, "")
	family = r.applyFooterStyle(r.opts.FooterCenterStyle)
	r.pdf.CellFormat(zoneWidth, 10, r.transformTextForFamily(replacer.Replace(r.opts.FooterCenter), family), "", 0, "C", false, 0, "")
	family = r.applyFooterStyle(r.opts.FooterRightStyle)
	r.pdf.CellFormat(zoneWidth, 10, r.transformTextForFamily(replacer.Replace(r.opts.FooterRight), family), "", 1, "R", false, 0, "")
}

// applyFooterStyle sets the font, size, weight and color for a footer zone
// and returns the resolved font family (for transformTextForFamily).
func (r *renderer) applyFooterStyle(st FooterStyle) string {
	family := firstNonEmpty(st.Font, r.opts.FontOverall, "Arial")
	size := st.Size
	if size <= 0 {
		size = 8
	}
	fontStyle := ""
	if st.Bold {
		fontStyle += "B"
	}
	if st.Italic {
		fontStyle += "I"
	}
	r.pdf.SetFont(family, resolveFontStyle(family, fontStyle), size)
	r.pdf.SetTextColor(st.Color[0], st.Color[1], st.Color[2])
	return family
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
