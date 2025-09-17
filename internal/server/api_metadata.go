package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"knov/internal/files"
)

// @Summary Get metadata for a single file
// @Description Get metadata for a file by providing filepath as query parameter
// @Tags files
// @Produce json,html
// @Param filepath query string true "File path"
// @Success 200 {object} files.Metadata
// @Failure 400 {string} string "missing filepath parameter"
// @Failure 404 {string} string "metadata not found"
// @Failure 500 {string} string "failed to get metadata"
// @Router /api/files/metadata [get]
func handleAPIGetMetadata(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	metadata, err := files.MetaDataGet(filepath)
	if err != nil {
		http.Error(w, "failed to get metadata", http.StatusInternalServerError)
		return
	}

	if metadata == nil {
		http.Error(w, "metadata not found", http.StatusNotFound)
		return
	}

	var html strings.Builder
	html.WriteString("<div class='metadata'>")
	html.WriteString(fmt.Sprintf("<p>Path: %s</p>", metadata.Path))
	html.WriteString(fmt.Sprintf("<p>Project: %s</p>", metadata.Project))
	html.WriteString(fmt.Sprintf("<p>Type: %s</p>", metadata.FileType))
	html.WriteString(fmt.Sprintf("<p>Status: %s</p>", metadata.Status))
	html.WriteString(fmt.Sprintf("<p>Priority: %s</p>", metadata.Priority))
	if len(metadata.Tags) > 0 {
		html.WriteString(fmt.Sprintf("<p>Tags: %s</p>", strings.Join(metadata.Tags, ", ")))
	}
	html.WriteString("</div>")

	writeResponse(w, r, metadata, html.String())
}

// @Summary Set metadata for a single file
// @Description Set metadata for a file using JSON payload
// @Tags files
// @Accept json
// @Produce json,html
// @Param metadata body files.Metadata true "Metadata object"
// @Success 200 {string} string "metadata saved"
// @Failure 400 {string} string "invalid json or missing path"
// @Failure 500 {string} string "failed to save metadata"
// @Router /api/files/metadata [post]
func handleAPISetMetadata(w http.ResponseWriter, r *http.Request) {
	var metadata files.Metadata

	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if metadata.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	err := files.MetaDataSave(&metadata)
	if err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	data := "metadata saved"
	html := `<span class="status-ok">Metadata saved successfully</span>`
	writeResponse(w, r, data, html)
}

// @Summary Initialize/Rebuild metadata for all files
// @Description Creates metadata for all files that don't have metadata yet
// @Tags files
// @Produce json,html
// @Success 200 {string} string "metadata initialized"
// @Failure 500 {string} string "failed to initialize metadata"
// @Router /api/files/metadata/rebuild [post]
func handleAPIRebuildMetadata(w http.ResponseWriter, r *http.Request) {
	if err := files.MetaDataInitializeAll(); err != nil {
		http.Error(w, "failed to initialize metadata", http.StatusInternalServerError)
		return
	}

	err := files.MetaDataLinksRebuild()
	if err != nil {
		http.Error(w, "failed to rebuild metadata links", http.StatusInternalServerError)
		return
	}

	data := map[string]string{"status": "metadata initialized"}
	html := `<span class="status-ok">Metadata initialized and rebuilt successfully</span>`
	writeResponse(w, r, data, html)
}
