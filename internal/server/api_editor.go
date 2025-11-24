package server

import (
	"net/http"

	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/utils"
)

// editorType defines the type of editor to be used
type editorType string

const (
	editorTypeMarkdown editorType = "markdown-editor"
	editorTypeTextarea editorType = "textarea-editor"
	editorTypeFilter   editorType = "filter-editor"
	editorTypeList     editorType = "list-editor"
	editorTypeIndex    editorType = "index-editor"
)

// GetEditor determines the appropriate editor type for a file based on its metadata
func GetEditor(filepath string) (editorType, error) {
	metadata, err := files.MetaDataGet(filepath)

	// always detect syntax from file type handler
	handler := files.GetParserRegistry().GetHandler(filepath)
	var handlerName string
	if handler != nil {
		handlerName = handler.Name()
	} else {
		handlerName = "markdown" // fallback
	}

	// if metadata exists, use FileType to determine editor
	if err == nil && metadata != nil {
		switch metadata.FileType {
		case files.FileTypeTodo, files.FileTypeJournaling:
			return editorTypeList, nil
		case files.FileTypeFilter:
			return editorTypeFilter, nil
		case files.FileTypeMOC:
			return editorTypeIndex, nil
		case files.FileTypeFleeting, files.FileTypePermanent, files.FileTypeLiterature:
			if handlerName == "markdown" {
				return editorTypeMarkdown, nil
			}
			// dokuwiki and plaintext both use textarea editor
			return editorTypeTextarea, nil
		}
	}

	// for new files or fallback, use handler name to determine editor
	if handlerName == "markdown" {
		return editorTypeMarkdown, nil
	}
	// dokuwiki and plaintext both use textarea editor
	return editorTypeTextarea, nil
}

// @Summary Get appropriate editor for file
// @Description Returns the appropriate editor based on file metadata
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Produce html
// @Router /api/editor [get]
func handleAPIGetEditorHandler(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")

	// if no filepath provided, default to markdown editor for new files
	if filepath == "" {
		html := render.RenderMarkdownEditorForm("")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	editorType, err := GetEditor(filepath)
	if err != nil {
		logging.LogError("failed to determine editor type for %s: %v", filepath, err)
		// fallback to markdown editor on error
		editorType = editorTypeMarkdown
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	var html string
	switch editorType {
	case editorTypeMarkdown:
		html = render.RenderMarkdownEditorForm(filepath)
	case editorTypeTextarea:
		if err != nil {
			content = ""
		}
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeList:
		// TODO: implement list editor
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeFilter:
		// TODO: implement filter editor
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeIndex:
		// TODO: implement index editor
		html = render.RenderTextareaEditorComponent(filepath, content)
	default:
		html = render.RenderMarkdownEditorForm(filepath)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get markdown editor form HTML
// @Description Returns a markdown editor form for creating or editing files
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Produce html
// @Router /api/editor/markdown-form [get]
func handleAPIMarkdownEditorForm(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")

	html := render.RenderMarkdownEditorForm(filePath)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get textarea editor component
// @Description Returns a simple textarea editor component for editing file content
// @Tags editor
// @Param filepath query string true "file path"
// @Produce html
// @Router /api/editor/textarea [get]
func handleAPIGetTextareaEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		content = "" // empty for new files
	}

	html := render.RenderTextareaEditorComponent(filepath, content)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
