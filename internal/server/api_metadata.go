// Package server ..
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/job"
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// ----------------------------------------------------------------------------------------
// ----------------------------------- BULK OPERATIONS -----------------------------------
// ----------------------------------------------------------------------------------------

type bulkUpdatePatch struct {
	Editor     *files.EditorType `json:"editor,omitempty"`
	TagsAdd    []string          `json:"tagsAdd,omitempty"`
	TagsRemove []string          `json:"tagsRemove,omitempty"`
}

type bulkUpdateResult struct {
	Updated []string `json:"updated"`
	Count   int      `json:"count"`
	Preview bool     `json:"preview"`
}

// @Summary Bulk update metadata for files matching a filter
// @Description Applies a metadata patch to all files that match the given filter criteria. Pass preview=true to see which files would be affected without applying changes. Supported actions: set-editor, add-tag, remove-tag.
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filterField formData string true "Metadata field to filter on"
// @Param filterOp formData string true "Filter operator (equals, contains, regex)"
// @Param filterValue formData string true "Filter value"
// @Param action formData string true "Patch action (set-editor, add-tag, remove-tag)"
// @Param patchValue formData string true "Patch value"
// @Param preview formData bool false "Preview only, do not apply"
// @Success 200 {object} bulkUpdateResult
// @Failure 400 {string} string "invalid form data or patch"
// @Failure 500 {string} string "internal error"
// @Router /api/metadata/bulk-update [post]
func handleAPIBulkUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}

	patchValue := r.FormValue("patchValue")
	if patchValue == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no patch fields provided"))
		return
	}

	var p bulkUpdatePatch
	switch r.FormValue("action") {
	case "set-editor":
		editor := files.EditorType(patchValue)
		if !slices.Contains(files.AllEditorTypes(), editor) {
			writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid editor type"))
			return
		}
		p.Editor = &editor
	case "add-tag":
		p.TagsAdd = []string{patchValue}
	case "remove-tag":
		p.TagsRemove = []string{patchValue}
	default:
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no patch fields provided"))
		return
	}

	criteria := []filter.Criteria{{
		Metadata: r.FormValue("filterField"),
		Operator: r.FormValue("filterOp"),
		Value:    r.FormValue("filterValue"),
		Action:   "include",
	}}
	matched, err := filter.FilterFiles(criteria, "and")
	if err != nil {
		logging.LogError(logging.KeyApp, "bulk-update: filter failed: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to filter files"))
		return
	}

	paths := make([]string, 0, len(matched))
	for _, f := range matched {
		paths = append(paths, f.Metadata.Path)
	}

	if r.FormValue("preview") == "true" {
		writeResponse(w, r, bulkUpdateResult{Updated: paths, Count: len(paths), Preview: true},
			render.RenderBulkUpdateResult(paths, len(paths), true))
		return
	}

	result, err := job.RunBulkUpdateMetadata(matched, files.BulkUpdatePatch{
		Editor:     p.Editor,
		TagsAdd:    p.TagsAdd,
		TagsRemove: p.TagsRemove,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}
	if result.Failed > 0 {
		logging.LogWarning(logging.KeyApp, "bulk-update: %d/%d files failed to save", result.Failed, len(matched))
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "%d files updated", result.Updated))
	writeResponse(w, r, bulkUpdateResult{Updated: paths, Count: result.Updated, Preview: false},
		render.RenderBulkUpdateResult(paths, result.Updated, false))
}

