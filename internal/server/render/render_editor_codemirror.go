package render

import (
	"fmt"
	"html"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// RenderCodeMirrorSectionEditorForm renders a CodeMirror editor form for editing a single section.
func RenderCodeMirrorSectionEditorForm(filePath, sectionID string) string {
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
		jsBool(configmanager.CodeMirrorVimMode.Get()),
		jsBool(configmanager.CodeMirrorLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorRelativeLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorFoldGutter.Get()),
		jsBool(configmanager.CodeMirrorBracketMatching.Get()),
		jsBool(configmanager.CodeMirrorAutoBrackets.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelection.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelectionWholeWord.Get()),
		jsBool(configmanager.SpellCheck.Get()),
		configmanager.WikiLinkCursorEnd.Get())

	return fmt.Sprintf(`
		<form hx-post="/api/files/section/save" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="sectionid" value="%s" readonly />
			</div>
			<div class="form-group">
				<div id="codemirror-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
				<input type="hidden" name="filepath" value="%s" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
			</div>
			<div id="editor-status"></div>
		</form>
		%s`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		script)
}

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
		jsBool(configmanager.CodeMirrorVimMode.Get()),
		jsBool(configmanager.CodeMirrorLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorRelativeLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorFoldGutter.Get()),
		jsBool(configmanager.CodeMirrorBracketMatching.Get()),
		jsBool(configmanager.CodeMirrorAutoBrackets.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelection.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelectionWholeWord.Get()),
		jsBool(configmanager.SpellCheck.Get()),
		configmanager.WikiLinkCursorEnd.Get())

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
