// Package server - API utility functions
package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"knov/internal/server/render"
)

func writeResponse(w http.ResponseWriter, r *http.Request, jsonData any, htmlData string) {
	acceptHeader := r.Header.Get("Accept")

	if strings.Contains(acceptHeader, "text/html") || strings.Contains(acceptHeader, "*/*") {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlData))
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonData)
	}
}

// setAttachmentFilename sets a Content-Disposition header for filename,
// properly quoting/escaping it per RFC 6266/2231 (via mime.FormatMediaType)
// instead of splicing it into the header unquoted — a filename containing a
// space or other token-breaking character would otherwise produce a
// malformed header.
func setAttachmentFilename(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
}

// writeAPIError writes a status-coded HTML error response, replacing the
// repeated header/status/write block previously duplicated across the file
// rename/move/delete handlers.
func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	w.Write([]byte(render.RenderStatusMessage(render.StatusError, message)))
}