// @Summary Get metadata for a single file
// @Description Get metadata for a file using filepath query parameter. Supports both media/ and docs/ paths.
// @Tags metadata
// @Produce json,html
// @Param filepath query string true "File path (with or without media/docs prefix)"
// @Success 200 {object} files.Metadata
// @Failure 400 {string} string "missing filepath parameter"
// @Failure 404 {string} string "metadata not found"
// @Failure 500 {string} string "failed to get metadata"
// @Router /api/metadata [get]
func handleAPIGetMetadata(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	metadata, err := files.MetaDataGet(normalizedPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get metadata for %s: %v", normalizedPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}

	if metadata == nil {
		if strings.HasPrefix(normalizedPath, "media/") {
			metadata = &files.Metadata{Path: normalizedPath}
		} else {
			writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
			return
		}
	}

	var html string
	if strings.HasPrefix(normalizedPath, "media/") {
		html = render.RenderMediaDetail(metadata)
	} else {
		html = render.RenderFileMetadataSimple(metadata)
	}
	writeResponse(w, r, metadata, html)
}

// @Summary Set metadata for a single file
// @Description Set metadata for a file using JSON payload
// @Tags metadata
// @Accept json
// @Produce json,html
// @Param metadata body files.Metadata true "Metadata object"
// @Success 200 {string} string "metadata saved"
// @Failure 400 {string} string "invalid json or missing path"
// @Failure 500 {string} string "failed to save metadata"
// @Router /api/metadata [post]
func handleAPISetMetadata(w http.ResponseWriter, r *http.Request) {
	var metadata files.Metadata

	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid json"))
		return
	}

	if metadata.Path == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "path is required"))
		return
	}

	// SetMetadataNoRefresh applies every provided field plus the derived-field resync under a
	// single lock acquisition for path, so the request is atomic against other writers again.
	path := pathutils.ToWithPrefix(metadata.Path)
	if err := files.SetMetadataNoRefresh(path, &metadata); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}
	files.RefreshCaches()

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata saved"))
	writeResponse(w, r, "metadata saved", "")
}

// @Summary Initialize/Rebuild metadata for all files
// @Description Starts a full metadata rebuild (init all + purge stale/duplicates + links +
// @Description orphaned media cache) in the background; returns a polling status fragment - see
// @Description GET /api/jobs/{id}.
// @Tags metadata
// @Produce json,html
// @Success 200 {object} jobStorage.JobRecord
// @Failure 409 {string} string "rebuild already running"
// @Failure 500 {string} string "failed to start rebuild"
// @Router /api/metadata/rebuild [post]
func handleAPIRebuildMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := job.StartFullRebuild()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}

	respondJobStarted(w, r, id, job.JobTypeFullRebuild)
}

// @Summary Rebuild metadata links for a single file
// @Description Rebuilds metadata links (ancestors, kids, usedLinks, linksToHere) for one file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath path string true "File path"
// @Success 200 {string} string "metadata links rebuilt"
// @Failure 400 {string} string "missing filepath"
// @Failure 500 {string} string "failed to rebuild metadata links"
// @Router /api/metadata/rebuild/{filepath} [post]
func handleAPIRebuildFileMetadata(w http.ResponseWriter, r *http.Request) {
	// r.URL.Path, not chi.URLParam(r, "*") - that is still percent-encoded when the request
	// has a RawPath (see handleAPIDeleteMedia)
	filePath := strings.TrimPrefix(r.URL.Path, "/api/metadata/rebuild/")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath"))
		return
	}

	if err := files.MetaDataLinksRebuildForFile(filePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to rebuild metadata links for %s: %v", filePath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to rebuild metadata links"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata links rebuilt"))
	writeResponse(w, r, map[string]string{"status": "metadata links rebuilt"}, "")
}

// @Summary Export all metadata
// @Description Export all metadata as JSON or CSV file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce application/json,text/csv
// @Param format formData string false "Export format (json or csv)" default(json)
// @Success 200 {file} file "exported metadata file"
// @Failure 500 {string} string "failed to export metadata"
// @Router /api/metadata/export [post]
func handleAPIExportMetadata(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	format := r.FormValue("format")
	if format == "" {
		format = "json"
	}

	allMetadata, err := files.MetaDataExportAll()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export metadata"))
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=metadata_export.csv")
		csvData := render.RenderMetadataCSV(allMetadata)
		w.Write([]byte(csvData))
	case "json":
		fallthrough
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=metadata_export.json")
		if err := json.NewEncoder(w).Encode(allMetadata); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to encode json"))
			return
		}
	}
}

