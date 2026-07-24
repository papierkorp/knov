package pdfexport

// ---------------------------------------------------------------------------
// word-wrap — shared by paragraph flow (writeParagraph, draws as it wraps)
// and table cells (wrapTokens, in table.go, wraps into lines to draw later
// once every column's height is known).
// ---------------------------------------------------------------------------

// writeParagraph word-wraps tokens (each carrying its own style) across lines
// starting at the current indent, then adds spacingAfterMM of vertical gap.
func (r *renderer) writeParagraph(tokens []token, spacingAfterMM float64) {
	if len(tokens) == 0 {
		return
	}
	leftX := r.margin + r.indent
	width := r.contentWidth()
	tokens = r.expandLongTokens(tokens, width)
	r.pdf.SetX(leftX)

	used := 0.0
	lineH := 0.0
	first := true
	for _, tok := range tokens {
		r.applyStyle(tok.style)
		text := r.translate(tok.text)
		w := r.pdf.GetStringWidth(text)
		spaceW := r.pdf.GetStringWidth(" ")

		need := w
		if !first {
			need += spaceW
		}
		if !first && used+need > width {
			r.pdf.Ln(lineH)
			r.pdf.SetX(leftX)
			used = 0
			lineH = 0
			first = true
			need = w
		}
		if !first {
			r.pdf.CellFormat(spaceW, tok.style.lineHeight(), "", "", 0, "L", false, 0, "")
			used += spaceW
		}
		r.applyStyle(tok.style)
		link := ""
		if tok.style.link {
			link = tok.style.href
		}
		r.pdf.CellFormat(w, tok.style.lineHeight(), text, "", 0, "L", false, 0, link)
		used += w
		if tok.style.lineHeight() > lineH {
			lineH = tok.style.lineHeight()
		}
		first = false
	}
	r.pdf.Ln(lineH)
	if spacingAfterMM > 0 {
		r.pdf.Ln(spacingAfterMM)
	}
}

// wrapTokens breaks tokens into lines that each fit within width (mm).
func (r *renderer) wrapTokens(tokens []token, width float64) [][]token {
	tokens = r.expandLongTokens(tokens, width)
	var lines [][]token
	var cur []token
	used := 0.0
	for _, tok := range tokens {
		r.applyStyle(tok.style)
		w := r.pdf.GetStringWidth(r.translate(tok.text))
		spaceW := r.pdf.GetStringWidth(" ")

		need := w
		if len(cur) > 0 {
			need += spaceW
		}
		if len(cur) > 0 && used+need > width {
			lines = append(lines, cur)
			cur = nil
			used = 0
			need = w
		}
		cur = append(cur, tok)
		used += need
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = [][]token{{}}
	}
	return lines
}

// expandLongTokens splits any token wider than width into multiple pieces
// that each fit — a single unbreakable word or URL would otherwise overflow
// the line no matter where it starts, since word-wrap alone can't break it.
func (r *renderer) expandLongTokens(tokens []token, width float64) []token {
	out := make([]token, 0, len(tokens))
	for _, tok := range tokens {
		r.applyStyle(tok.style)
		if width <= 0 || r.pdf.GetStringWidth(r.translate(tok.text)) <= width {
			out = append(out, tok)
			continue
		}
		out = append(out, r.splitLongToken(tok, width)...)
	}
	return out
}

// splitLongToken breaks tok's text into consecutive pieces that each fit
// within width, always advancing by at least one rune so pathologically
// wide single characters can't loop forever.
func (r *renderer) splitLongToken(tok token, width float64) []token {
	r.applyStyle(tok.style)
	runes := []rune(tok.text)
	var out []token
	start := 0
	for start < len(runes) {
		end := start + 1
		for end < len(runes) && r.pdf.GetStringWidth(r.translate(string(runes[start:end+1]))) <= width {
			end++
		}
		out = append(out, token{text: string(runes[start:end]), style: tok.style})
		start = end
	}
	return out
}
