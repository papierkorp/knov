// Package fonts is the single source of truth for the font families bundled
// under static/fonts, used by both the pdf export settings and the general
// html font settings. The settings registry derives its option lists from
// Families, main.go loads and registers the ttf files listed here, and the
// settings page and html pages derive their @font-face rules from it —
// adding a font means adding one entry here plus its files under
// static/fonts.
package fonts

import (
	"fmt"
	"strings"
)

// Family describes one selectable font family.
type Family struct {
	// Name is the fpdf family name, the settings option value, and the
	// css font-family used for the settings preview.
	Name string
	// Dir is the subdirectory under static/fonts holding the ttf files.
	Dir string
	// Ttf filenames within Dir; empty when the family doesn't ship that
	// style (pdfexport falls back to the nearest available style).
	Regular, Bold, Italic, BoldItalic string
	// Monospace marks families safe for the code block font setting, whose
	// wrapping assumes a fixed character width.
	Monospace bool
}

//nolint:gochecknoglobals
var Families = []Family{
	{Name: "Roboto", Dir: "Roboto", Regular: "Roboto-Regular.ttf", Bold: "Roboto-Bold.ttf", Italic: "Roboto-Italic.ttf", BoldItalic: "Roboto-BoldItalic.ttf"},
	{Name: "PT Serif", Dir: "PT_Serif", Regular: "PTSerif-Regular.ttf", Bold: "PTSerif-Bold.ttf", Italic: "PTSerif-Italic.ttf", BoldItalic: "PTSerif-BoldItalic.ttf"},
	{Name: "Cormorant", Dir: "Cormorant", Regular: "Cormorant-Regular.ttf", Bold: "Cormorant-Bold.ttf", Italic: "Cormorant-Italic.ttf", BoldItalic: "Cormorant-BoldItalic.ttf"},
	{Name: "Courier Prime", Dir: "Courier_Prime", Regular: "CourierPrime-Regular.ttf", Bold: "CourierPrime-Bold.ttf", Italic: "CourierPrime-Italic.ttf", BoldItalic: "CourierPrime-BoldItalic.ttf", Monospace: true},
	{Name: "Science Gothic", Dir: "Science_Gothic", Regular: "ScienceGothic-Regular.ttf", Bold: "ScienceGothic-Bold.ttf"},
	{Name: "Tangerine", Dir: "Tangerine", Regular: "Tangerine-Regular.ttf", Bold: "Tangerine-Bold.ttf"},
	{Name: "Unica One", Dir: "Unica_One", Regular: "UnicaOne-Regular.ttf"},
	{Name: "Bebas Neue", Dir: "Bebas_Neue", Regular: "BebasNeue-Regular.ttf"},
	{Name: "Black Ops One", Dir: "Black_Ops_One", Regular: "BlackOpsOne-Regular.ttf"},
	{Name: "Playwrite NZ Guides", Dir: "Playwrite_NZ_Guides", Regular: "PlaywriteNZGuides-Regular.ttf"},
	{Name: "Fira Code", Dir: "Fira_Code", Regular: "FiraCode-Regular.ttf", Bold: "FiraCode-Bold.ttf", Monospace: true},
	{Name: "Roboto Mono", Dir: "Roboto_Mono", Regular: "RobotoMono-Regular.ttf", Bold: "RobotoMono-Bold.ttf", Italic: "RobotoMono-Italic.ttf", BoldItalic: "RobotoMono-BoldItalic.ttf", Monospace: true},
}

// FontFaceCSS returns @font-face rules for the named families, one rule per
// bundled style so browsers render bold/italic text in the correct weight
// instead of synthesizing it. Duplicate names and families with no bundled
// Dir (core/system fonts) are skipped.
func FontFaceCSS(names ...string) string {
	var css strings.Builder
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, f := range Families {
			if f.Name != name || f.Dir == "" {
				continue
			}
			writeFontFace(&css, f.Name, f.Dir, f.Regular, "normal", "normal")
			writeFontFace(&css, f.Name, f.Dir, f.Bold, "bold", "normal")
			writeFontFace(&css, f.Name, f.Dir, f.Italic, "normal", "italic")
			writeFontFace(&css, f.Name, f.Dir, f.BoldItalic, "bold", "italic")
			break
		}
	}
	return css.String()
}

func writeFontFace(css *strings.Builder, name, dir, file, weight, style string) {
	if file == "" {
		return
	}
	fmt.Fprintf(css, `@font-face{font-family:"%s";src:url("/static/fonts/%s/%s") format("truetype");font-weight:%s;font-style:%s;font-display:swap;}`, name, dir, file, weight, style)
}
