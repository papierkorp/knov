package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
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
// @Description Returns the appropriate editor based on file metadata or filetype parameter
// @Tags editor
// @Param filepath query string false "file path (optional for new files)"
// @Param filetype query string false "file type (optional for new files)"
// @Produce html
// @Router /api/editor [get]
func handleAPIGetEditorHandler(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	filetype := r.URL.Query().Get("filetype")

	var html string
	var editorType editorType
	var err error

	// if filetype parameter is provided (for new files), use that to determine editor
	if filetype != "" {
		switch files.Filetype(filetype) {
		case files.FileTypeTodo, files.FileTypeJournaling:
			editorType = editorTypeList
		case files.FileTypeFilter:
			editorType = editorTypeFilter
		case files.FileTypeMOC:
			editorType = editorTypeIndex
		case files.FileTypeFleeting, files.FileTypePermanent, files.FileTypeLiterature:
			editorType = editorTypeMarkdown
		default:
			editorType = editorTypeMarkdown
		}
	} else if filepath == "" {
		// no filepath and no filetype provided, default to markdown editor for new files
		html = render.RenderMarkdownEditorForm("")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	} else {
		// use existing logic for files with path
		editorType, err = GetEditor(filepath)
		if err != nil {
			logging.LogError("failed to determine editor type for %s: %v", filepath, err)
			editorType = editorTypeMarkdown // fallback
		}
	}

	// get file content if editing existing file
	var content string
	if filepath != "" {
		if rawContent, err := files.GetRawContent(filepath); err == nil {
			content = rawContent
		}
	}

	// render the appropriate editor
	switch editorType {
	case editorTypeMarkdown:
		html = render.RenderMarkdownEditorForm(filepath)
	case editorTypeTextarea:
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeList:
		// TODO: implement list editor, fallback to textarea for now
		html = render.RenderTextareaEditorComponent(filepath, content)
	case editorTypeFilter:
		var renderErr error
		html, renderErr = render.RenderFilterEditor(filepath)
		if renderErr != nil {
			logging.LogError("failed to render filter editor: %v", renderErr)
			html = render.RenderTextareaEditorComponent(filepath, content)
		}
	case editorTypeIndex:
		// TODO: implement index editor, fallback to textarea for now
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	content, err := files.GetRawContent(filepath)
	if err != nil {
		content = "" // empty for new files
	}

	html := render.RenderTextareaEditorComponent(filepath, content)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
