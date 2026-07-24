package pdfexport

import (
	"os"
	"path/filepath"
	"strings"

	"knov/internal/logging"
	"knov/internal/pathutils"

	"github.com/go-pdf/fpdf"
	"github.com/yuin/goldmark/ast"
)

// imageExt maps supported extensions to the fpdf image type. Formats fpdf
// can't decode (svg, webp, ...) fall back to the alt-text placeholder.
var imageExt = map[string]string{
	".png":  "png",
	".jpg":  "jpg",
	".jpeg": "jpg",
	".gif":  "gif",
}

// renderImage draws a block-level image (a paragraph containing only an
// image) at full content width, scaled down to keep its aspect ratio. Local
// media images are embedded; everything else (external URLs, unreadable or
// unsupported files) falls back to a bracketed alt-text placeholder.
func (r *renderer) renderImage(img *ast.Image) {
	dest := string(img.Destination)
	external := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")
	if external || !r.embedImage(dest) {
		alt := plainText(img, r.source)
		r.writeParagraph(splitWords("[image: "+alt+"]", normalStyle()), 3)
	}
}

func (r *renderer) embedImage(dest string) bool {
	tp, ok := imageExt[strings.ToLower(filepath.Ext(dest))]
	if !ok {
		return false
	}
	fullPath := pathutils.ToMediaPath(dest)

	f, err := os.Open(fullPath)
	if err != nil {
		logging.LogWarning(logging.KeyPdfExport, "pdf export: image not found: %s", fullPath)
		return false
	}
	defer f.Close()

	name := "img:" + fullPath
	r.pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: tp, ReadDpi: true}, f)
	if r.pdf.Err() {
		logging.LogWarning(logging.KeyPdfExport, "pdf export: failed to decode image: %s", fullPath)
		r.pdf.ClearError()
		return false
	}

	x := r.margin + r.indent
	r.pdf.ImageOptions(name, x, r.pdf.GetY(), r.contentWidth(), 0, true, fpdf.ImageOptions{ImageType: tp}, 0, "")
	r.pdf.Ln(3)
	return true
}
