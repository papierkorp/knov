// Package pdfexport converts markdown source into PDF documents.
package pdfexport

import (
	"bytes"

	"knov/internal/parser"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// MarkdownToPDF renders markdown source to a PDF document.
func MarkdownToPDF(markdown []byte) ([]byte, error) {
	source := []byte(parser.ResolveWikiLinks(string(markdown)))

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := md.Parser().Parse(text.NewReader(source))

	r := newRenderer(source)
	r.renderChildren(root)

	var buf bytes.Buffer
	if err := r.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
