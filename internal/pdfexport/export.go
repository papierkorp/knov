package pdfexport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

// LoadSource returns the markdown to export for filePath - a book's composed
// document, otherwise the file's raw content.
func LoadSource(meta pathutils.MetaPath) ([]byte, error) {
	if files.IsBook(meta) {
		composed, err := book.Compose(meta)
		return []byte(composed), err
	}
	return os.ReadFile(meta.FullPath())
}

// ExportAll renders every markdown file and book to a pdf and passes each to add,
// named by its relative path + ".pdf" (keeping the folder structure). Files that
// fail to load or convert are skipped and counted. report receives the progress, ctx
// cancels between files.
func ExportAll(ctx context.Context, add func(name string, modified time.Time, r io.Reader) error, report func(done, total int)) (skipped int, err error) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return 0, err
	}
	// .book is a markdown extension, so books pass this check too
	allFiles = slices.DeleteFunc(allFiles, func(f files.File) bool { return !parser.IsMarkdownExtension(f.Path.String()) })
	opts := settingsOptions()

	for i, f := range allFiles {
		if ctx.Err() != nil {
			return skipped, ctx.Err()
		}
		report(i, len(allFiles))

		content, err := LoadSource(f.Path)
		if err != nil {
			logging.LogWarning(logging.KeyExport, "pdf export all: skip %s (load failed): %v", f.Path, err)
			skipped++
			continue
		}

		pdf, err := render(opts, f.Path, content)
		if err != nil {
			logging.LogWarning(logging.KeyExport, "pdf export all: skip %s (convert failed): %v", f.Path, err)
			skipped++
			continue
		}

		var modified time.Time
		if info, err := os.Stat(f.Path.FullPath()); err == nil {
			modified = info.ModTime()
		}
		// keep the source extension so e.g. a.md and a.todo don't collide
		if err := add(f.Path.Rel()+".pdf", modified, bytes.NewReader(pdf)); err != nil {
			return skipped, err
		}
	}
	report(len(allFiles), len(allFiles))
	return skipped, nil
}

// RenderFile renders content to a pdf using the pdf settings, with filePath
// resolving the header/footer tokens. Any panic is turned into an error so it
// lands in the app log instead of only the recoverer's stderr output, which is
// easy to miss when the binary runs as a background service.
func RenderFile(filePath pathutils.MetaPath, content []byte) ([]byte, error) {
	return render(settingsOptions(), filePath, content)
}

// render renders content with opts (from settingsOptions), resolving the header/footer tokens
// for filePath - see RenderFile.
func render(opts Options, filePath pathutils.MetaPath, content []byte) (pdf []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: panic during conversion: %v", r)
			err = fmt.Errorf("panic during pdf conversion: %v", r)
		}
	}()
	if opts.FooterLeft != "" || opts.FooterCenter != "" || opts.FooterRight != "" {
		opts.FooterTokens = zoneTokens(filePath)
	}
	if opts.HeaderLeft != "" || opts.HeaderCenter != "" || opts.HeaderRight != "" {
		opts.HeaderTokens = zoneTokens(filePath)
	}
	return MarkdownToPDF(content, filePath.String(), opts)
}

