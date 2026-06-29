package render

import (
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// RenderTextareaSectionEditorForm renders a plain textarea editor form for editing a single section.
func RenderTextareaSectionEditorForm(filePath, sectionID string) string {
	content := ""

	if filePath != "" && sectionID != "" {
		handler := contentHandler.GetHandler("markdown")
		includeSubheaders := configmanager.GetSectionEditIncludeSubheaders()
		sectionContent, err := handler.ExtractSection(filePath, sectionID, includeSubheaders)
		if err == nil {
			content = sectionContent
		}
	}

	cancelURL := fmt.Sprintf("/files/%s#%s", filePath, sectionID)

	return fmt.Sprintf(`
		<div class="component-textarea-editor">
			<form hx-post="/api/files/section/save" hx-target="#editor-status" hx-swap="innerHTML">
				<div class="form-group">
					<label>%s:</label>
					<input type="text" name="sectionid" value="%s" readonly />
				</div>
				<input type="hidden" name="filepath" value="%s" />
				<textarea name="content" rows="25" class="textarea-editor-input">%s</textarea>
				<div class="form-actions">
					<button type="submit" class="btn-primary">%s</button>
					<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				</div>
				<div id="editor-status"></div>
			</form>
			<script>(function(){var c=document.currentScript.parentElement;if(window.initWikiAutocompleteForInputs)initWikiAutocompleteForInputs(c,{cursorEnd:%t},'.textarea-editor-input');})()</script>
		</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		content,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		configmanager.WikiLinkCursorEnd.Get())
}

// RenderTextareaEditorComponent renders a plain textarea editor with save/cancel buttons.
// Shows an extra "convert to markdown" button for DokuWiki files.
func RenderTextareaEditorComponent(filepath, content string, editorType ...string) string {
	isNew := filepath == ""
	cancelURL := "/"
	if !isNew {
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	var convertButton string
	if !isNew {
		fullPath := pathutils.ToDocsPath(filepath)
		handler := parser.GetParserRegistry().GetHandler(fullPath)
		if handler != nil && handler.Name() == "dokuwiki" {
			convertButton = fmt.Sprintf(`
				<button type="button"
						hx-post="/api/files/convert-to-markdown"
						hx-vals='{"filepath": "%s"}'
						hx-swap="none"
						class="btn-secondary">
					%s
				</button>`,
				filepath,
				translation.SprintfForRequest(configmanager.GetLanguage(), "convert to markdown"))
		}
	}

	var filepathField string
	if isNew {
		var editorHidden string
		if len(editorType) > 0 && editorType[0] != "" {
			editorHidden = fmt.Sprintf(`<input type="hidden" name="editor" value="%s">`, editorType[0])
		}
		filepathField = fmt.Sprintf(`
			<div class="form-group">
				<label for="filepath-input">%s</label>
				<input type="text" id="filepath-input" name="filepath" required placeholder="%s" class="form-input" />
				<script>(function(){var el=document.getElementById('filepath-input');if(el&&window.initPathAutocomplete)window.initPathAutocomplete(el,'/api/files/folder-suggestions');})()</script>
			</div>%s`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "my-file.md"),
			editorHidden)
	} else {
		filepathField = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s">`, filepath)
	}

	return fmt.Sprintf(`
		<div class="component-textarea-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status" hx-swap="innerHTML">
				%s
				<textarea name="content" rows="25" class="textarea-editor-input">%s</textarea>
				<div class="form-actions">
					<button type="submit" class="btn-primary">%s</button>
					<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
					%s
				</div>
				<div id="editor-status"></div>
			</form>
			<script>(function(){var c=document.currentScript.parentElement;if(window.initWikiAutocompleteForInputs)initWikiAutocompleteForInputs(c,{cursorEnd:%t},'.textarea-editor-input');})()</script>
		</div>`,
		filepathField,
		content,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		convertButton,
		configmanager.WikiLinkCursorEnd.Get())
}
