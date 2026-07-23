package pdfexport

const (
	marginMM   = 20.0
	baseFontPt = 11.0
	cellPadMM  = 2.0
)

// style carries the inline formatting for a single token of text.
type style struct {
	bold, italic, code, link bool
	size                     float64 // pt
	href                     string
}

func (s style) fontFamily() string {
	if s.code {
		return "Courier"
	}
	return "Arial"
}

func (s style) fontStyle() string {
	fs := ""
	if s.bold {
		fs += "B"
	}
	if s.italic {
		fs += "I"
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
	return style{bold: true, size: size}
}

func codeBlockStyle() style { return style{code: true, size: baseFontPt - 1} }

// token is a single word (plus its trailing punctuation) with its style.
type token struct {
	text  string
	style style
}