// @Summary Scan for broken links
// @Description Scans link metadata (no file content is read) for outbound links pointing to files that no longer exist, suggesting a repair target where the broken link's filename uniquely matches an existing file.
// @Tags metadata
// @Produce json,html
// @Success 200 {array} files.BrokenLink
// @Failure 500 {string} string "failed to scan for broken links"
// @Router /api/metadata/broken-links [get]
func handleAPIScanBrokenLinks(w http.ResponseWriter, r *http.Request) {
	broken, err := files.FindBrokenLinks()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to scan for broken links: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to scan for broken links"))
		return
	}

	html := render.RenderBrokenLinksHTML(broken)
	writeResponse(w, r, broken, html)
}

// @Summary Repair selected broken links
// @Description Applies the selected repairs from a broken-links scan, rewriting each link to its suggested target
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param repair formData []string false "Repair entries as sourceFile|target|suggested, repeatable"
// @Success 200 {string} string "broken links repaired"
// @Failure 400 {string} string "failed to parse form"
// @Router /api/metadata/broken-links/repair [post]
func handleAPIRepairBrokenLinks(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	entries := r.Form["repair"]
	logging.LogInfo(logging.KeyRepairLinks, "broken links repair started: %d requested", len(entries))

	result, err := job.RunRepairBrokenLinks(entries)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}
	repaired, skipped := result.Repaired, result.Skipped

	logging.LogInfo(logging.KeyRepairLinks, "broken links repair completed: %d repaired, %d skipped", repaired, skipped)

	broken, _ := files.FindBrokenLinks()
	html := render.RenderBrokenLinksHTML(broken)
	if skipped > 0 {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "%d links repaired, %d could not be matched in their file", repaired, skipped))
	} else {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "%d links repaired", repaired))
	}
	writeResponse(w, r, map[string]int{"repaired": repaired, "skipped": skipped}, html)
}

// @Summary Scan for bare links read from their doc's folder
// @Description Lists every bare markdown or html link (no "./", "../", leading "/" or "media/") whose target changed when bare links started to be read from the doc's folder instead of the docs root, with its old and new target. Changes nothing.
// @Tags metadata
// @Produce json,html
// @Success 200 {array} files.RelativeLinkChange
// @Failure 500 {string} string "failed to scan for relative links"
// @Router /api/metadata/relative-links [get]
func handleAPIScanRelativeLinks(w http.ResponseWriter, r *http.Request) {
	changes, err := files.ScanRelativeLinks()
	if err != nil {
		logging.LogError(logging.KeyRepairLinks, "failed to scan for relative links: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to scan for relative links"))
		return
	}
	writeResponse(w, r, changes, render.RenderRelativeLinksHTML(changes))
}

// @Summary Keep the old targets of selected bare links
// @Description Rewrites the selected bare links of a relative links scan to their docs-root form ("/a.md", "/media/x.png"), so they keep pointing at their old target
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param migrate formData []string false "Entries as a json array [sourceFile, oldTarget], repeatable"
// @Success 200 {string} string "links migrated"
// @Failure 400 {string} string "invalid migrate entry"
// @Failure 409 {string} string "already running"
// @Router /api/metadata/relative-links/migrate [post]
func handleAPIMigrateRelativeLinks(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}
	var changes []files.RelativeLinkChange
	for _, entry := range r.Form["migrate"] {
		var pair [2]string
		if err := json.Unmarshal([]byte(entry), &pair); err != nil || pair[0] == "" || pair[1] == "" {
			writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid migrate entry"))
			return
		}
		changes = append(changes, files.RelativeLinkChange{SourceFile: pair[0], OldTarget: pair[1]})
	}

	result, err := job.RunMigrateRelativeLinks(changes)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, err.Error())
		return
	}

	remaining, _ := files.ScanRelativeLinks()
	if result.Skipped > 0 {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "links migrated in %d files, %d could not be matched in their file", result.Migrated, result.Skipped))
	} else {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "links migrated in %d files", result.Migrated))
	}
	writeResponse(w, r, map[string]int{"migrated": result.Migrated, "skipped": result.Skipped}, render.RenderRelativeLinksHTML(remaining))
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- GET INDIVIDUAL ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get file collection
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/collection [get]
func handleAPIGetMetadataCollection(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderMetadataLinkHTML(metadata.Collection, "collection")
	writeResponse(w, r, metadata.Collection, html)
}

