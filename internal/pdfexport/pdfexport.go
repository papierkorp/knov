// Package pdfexport converts markdown source into PDF documents.
package pdfexport

import (
	"bytes"

	"knov/internal/logging"
	"knov/internal/parser"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Options controls document-wide pdf export behaviour.
type Options struct {
	// PageBreakBeforeHeadings starts a new page before each top-level heading.
	PageBreakBeforeHeadings bool
	// PageFormat is an fpdf page size string ("A4", "A3", "A5", "Letter", "Legal"). Empty defaults to "A4".
	PageFormat string
	// Orientation is "P" (portrait) or "L" (landscape). Empty defaults to "P".
	Orientation string
	// MarginMM is the page margin in millimeters on every side. Zero defaults to 20.
	MarginMM float64
	// UseTaskIcons draws task-list checkboxes as status icons instead of plain
	// text marks ("[ ]", "[x]", "[-]", "[O]"). Has no effect if no icon font
	// was registered via SetIconFont.
	UseTaskIcons bool

	// FontOverall is the base fpdf font family for body text. Empty defaults
	// to "Arial". Also the fallback for FontHeadings/FontH1 when they're empty.
	FontOverall string
	// FontCodeBlock is the fpdf font family for code blocks and inline code
	// spans. Empty defaults to "Courier". Non-monospace fonts will misalign
	// code-block wrapping, which assumes a fixed character width.
	FontCodeBlock string
	// FontH1 is the fpdf font family for level-1 headings. Empty falls back
	// to FontHeadings, then FontOverall.
	FontH1 string
	// FontHeadings is the fpdf font family for headings (levels 2-6, and 1 if
	// FontH1 is empty). Empty falls back to FontOverall.
	FontHeadings string

	// FooterLeft, FooterCenter and FooterRight are the templates drawn in the
	// respective footer zone of every page. Each may contain {{page}},
	// {{pages}} and any key present in FooterTokens (e.g. {{date}}).
	// Empty zones are left blank.
	FooterLeft, FooterCenter, FooterRight string
	// FooterTokens maps a token name (without braces) to its resolved value,
	// e.g. "date" -> "{{date}}" substitutions. {{page}} and {{pages}} are
	// handled separately since they vary per page.
	FooterTokens map[string]string
	// FooterLeftStyle, FooterCenterStyle and FooterRightStyle control the
	// font, size, weight and color used to draw the respective footer zone.
	FooterLeftStyle, FooterCenterStyle, FooterRightStyle FooterStyle
	// FooterRule draws a horizontal rule above the footer when true.
	FooterRule bool
}

// FooterStyle controls how a single footer zone is drawn.
type FooterStyle struct {
	// Font is the fpdf font family. Empty falls back to Options.FontOverall, then "Arial".
	Font string
	// Color is the RGB text color. Zero value is black.
	Color [3]int
	// Size is the font size in points. Zero or negative defaults to 8.
	Size         float64
	Bold, Italic bool
}

// MarkdownToPDF renders markdown source to a PDF document.
func MarkdownToPDF(markdown []byte, opts Options) ([]byte, error) {
	logging.LogDebug(logging.KeyPdfExport, "pdf export: converting %d bytes of markdown", len(markdown))

	source := []byte(parser.ResolveWikiLinks(string(markdown)))
	source = parser.PreprocessTodoStates(source)

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := md.Parser().Parse(text.NewReader(source))

	r := newRenderer(source, opts)
	r.renderChildren(root)

	var buf bytes.Buffer
	if err := r.pdf.Output(&buf); err != nil {
		logging.LogError(logging.KeyPdfExport, "pdf export: fpdf output failed: %v", err)
		return nil, err
	}

	logging.LogDebug(logging.KeyPdfExport, "pdf export: produced %d byte pdf", buf.Len())
	return buf.Bytes(), nil
}
