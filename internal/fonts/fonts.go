// Package fonts is the single source of truth for the font families bundled
// under static/fonts (currently used by the pdf export settings, later maybe
// by the general/theme settings too). The settings registry derives its
// option lists from Families, main.go loads and registers the ttf files
// listed here, and the settings page derives its @font-face preview rules
// from it — adding a font means adding one entry here plus its files under
// static/fonts.
package fonts

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
	{Name: "Noto Sans", Dir: "noto-sans", Regular: "NotoSans-Regular.ttf", Bold: "NotoSans-Bold.ttf", Italic: "NotoSans-Italic.ttf", BoldItalic: "NotoSans-BoldItalic.ttf"},
	{Name: "Noto Serif", Dir: "noto-serif", Regular: "NotoSerif-Regular.ttf", Bold: "NotoSerif-Bold.ttf", Italic: "NotoSerif-Italic.ttf", BoldItalic: "NotoSerif-BoldItalic.ttf"},
	{Name: "Noto Sans Mono", Dir: "noto-sans-mono", Regular: "NotoSansMono-Regular.ttf", Bold: "NotoSansMono-Bold.ttf", Monospace: true},
	{Name: "Inter", Dir: "Inter", Regular: "Inter_18pt-Regular.ttf", Bold: "Inter_18pt-Bold.ttf", Italic: "Inter_18pt-Italic.ttf", BoldItalic: "Inter_18pt-BoldItalic.ttf"},
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
	{Name: "Roboto Mono", Dir: "Roboto_Mono", Regular: "RobotoMono-Regular.ttf", Bold: "RobotoMono-Bold.ttf", Italic: "RobotoMono-Italic.ttf", BoldItalic: "RobotoMono-BoldItalic.ttf", Monospace: true},
}
