package pdfexport

const (
	defaultMarginMM    = 20.0
	defaultPageFormat  = "A4"
	defaultOrientation = "P"
	baseFontPt         = 11.0
	cellPadMM          = 2.0
)

// style carries the inline formatting for a single token of text.
type style struct {
	bold, italic, code, link, strike bool
	size                             float64 // pt
	href                             string

	// heading is the source heading level (1-6) for tokens from a heading
	// block, 0 otherwise. Used only to pick a font family — see
	// renderer.fontFamilyFor.
	heading int

	// isIcon marks a token as a task-status glyph drawn with the embedded icon
	// font (see icons.go) instead of the normal text font.
	isIcon    bool
	iconColor [3]int // RGB used only when isIcon is set

	// textColor overrides the default black; zero value means "use default".
	textColor [3]int
}

func (s style) fontStyle() string {
	fs := ""
	if s.bold {
		fs += "B"
	}
	if s.italic {
		fs += "I"
	}
	if s.strike {
		fs += "S"
	}
	return fs
}

// lineHeight approximates a readable line height in mm for the given point size.
func (s style) lineHeight() float64 {
	return s.size * 0.3528 * 1.35
}

func normalStyle() style { return style{size: baseFontPt} }

func headingStyle(level int) style {
	sizes := map[int]float64{1: 22, 2: 18, 3: 15, 4: 13, 5: 12, 6: 11}
	size, ok := sizes[level]
	if !ok {
		size = 11
	}
	return style{bold: true, size: size, heading: level}
}

func codeBlockStyle() style { return style{code: true, size: baseFontPt - 1} }

// token is a single word (plus its trailing punctuation) with its style.
type token struct {
	text  string
	style style
	// breakLine marks a forced line break (a soft or hard line break in the
	// markdown source, e.g. a line ending in the middle of a paragraph)
	// rather than a drawable word. text is empty when set.
	breakLine bool
}
