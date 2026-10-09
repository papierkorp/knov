// Package server ..
package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pdfexport"
	"knov/internal/translation"
)

// @Summary Export file to pdf
// @Description Renders a file's markdown source (or one section of it, or a `.book` file's composed document) to a downloadable pdf
// @Tags files
// @Produce application/pdf
// @Param filepath query string true "File path"
// @Param section query string false "Section ID (optional, exports only this section)"
// @Success 200 {file} file "pdf file"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "export failed"
// @Router /api/files/export/pdf [get]
func handleAPIExportToPDF(w http.ResponseWriter, r *http.Request) {
	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	sectionID := r.URL.Query().Get("section")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	var content []byte
	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	// a book exports as its composed document, so a section only applies to other files
	if sectionID != "" && !files.IsBook(filePath) {
		logging.LogDebug(logging.KeyPdfExport, "pdf export requested: %s section %s", filePath, sectionID)

		handler := contentHandler.GetHandler("markdown")
		sectionContent, err := handler.ExtractSection(filePath, sectionID, configmanager.GetSectionEditIncludeSubheaders())
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: failed to extract section %s in file %s: %v", sectionID, filePath, err)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
			return
		}
		content = []byte(sectionContent)
		filename += "-" + sectionID
	} else {
		logging.LogDebug(logging.KeyPdfExport, "pdf export requested: %s", filePath)

		source, err := pdfexport.LoadSource(filePath)
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "pdf export: failed to load file %s: %v", filePath, err)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
			return
		}
		content = source
	}

	pdf, err := pdfexport.RenderFile(filePath, content)
	if err != nil {
		logging.LogError(logging.KeyPdfExport, "pdf export: failed to convert file to pdf %s: %v", filePath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to convert file to pdf"))
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	setAttachmentFilename(w, filename+".pdf")
	w.Write(pdf)

	logging.LogInfo(logging.KeyPdfExport, "exported file to pdf: %s (%d bytes)", filePath, len(pdf))
}