// @Summary Get editor type for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/editor [get]
func handleAPIGetMetadataEditor(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderMetadataLinkHTML(string(metadata.Editor), "editor")
	writeResponse(w, r, string(metadata.Editor), html)
}

// @Summary Get file path
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/path [get]
func handleAPIGetMetadataPath(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := fmt.Sprintf(`<span class="path">%s</span>`, metadata.Path)
	writeResponse(w, r, metadata.Path, html)
}

// @Summary Get file creation date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/createdat [get]
func handleAPIGetMetadataCreatedAt(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	createdAt := configmanager.FormatDateTime(metadata.CreatedAt)
	html := fmt.Sprintf(`<span class="createdat">%s</span>`, createdAt)
	writeResponse(w, r, createdAt, html)
}

// @Summary Get file last edited date
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/lastedited [get]
func handleAPIGetMetadataLastEdited(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	lastEdited := configmanager.FormatDateTime(metadata.LastEdited)
	html := fmt.Sprintf(`<span class="lastedited">%s</span>`, lastEdited)
	writeResponse(w, r, lastEdited, html)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- SET INDIVIDUAL ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Set editor type for a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param editor formData string true "Editor type"
// @Success 200 {string} string
// @Router /api/metadata/editor [post]
func handleAPISetMetadataEditor(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	editor := r.FormValue("editor")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	if err := files.SetEditor(pathutils.ToWithPrefix(filePath), files.EditorType(editor)); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor updated"))
	writeResponse(w, r, "editor updated", "")
}

// @Summary Set file path
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "Current file path"
// @Param newpath formData string true "New file path"
// @Success 200 {string} string
// @Router /api/metadata/path [post]
func handleAPISetMetadataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	newpath := r.FormValue("newpath")

	if filePath == "" || newpath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or newpath parameter"))
		return
	}

	newpath = filepath.Clean(newpath)

	if filePath == newpath {
		writeResponse(w, r, newpath, "")
		return
	}

	logging.LogInfo(logging.KeyApp, "changing file path via metadata: %s -> %s", filePath, newpath)

	isMedia := strings.HasPrefix(filePath, "media/")
	context := "move file via metadata"
	var err error
	if isMedia {
		context = "move media via metadata"
		err = files.MoveMediaFileNoRefresh(pathutils.ToRelative(filePath), pathutils.ToRelative(newpath))
	} else {
		err = files.MoveFileNoRefresh(logging.KeyApp, pathutils.ToRelative(filePath), pathutils.ToRelative(newpath))
	}
	msgs := moveErrorMessages{
		sourceMissing: translation.SprintfForRequest(configmanager.GetLanguage(), "current file does not exist"),
		targetExists:  translation.SprintfForRequest(configmanager.GetLanguage(), "file with new path already exists"),
		moveFailed:    translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move file"),
	}
	if handleMoveError(err, context, filePath, newpath, msgs, func(status int, message string) {
		writeAPIError(w, r, status, message)
	}) {
		return
	}
	files.RefreshCaches()

	logging.LogInfo(logging.KeyApp, "successfully moved file via metadata: %s -> %s", filePath, newpath)
	newRelPath := pathutils.ToRelative(newpath)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file moved successfully"))
	w.Header().Set("HX-Redirect", pathutils.ToFileURL(newRelPath))
	w.WriteHeader(http.StatusOK)
}

// @Summary Set file creation date
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param createdat formData string true "Creation date (YYYY-MM-DD HH:MM:SS)"
// @Success 200 {string} string
// @Router /api/metadata/createdat [post]
func handleAPISetMetadataCreatedAt(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	createdAtStr := r.FormValue("createdat")

	if filePath == "" || createdAtStr == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or createdat parameter"))
		return
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid date format"))
		return
	}

	if err := files.SetCreatedAt(pathutils.ToWithPrefix(filePath), createdAt); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "created at updated"))
	writeResponse(w, r, "createdat updated", "")
}

