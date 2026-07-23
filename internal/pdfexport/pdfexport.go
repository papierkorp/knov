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

// MarkdownToPDF renders markdown source to a PDF document.
func MarkdownToPDF(markdown []byte) ([]byte, error) {
	logging.LogDebug(logging.KeyPdfExport, "pdf export: converting %d bytes of markdown", len(markdown))

	source := []byte(parser.ResolveWikiLinks(string(markdown)))

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := md.Parser().Parse(text.NewReader(source))

	r := newRenderer(source)
	r.renderChildren(root)

	var buf bytes.Buffer
	if err := r.pdf.Output(&buf); err != nil {
		logging.LogError(logging.KeyPdfExport, "pdf export: fpdf output failed: %v", err)
		return nil, err
	}

	logging.LogDebug(logging.KeyPdfExport, "pdf export: produced %d byte pdf", buf.Len())
	return buf.Bytes(), nil
}