// settingsOptions builds the pdf options from the pdf settings, without the per-file tokens.
func settingsOptions() Options {
	return Options{
		PageBreakBeforeHeadings: configmanager.GetPDFPageBreakBeforeHeadings(),
		PageFormat:              configmanager.GetPDFPageFormat(),
		Orientation:             configmanager.GetPDFOrientation(),
		MarginMM:                configmanager.GetPDFMarginMM(),
		UseTaskIcons:            configmanager.GetPDFUseTaskIcons(),
		SyntaxHighlighting:      configmanager.GetPDFSyntaxHighlighting(),
		FontOverall:             configmanager.GetPDFFontOverall(),
		FontCodeBlock:           configmanager.GetPDFFontCodeBlock(),
		FontHeadings:            configmanager.GetPDFFontHeadings(),
		FontH1:                  configmanager.GetPDFFontH1(),
		FooterLeft:              configmanager.GetPDFFooterLeft(),
		FooterCenter:            configmanager.GetPDFFooterCenter(),
		FooterRight:             configmanager.GetPDFFooterRight(),
		FooterLeftStyle:         zoneStyle(configmanager.GetPDFFooterLeftFont(), configmanager.GetPDFFooterLeftColor(), configmanager.GetPDFFooterLeftSize(), configmanager.GetPDFFooterLeftBold(), configmanager.GetPDFFooterLeftItalic()),
		FooterCenterStyle:       zoneStyle(configmanager.GetPDFFooterCenterFont(), configmanager.GetPDFFooterCenterColor(), configmanager.GetPDFFooterCenterSize(), configmanager.GetPDFFooterCenterBold(), configmanager.GetPDFFooterCenterItalic()),
		FooterRightStyle:        zoneStyle(configmanager.GetPDFFooterRightFont(), configmanager.GetPDFFooterRightColor(), configmanager.GetPDFFooterRightSize(), configmanager.GetPDFFooterRightBold(), configmanager.GetPDFFooterRightItalic()),
		FooterRule:              configmanager.GetPDFFooterRule(),
		FooterSkipFirstPage:     configmanager.GetPDFFooterSkipFirstPage(),
		HeaderLeft:              configmanager.GetPDFHeaderLeft(),
		HeaderCenter:            configmanager.GetPDFHeaderCenter(),
		HeaderRight:             configmanager.GetPDFHeaderRight(),
		HeaderLeftStyle:         zoneStyle(configmanager.GetPDFHeaderLeftFont(), configmanager.GetPDFHeaderLeftColor(), configmanager.GetPDFHeaderLeftSize(), configmanager.GetPDFHeaderLeftBold(), configmanager.GetPDFHeaderLeftItalic()),
		HeaderCenterStyle:       zoneStyle(configmanager.GetPDFHeaderCenterFont(), configmanager.GetPDFHeaderCenterColor(), configmanager.GetPDFHeaderCenterSize(), configmanager.GetPDFHeaderCenterBold(), configmanager.GetPDFHeaderCenterItalic()),
		HeaderRightStyle:        zoneStyle(configmanager.GetPDFHeaderRightFont(), configmanager.GetPDFHeaderRightColor(), configmanager.GetPDFHeaderRightSize(), configmanager.GetPDFHeaderRightBold(), configmanager.GetPDFHeaderRightItalic()),
		HeaderRule:              configmanager.GetPDFHeaderRule(),
		HeaderSkipFirstPage:     configmanager.GetPDFHeaderSkipFirstPage(),
	}
}

// zoneStyle builds a ZoneStyle from the resolved header/footer setting values.
func zoneStyle(font, hexColor string, size int, bold, italic bool) ZoneStyle {
	return ZoneStyle{
		Font:   font,
		Color:  utils.HexToRGB(hexColor),
		Size:   float64(size),
		Bold:   bold,
		Italic: italic,
	}
}

// zoneTokens resolves the values available for pdf header/footer templates.
func zoneTokens(filePath pathutils.MetaPath) map[string]string {
	relPath := filePath.Rel()
	tokens := map[string]string{
		"date":     configmanager.FormatDate(time.Now()),
		"filename": filepath.Base(relPath),
		"filepath": relPath,
		"folder":   files.FolderFromPath(filePath),
	}
	if metadata, err := files.MetaDataGet(filePath); err == nil && metadata != nil {
		tokens["created"] = configmanager.FormatDateTime(metadata.CreatedAt)
		tokens["edited"] = configmanager.FormatDateTime(metadata.LastEdited)
		tokens["tags"] = strings.Join(metadata.Tags, ", ")
		tokens["collection"] = metadata.Collection
	}
	return tokens
}