// @Summary Set file last edited date
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param lastedited formData string true "Last edited date (YYYY-MM-DD HH:MM:SS)"
// @Success 200 {string} string
// @Router /api/metadata/lastedited [post]
func handleAPISetMetadataLastEdited(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	lastEditedStr := r.FormValue("lastedited")

	if filePath == "" || lastEditedStr == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or lastedited parameter"))
		return
	}

	lastEdited, err := time.Parse("2006-01-02 15:04:05", lastEditedStr)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid date format"))
		return
	}

	if err := files.SetLastEdited(pathutils.ToWithPrefix(filePath), lastEdited); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "last edited updated"))
	writeResponse(w, r, "lastedited updated", "")
}

// @Summary Set file tags
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param tags formData string true "Comma-separated tag list"
// @Success 200 {string} string
// @Router /api/metadata/tags [post]
func handleAPISetMetadataTags(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	tagsStr := r.FormValue("tags")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		var filteredTags []string
		for _, tag := range tags {
			if tag != "" {
				filteredTags = append(filteredTags, tag)
			}
		}
		tags = filteredTags
	} else {
		tags = []string{}
	}

	oldTags, newTags, err := files.SetTagsStrict(pathutils.ToWithPrefix(filePath), tags)
	if errors.Is(err, files.ErrInvalidKanbanTags) {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	if msg := kanban.TagNotifyMsg(kanban.TagFromList(oldTags), kanban.TagFromList(newTags)); msg != "" {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), msg))
	} else {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "tags updated"))
	}
	writeResponse(w, r, "tags updated", "")
}

// @Summary Set file parents
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param parents formData string true "Comma-separated parent file paths"
// @Success 200 {string} string
// @Router /api/metadata/parents [post]
func handleAPISetMetadataParents(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filePath := r.FormValue("filepath")
	parentsStr := r.FormValue("parents")

	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	var parents []string
	if parentsStr != "" {
		parents = strings.Split(parentsStr, ",")
		for i := range parents {
			parents[i] = strings.TrimSpace(parents[i])
		}
		for _, parent := range parents {
			if parent == "" {
				continue
			}
			fullParentPath := pathutils.ToFullPath(parent)
			if _, err := os.Stat(fullParentPath); os.IsNotExist(err) {
				writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "parent file does not exist: %s", parent))
				return
			}
		}
	} else {
		parents = []string{}
	}

	if err := files.SetParents(pathutils.ToWithPrefix(filePath), parents); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "parents updated"))
	writeResponse(w, r, "parents updated", "")
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- GET ALL ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get all tags or tags for a specific file
// @Description Get all tags with counts, or tags for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns tags for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.TagCount
// @Router /api/metadata/tags [get]
func handleAPIGetAllTags(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath != "" {
		handleAPIGetFileMetadataTags(w, r)
		return
	}

	// options feed the edit/filter inputs, so only unscoped hide entries apply there
	format := r.URL.Query().Get("format")
	scope := configmanager.HideScopeBrowse
	if format == "options" {
		scope = ""
	}

	tags, err := files.GetAllTags(scope)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get tags"))
		return
	}

	if format == "options" {
		names := slices.Sorted(maps.Keys(tags))
		var html strings.Builder
		for _, name := range names {
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, name, name)
		}
		writeResponse(w, r, names, html.String())
		return
	}

	html := render.RenderBrowseHTML(tags, "/browse/tag", r.URL.Query().Get("actions") == "true", "tag")
	writeResponse(w, r, tags, html)
}

