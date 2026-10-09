package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

func handleFileContent(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/files/")
	filePath := pathutils.DocsPath(rel).String()
	fullPath := pathutils.ToDocsPath(filePath)
	ext := strings.ToLower(filepath.Ext(fullPath))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, fullPath)
		return
	}

	fileContent, err := files.GetFileContent(fullPath)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"))
		return
	}

	// a book renders as its composed document (see files.GetFileContent); prepend the
	// export toolbar, or an empty-book notice, above it.
	if files.IsBook(filePath) {
		fileContent.HTML = render.RenderBookViewPrefix(filePath) + fileContent.HTML
	}

	// show the requested view (?view=) of the file, e.g. a tracker's live stats or the raw source
	fileContent.HTML = render.RenderFileView(rel, r.URL.Query().Get("view"), fileContent.HTML)

	if r.URL.Query().Get("snippet") == "true" || r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fileContent.HTML))
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData(filepath.Base(rel), rel, fileContent)

	err = tm.Render(w, "fileview", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edit/")
	sectionID := r.URL.Query().Get("section")
	editorType := r.URL.Query().Get("editor")

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileEditTemplateData(filePath, sectionID, editorType)

	err := tm.Render(w, "fileedit", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

// ----------------------------------------------------------------------------------------
// -------------------------------- Filetype-specific handlers ---------------------------
// ----------------------------------------------------------------------------------------

func handleFileNewList(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("list-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewTodo(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("todo-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewFilter(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("filter-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewTracker(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("tracker-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewIndex(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("index-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewBook(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("book-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileNewCodeMirror(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("codemirror-editor")
	data.Data.PrefillPath = r.URL.Query().Get("prefillpath")
	if err := tm.Render(w, "filenew", data); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
	}
}

func handleFileEditTable(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edittable/")

	tableIndex := 0
	if idxStr := r.URL.Query().Get("tableindex"); idxStr != "" {
		if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 {
			tableIndex = idx
		}
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileEditTableTemplateData(filePath, tableIndex)

	err := tm.Render(w, "filedittable", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	if strings.HasPrefix(r.URL.Path, "/files/history/") {
		rel := strings.TrimPrefix(r.URL.Path, "/files/history/")

		if rel == "" {
			writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"))
			return
		}

		filePath := pathutils.DocsPath(rel).String()
		fullPath := pathutils.ToFullPath(filePath)
		selectedCommit := r.URL.Query().Get("commit")

		versions, err := git.GetFileHistory(fullPath)
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to get file history for %s: %v", filePath, err)
			writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file history"))
			return
		}

		currentCommit := ""
		if len(versions) > 0 {
			currentCommit = versions[0].Commit
		}

		// resolve "previous" to the actual second commit hash
		if selectedCommit == "previous" && len(versions) > 1 {
			selectedCommit = versions[1].Commit
		} else if selectedCommit == "previous" {
			selectedCommit = currentCommit
		}

		// GetFileHistory returns short (7-char) hashes; a caller may pass a full
		// hash for the same commit (e.g. a direct link), which would otherwise
		// never string-equal currentCommit and wrongly look like a non-current
		// version. Normalize to the same short form git already resolves fine.
		if len(selectedCommit) > 7 {
			selectedCommit = selectedCommit[:7]
		}

		data := thememanager.NewHistoryTemplateData(rel, currentCommit, selectedCommit, versions, false)
		data.Data.CompareFrom = r.URL.Query().Get("from")
		data.Data.CompareTo = r.URL.Query().Get("to")
		_, statErr := os.Stat(pathutils.ToFullPath(filePath))
		data.Data.FileDeleted = os.IsNotExist(statErr)

		err = tm.Render(w, "history", data)
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
			return
		}
		return
	}

	data := thememanager.NewHistoryTemplateData("", "", "", nil, false)
	data.Data.Collection = r.URL.Query().Get("collection")
	data.Data.Folder = r.URL.Query().Get("folder")

	err := tm.Render(w, "history", data)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, fmt.Sprintf("error rendering template: %v", err))
		return
	}
}
