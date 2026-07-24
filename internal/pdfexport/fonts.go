package pdfexport

import (
	"strings"

	"github.com/go-pdf/fpdf"
)

// customFont holds the TrueType bytes for a font family's available style
// variants ("" | "B" | "I" | "BI" -> bytes). Registered once at startup via
// RegisterFont; a fresh *fpdf.Fpdf still needs each variant added to it
// individually per document — see registerCustomFonts.
type customFont struct {
	styles map[string][]byte
}

//nolint:gochecknoglobals
var customFonts = map[string]customFont{}

// RegisterFont registers a custom TrueType font family for use in the pdf
// export font settings (Options.FontOverall/FontCodeBlock/FontHeadings/FontH1).
// A family with no regular bytes is skipped entirely — feeding fpdf empty
// font data would poison the whole document later. bold/italic/boldItalic
// may be nil if the family doesn't ship that style (e.g. Noto Sans Mono has
// no italic) — text needing a missing style falls back to the nearest
// available one instead of silently failing to render (see resolveFontStyle).
func RegisterFont(family string, regular, bold, italic, boldItalic []byte) {
	if len(regular) == 0 {
		return
	}
	styles := map[string][]byte{"": regular}
	if len(bold) > 0 {
		styles["B"] = bold
	}
	if len(italic) > 0 {
		styles["I"] = italic
	}
	if len(boldItalic) > 0 {
		styles["BI"] = boldItalic
	}
	customFonts[family] = customFont{styles: styles}
}

// registerCustomFonts adds every style variant available for family to pdf.
// A no-op if family wasn't registered via RegisterFont (e.g. it's a core
// font like "Arial"). Must be called per *fpdf.Fpdf instance — font
// registration doesn't carry over between documents.
func registerCustomFonts(pdf *fpdf.Fpdf, family string) {
	cf, ok := customFonts[family]
	if !ok {
		return
	}
	for st, data := range cf.styles {
		pdf.AddUTF8FontFromBytes(family, st, data)
	}
}

// resolveFontStyle returns the closest style actually registered for family,
// preserving a trailing strikeout flag ("S", drawn by fpdf itself regardless
// of font) while falling back bold-only if italic isn't available, then
// plain regular. Core fonts (not in customFonts) are returned unchanged —
// fpdf synthesizes all four core styles itself.
func resolveFontStyle(family, want string) string {
	cf, ok := customFonts[family]
	if !ok {
		return want
	}
	strike := strings.HasSuffix(want, "S")
	base := strings.TrimSuffix(want, "S")
	if _, ok := cf.styles[base]; !ok {
		if _, ok := cf.styles["B"]; ok && strings.Contains(base, "B") {
			base = "B"
		} else {
			base = ""
		}
	}
	if strike {
		base += "S"
	}
	return base
}