// @Summary Get all collections or collection for a specific file
// @Description Get all collections with counts, or collection for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns collection for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.CollectionCount
// @Router /api/metadata/collections [get]
func handleAPIGetAllCollections(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath != "" {
		handleAPIGetFileMetadataCollection(w, r)
		return
	}

	// options feed the edit/filter inputs, so only unscoped hide entries apply there
	format := r.URL.Query().Get("format")
	scope := configmanager.HideScopeBrowse
	if format == "options" {
		scope = ""
	}

	collections, err := files.GetAllCollections(scope)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get collections"))
		return
	}

	if format == "options" {
		names := slices.Sorted(maps.Keys(collections))
		var html strings.Builder
		for _, name := range names {
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, name, name)
		}
		writeResponse(w, r, names, html.String())
		return
	}

	html := render.RenderBrowseHTML(collections, "/browse/collection", r.URL.Query().Get("actions") == "true", "collection")
	writeResponse(w, r, collections, html)
}

// @Summary Get all folders or folders for a specific file
// @Description Get all folders with counts, or folders for a specific file if filepath is provided
// @Tags metadata
// @Param filepath query string false "File path (optional - if provided, returns folders for that specific file)"
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Success 200 {object} files.FolderCount
// @Router /api/metadata/folders [get]
func handleAPIGetAllFolders(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath != "" {
		handleAPIGetFileMetadataFolders(w, r)
		return
	}

	// options feed the edit/filter inputs, so only unscoped hide entries apply there
	format := r.URL.Query().Get("format")
	scope := configmanager.HideScopeBrowse
	if format == "options" {
		scope = ""
	}

	folders, err := files.GetAllFolders(scope)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get folders"))
		return
	}

	if format == "options" {
		names := slices.Sorted(maps.Keys(folders))
		var html strings.Builder
		for _, name := range names {
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, name, name)
		}
		writeResponse(w, r, names, html.String())
		return
	}

	html := render.RenderBrowseHTML(folders, "/browse/folder", r.URL.Query().Get("actions") == "true", "folder")
	writeResponse(w, r, folders, html)
}

// @Summary Get all file titles
// @Description Returns all non-empty titles extracted from file content, as options for datalist
// @Tags metadata
// @Param format query string false "Response format (options for HTML datalist options)"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/titles [get]
func handleAPIGetAllTitles(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		cachedTitles, err := files.GetAllTitlesFromCache()
		if err != nil || len(cachedTitles) == 0 {
			if err != nil {
				logging.LogError(logging.KeyApp, "failed to get cached titles, fallback to live data: %v", err)
			}
			cachedTitles, err = files.GetAllTitles()
			if err != nil {
				writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get titles"))
				return
			}
		}
		var html strings.Builder
		for _, title := range cachedTitles {
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, title, title)
		}
		writeResponse(w, r, cachedTitles, html.String())
		return
	}

	titles, err := files.GetAllTitlesFromCache()
	if err != nil || len(titles) == 0 {
		titles, err = files.GetAllTitles()
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get titles"))
			return
		}
	}
	writeResponse(w, r, titles, "")
}

// @Summary Get all available editor types
// @Tags metadata
// @Param format query string false "Response format: options for HTML select options"
// @Param context query string false "Context: chat excludes filter-editor from suggestions"
// @Produce json,html
// @Success 200 {object} files.EditorTypeCount
// @Router /api/metadata/editors [get]
func handleAPIGetAllEditors(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		context := r.URL.Query().Get("context")
		var html strings.Builder
		var types []files.EditorType
		for _, ft := range files.AllEditorTypes() {
			if context == "chat" && ft == files.EditorTypeFilter {
				continue
			}
			types = append(types, ft)
			fmt.Fprintf(&html, `<option value="%s">%s</option>`, ft, ft)
		}
		writeResponse(w, r, types, html.String())
		return
	}

	filetypes, err := files.GetAllEditors(configmanager.HideScopeBrowse)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get editor types"))
		return
	}
	html := render.RenderBrowseHTML(filetypes, "/browse/editor", false, "")
	writeResponse(w, r, filetypes, html)
}

// @Summary Get tags for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/file/tags [get]
func handleAPIGetFileMetadataTags(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderMetadataLinksHTML(metadata.Tags, "tags")
	writeResponse(w, r, metadata.Tags, html)
}

