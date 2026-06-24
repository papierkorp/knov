package render

import (
	"fmt"
	"html"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// RenderCodeMirrorEditorForm renders a CodeMirror editor for file creation/editing.
func RenderCodeMirrorEditorForm(filePath, prefillPath string, editorParam ...string) string {
	content := ""
	isEdit := filePath != ""

	if isEdit {
		fullPath := pathutils.ToDocsPath(filePath)
		if rawContent, err := contentStorage.ReadFile(fullPath); err == nil {
			content = string(rawContent)
		}
	}

	action := "/api/files/save"
	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	var currentEditor string
	if len(editorParam) > 0 {
		currentEditor = editorParam[0]
	}

	filepathInput := ""
	if !isEdit {
		filepathInput = fmt.Sprintf(`
				<div class="form-group">
					<label for="filepath-input">%s</label>
					<input type="text" id="filepath-input" name="filepath" required value="%s" placeholder="%s" class="form-input" />
					<script>(function(){var el=document.getElementById('filepath-input');if(el&&window.initPathAutocomplete)window.initPathAutocomplete(el,'/api/files/folder-suggestions');})()</script>
				</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
			html.EscapeString(prefillPath),
			translation.SprintfForRequest(configmanager.GetLanguage(), "my-file.md"))

		if currentEditor != "" {
			filepathInput += fmt.Sprintf(`<input type="hidden" name="editor" value="%s" />`, currentEditor)
		}
	} else {
		filepathInput = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, filePath)
	}

	es := configmanager.GetEditorSettings()
	jsBool := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}

	script := fmt.Sprintf(`<script>
(function() {
	var el = document.getElementById('codemirror-editor');
	(function() {
		var rect = el.getBoundingClientRect();
		var actions = document.querySelector('.file-form .form-actions');
		var actionsH = actions ? actions.offsetHeight + 48 : 80;
		var available = window.innerHeight - rect.top - actionsH;
		el.style.height = Math.max(300, available) + 'px';
	})();

	var view = createCodeMirror(el, %s, {
		vimMode:                        %s,
		lineNumbers:                    %s,
		relativeLineNumbers:            %s,
		foldGutter:                     %s,
		bracketMatching:                %s,
		autoBrackets:                   %s,
		highlightSelection:             %s,
		highlightSelectionWholeWord:    %s
	});
	view.contentDOM.setAttribute('spellcheck', '%s');

	initWikiAutocompleteForCodeMirror(view, {cursorEnd: %t});

	document.querySelector('.file-form').addEventListener('submit', function() {
		document.getElementById('editor-content').value = view.state.doc.toString();
	});
})();
</script>`,
		jsEscapeString(content),
		jsBool(es.CodeMirrorVimMode),
		jsBool(es.CodeMirrorLineNumbers),
		jsBool(es.CodeMirrorRelativeLineNumbers),
		jsBool(es.CodeMirrorFoldGutter),
		jsBool(es.CodeMirrorBracketMatching),
		jsBool(es.CodeMirrorAutoBrackets),
		jsBool(es.CodeMirrorHighlightSelection),
		jsBool(es.CodeMirrorHighlightSelectionWholeWord),
		jsBool(es.SpellCheck),
		es.WikiLinkCursorEnd)

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			%s
			<div class="form-group">
				<div id="codemirror-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
			</div>
			<div id="editor-status"></div>
		</form>
		%s`,
		action,
		filepathInput,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		script)
}
