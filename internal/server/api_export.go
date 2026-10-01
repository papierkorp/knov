package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"knov/internal/configmanager"
	"knov/internal/export"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/translation"
)

// @Summary Start the pdf export
// @Description Starts rendering every markdown file and book to pdf into a zip archive in the
// @Description background. Returns a polling status fragment - see GET /api/jobs/{id}. Download the
// @Description finished archive via GET /api/exports/pdf.
// @Tags exports
// @Produce json,html
// @Success 200 {object} jobStorage.JobRecord
// @Failure 409 {string} string "an export is already running"
// @Failure 500 {string} string "failed to start export"
// @Router /api/exports/pdf [post]
func handleAPIStartExport(w http.ResponseWriter, r *http.Request) {
	id, err := job.StartExport()
	if errors.Is(err, job.ErrAlreadyRunning) {
		writeAPIError(w, r, http.StatusConflict, translation.SprintfForRequest(configmanager.GetLanguage(), "an export is already running"))
		return
	}
	if err != nil {
		logging.LogError(logging.KeyExport, "failed to start pdf export: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to start export"))
		return
	}

	respondJobStarted(w, r, id, job.JobTypeExport)
}

// @Summary Download all files
// @Description Streams all files of the data folder as a zip archive
// @Tags exports
// @Produce application/zip
// @Success 200 {file} file "zip archive"
// @Router /api/exports/files [get]
func handleAPIExportFiles(w http.ResponseWriter, r *http.Request) {
	streamExport(w, r, export.KindFiles)
}

// @Summary Download all files converted to markdown
// @Description Streams all files of the data folder as a zip archive, dokuwiki files converted to markdown
// @Tags exports
// @Produce application/zip
// @Success 200 {file} file "zip archive"
// @Router /api/exports/markdown [get]
func handleAPIExportMarkdown(w http.ResponseWriter, r *http.Request) {
	streamExport(w, r, export.KindMarkdown)
}

// streamExport streams the zip archive of kind to the client.
func streamExport(w http.ResponseWriter, r *http.Request, kind string) {
	w.Header().Set("Content-Type", "application/zip")
	setAttachmentFilename(w, fmt.Sprintf("knov-export-%s_%s.zip", kind, time.Now().Format("2006-01-02_15-04-05")))
	// the response has already started, so an error can only be logged and leaves a broken zip
	if _, err := export.Write(r.Context(), kind, w, func(int, int) {}); err != nil {
		logging.LogError(logging.KeyExport, "failed to stream %s export: %v", kind, err)
	}
}

// @Summary Download the pdf export
// @Description Downloads the zip archive created by the last POST /api/exports/pdf run
// @Tags exports
// @Produce application/zip
// @Success 200 {file} file "zip archive"
// @Failure 404 {string} string "no pdf export available"
// @Router /api/exports/pdf [get]
func handleAPIDownloadPDFExport(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(export.Path())
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "no export available"))
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	setAttachmentFilename(w, fmt.Sprintf("knov-export-pdf_%s.zip", info.ModTime().Format("2006-01-02_15-04-05")))
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// @Summary Delete the pdf export
// @Description Deletes the pdf export zip archive to free its storage space
// @Tags exports
// @Produce json,html
// @Success 200 {object} map[string]string "deleted export kind"
// @Failure 500 {string} string "failed to delete export"
// @Router /api/exports/pdf [delete]
func handleAPIDeleteExport(w http.ResponseWriter, r *http.Request) {
	if err := export.Remove(); err != nil {
		logging.LogError(logging.KeyExport, "failed to delete pdf export: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete export"))
		return
	}
	writeResponse(w, r, map[string]string{"kind": export.KindPDF}, "")
}