// @Summary Get folders for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} string
// @Router /api/metadata/file/folders [get]
func handleAPIGetFileMetadataFolders(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderMetadataLinksHTML(metadata.Folders, "folders")
	writeResponse(w, r, metadata.Folders, html)
}

// @Summary Get collection for a specific file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {string} string
// @Router /api/metadata/file/collection [get]
func handleAPIGetFileMetadataCollection(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get metadata"))
		return
	}
	if metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderMetadataLinkHTML(metadata.Collection, "collection")
	writeResponse(w, r, metadata.Collection, html)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- REFERENCES ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get references for a file
// @Tags metadata
// @Param filepath query string true "File path"
// @Produce json,html
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [get]
func handleAPIGetMetadataReferences(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"))
		return
	}

	metadata, err := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	if err != nil || metadata == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}

	html := render.RenderReferencesHTML(filePath, metadata.References)
	if r.URL.Query().Get("sidebar") == "true" {
		html = render.RenderReferencesSidebarHTML(metadata.References)
	}
	writeResponse(w, r, metadata.References, html)
}

// @Summary Add a reference to a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param url formData string true "Reference URL"
// @Param description formData string false "Why this link was added"
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [post]
func handleAPIAddMetadataReference(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	refURL := r.FormValue("url")
	description := r.FormValue("description")

	if filePath == "" || refURL == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath and url are required"))
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	notFound := false
	var references []files.Reference
	err := files.MetaDataMutate(normalizedPath, func(m *files.Metadata, existed bool) (bool, error) {
		if !existed {
			notFound = true
			return false, nil
		}
		m.References = append(m.References, files.Reference{
			URL:         refURL,
			Description: description,
			AddedAt:     time.Now(),
		})
		references = m.References
		return true, nil
	})
	if notFound {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to save references for %s: %v", normalizedPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	html := render.RenderReferencesHTML(filePath, references)
	writeResponse(w, r, references, html)
}

// @Summary Delete a reference from a file
// @Tags metadata
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param url formData string true "Reference URL to remove"
// @Success 200 {array} files.Reference
// @Router /api/metadata/references [delete]
func handleAPIDeleteMetadataReference(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath := r.FormValue("filepath")
	refURL := r.FormValue("url")

	if filePath == "" || refURL == "" {
		logging.LogWarning(logging.KeyApp, "delete reference: missing filepath or url in request")
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath and url are required"))
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	notFound := false
	var references []files.Reference
	err := files.MetaDataMutate(normalizedPath, func(m *files.Metadata, existed bool) (bool, error) {
		if !existed {
			notFound = true
			return false, nil
		}
		filtered := m.References[:0]
		for _, ref := range m.References {
			if ref.URL != refURL {
				filtered = append(filtered, ref)
			}
		}
		m.References = filtered
		references = m.References
		return true, nil
	})
	if notFound {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "metadata not found"))
		return
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to save references for %s: %v", normalizedPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save metadata"))
		return
	}

	html := render.RenderReferencesHTML(filePath, references)
	writeResponse(w, r, references, html)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- HELPERS ----------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get inline display for a sidebar metadata field
// @Tags metadata
// @Param field query string true "Field name (tags, parents, editor, path)"
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/metadata/inline-display [get]
func handleAPIMetadataInlineDisplay(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	filePath := r.URL.Query().Get("filepath")
	if field == "" || filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing parameters"))
		return
	}
	metadata, _ := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	html := render.RenderSidebarFieldDisplay(field, filePath, metadata)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// @Summary Get inline editor for a sidebar metadata field
// @Tags metadata
// @Param field query string true "Field name (tags, parents, editor, path)"
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/metadata/inline-edit [get]
func handleAPIMetadataInlineEdit(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	filePath := r.URL.Query().Get("filepath")
	if field == "" || filePath == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing parameters"))
		return
	}
	metadata, _ := files.MetaDataGet(pathutils.ToWithPrefix(filePath))
	html := render.RenderSidebarFieldEdit(field, filePath, metadata)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}
