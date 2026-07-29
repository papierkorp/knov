// Package server ..
package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/pdfexport"
	"knov/internal/translation"
	"knov/internal/utils"
)

// @Summary Export file to pdf
// @Description Renders a file's markdown source, or optionally just one section of it, to a downloadable pdf
// @Tags files
// @Produce application/pdf
// @Param filepath query string true "File path"
// @Param section query string false "Section ID (optional, exports only this section)"
// @Success 200 {file} file "pdf file"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/pdf [get]
func handleAPIExportToPDF(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	sectionID := r.URL.Query().Get("section")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	var content []byte
	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	if sectionID != "" {
		logging.LogDebug(logging.KeyPdfExport, "pdf export requested: %s section %s", filePath, sectionID)

		handler := contentHandler.GetHandler("markdown")
		sectionContent, err := handler.ExtractSection(filePath, sectionID, configmanager.GetSectionEditIncludeSubheaders())
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: failed to extract section %s in file %s: %v", sectionID, filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
			return
		}
		content = []byte(sectionContent)
		filename += "-" + sectionID
	} else {
		fullPath := pathutils.ToDocsPath(filePath)
		logging.LogDebug(logging.KeyPdfExport, "pdf export requested: %s (resolved: %s)", filePath, fullPath)

		fileContent, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: failed to read file %s: %v", fullPath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
			return
		}
		content = fileContent
	}

	pdf, err := renderPDFSafely(filePath, content)
	if err != nil {
		logging.LogError(logging.KeyPdfExport, "pdf export: failed to convert file to pdf %s: %v", filePath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to convert file to pdf"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	setAttachmentFilename(w, filename+".pdf")
	w.Write(pdf)

	logging.LogInfo(logging.KeyPdfExport, "exported file to pdf: %s (%d bytes)", filePath, len(pdf))
}

// renderPDFSafely calls pdfexport.MarkdownToPDF and turns any panic into an
// error so it lands in the app log instead of only the recoverer's stderr
// output, which is easy to miss when the binary runs as a background service.
func renderPDFSafely(filePath string, content []byte) (pdf []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: panic during conversion: %v", r)
			err = fmt.Errorf("panic during pdf conversion: %v", r)
		}
	}()
	opts := pdfexport.Options{
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
		HeaderLeft:              configmanager.GetPDFHeaderLeft(),
		HeaderCenter:            configmanager.GetPDFHeaderCenter(),
		HeaderRight:             configmanager.GetPDFHeaderRight(),
		HeaderLeftStyle:         zoneStyle(configmanager.GetPDFHeaderLeftFont(), configmanager.GetPDFHeaderLeftColor(), configmanager.GetPDFHeaderLeftSize(), configmanager.GetPDFHeaderLeftBold(), configmanager.GetPDFHeaderLeftItalic()),
		HeaderCenterStyle:       zoneStyle(configmanager.GetPDFHeaderCenterFont(), configmanager.GetPDFHeaderCenterColor(), configmanager.GetPDFHeaderCenterSize(), configmanager.GetPDFHeaderCenterBold(), configmanager.GetPDFHeaderCenterItalic()),
		HeaderRightStyle:        zoneStyle(configmanager.GetPDFHeaderRightFont(), configmanager.GetPDFHeaderRightColor(), configmanager.GetPDFHeaderRightSize(), configmanager.GetPDFHeaderRightBold(), configmanager.GetPDFHeaderRightItalic()),
		HeaderRule:              configmanager.GetPDFHeaderRule(),
	}
	if opts.FooterLeft != "" || opts.FooterCenter != "" || opts.FooterRight != "" {
		opts.FooterTokens = zoneTokens(filePath)
	}
	if opts.HeaderLeft != "" || opts.HeaderCenter != "" || opts.HeaderRight != "" {
		opts.HeaderTokens = zoneTokens(filePath)
	}
	return pdfexport.MarkdownToPDF(content, opts)
}

// zoneStyle builds a pdfexport.ZoneStyle from the resolved header/footer setting values.
func zoneStyle(font, hexColor string, size int, bold, italic bool) pdfexport.ZoneStyle {
	return pdfexport.ZoneStyle{
		Font:   font,
		Color:  utils.HexToRGB(hexColor),
		Size:   float64(size),
		Bold:   bold,
		Italic: italic,
	}
}

// zoneTokens resolves the values available for pdf header/footer templates.
func zoneTokens(filePath string) map[string]string {
	relPath := pathutils.ToRelative(filePath)
	tokens := map[string]string{
		"date":     configmanager.FormatDate(time.Now()),
		"filename": filepath.Base(relPath),
		"filepath": relPath,
		"folder":   files.FolderFromPath(filePath),
	}
	if metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath)); err == nil && metadata != nil {
		tokens["created"] = configmanager.FormatDateTime(metadata.CreatedAt)
		tokens["edited"] = configmanager.FormatDateTime(metadata.LastEdited)
		tokens["tags"] = strings.Join(metadata.Tags, ", ")
		tokens["collection"] = metadata.Collection
	}
	return tokens
}
