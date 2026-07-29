package pdfexport

import (
	"os"
	"path/filepath"
	"regexp"
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

// resolveImage registers a local media image with fpdf for later drawing via
// ImageOptions, returning the registered image name and fpdf image type.
// False if the extension is unsupported or the file can't be read/decoded.
func (r *renderer) resolveImage(dest string) (name, tp string, ok bool) {
	tp, ok = imageExt[strings.ToLower(filepath.Ext(dest))]
	if !ok {
		return "", "", false
	}
	fullPath := pathutils.ToMediaPath(dest)

	f, err := os.Open(fullPath)
	if err != nil {
		logging.LogWarning(logging.KeyPdfExport, "pdf export: image not found: %s", fullPath)
		return "", "", false
	}
	defer f.Close()

	name = "img:" + fullPath
	r.pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: tp, ReadDpi: true}, f)
	if r.pdf.Err() {
		logging.LogWarning(logging.KeyPdfExport, "pdf export: failed to decode image: %s", fullPath)
		r.pdf.ClearError()
		return "", "", false
	}
	return name, tp, true
}

func (r *renderer) embedImage(dest string) bool {
	name, tp, ok := r.resolveImage(dest)
	if !ok {
		return false
	}
	x := r.margin + r.indent
	r.pdf.ImageOptions(name, x, r.pdf.GetY(), r.contentWidth(), 0, true, fpdf.ImageOptions{ImageType: tp}, 0, "")
	r.pdf.Ln(3)
	return true
}

// zoneImageLinkRe matches a header/footer zone template that is nothing but
// a markdown image link (e.g. "![alt](media/logo.png)"), letting a zone
// embed a small image instead of drawing text.
var zoneImageLinkRe = regexp.MustCompile(`^!\[[^\]]*\]\(([^)]+)\)$`)

// zoneImageHeightMM is the height a header/footer zone image is scaled to
// (width follows from its aspect ratio), sized to fit inside the 10mm zone
// row alongside drawZone's vertical centering.
const zoneImageHeightMM float64 = 8

// drawZoneImage embeds dest as an image inside the zone spanning
// [x, x+width) at row y..y+10, aligned per alignStr ("L", "C" or "R") and
// scaled to zoneImageHeightMM tall (or narrower, if that would overflow the
// zone width). Returns false if dest can't be embedded — including external
// URLs, which don't resolve under the local media root.
func (r *renderer) drawZoneImage(dest string, x, y, width float64, alignStr string) bool {
	if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
		return false
	}
	name, tp, ok := r.resolveImage(dest)
	if !ok {
		return false
	}
	info := r.pdf.GetImageInfo(name)
	h := zoneImageHeightMM
	w := h * info.Width() / info.Height()
	if w > width {
		w = width
		h = w * info.Height() / info.Width()
	}
	imgX := x
	switch alignStr {
	case "C":
		imgX = x + (width-w)/2
	case "R":
		imgX = x + width - w
	}
	r.pdf.ImageOptions(name, imgX, y+(10-h)/2, w, h, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
	return true
}
