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
