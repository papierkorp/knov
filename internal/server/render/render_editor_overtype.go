// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"html"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// jsOverTypeInit returns the OverType editor constructor call.
// Reuses uploadMediaBlob (see render_editor_toastui.go) as the fileUpload.onInsertFile hook,
// so paste/drag-drop/toolbar upload all go through the same /api/media/upload flow.
func jsOverTypeInit(content, filePath string) string {
	toolbar := "true"
	if !configmanager.OverTypeShowToolbar.Get() {
		toolbar = "false"
	}
	spellcheck := "false"
	if configmanager.SpellCheck.Get() {
		spellcheck = "true"
	}
	return fmt.Sprintf(`
		var mountEl = document.getElementById('overtype-editor');
		(function() {
			var rect = mountEl.getBoundingClientRect();
			var actions = document.querySelector('.file-form .form-actions');
			var actionsH = actions ? actions.offsetHeight + 48 : 80;
			var available = window.innerHeight - rect.top - actionsH;
			mountEl.style.height = Math.max(300, available) + 'px';
		})();
		var editor = OverType.init(mountEl, {
			value: %s,
			toolbar: %s,
			spellcheck: %s,
			theme: document.body.getAttribute('data-dark-mode') === 'true' ? 'dark' : 'light',
			fileUpload: {
				enabled: true,
				onInsertFile: function(file) {
					return new Promise(function(resolve) {
						uploadMediaBlob(file, function(url, alt) {
							if (!url) { resolve(''); return; }
							var isImage = file.type.startsWith('image/');
							resolve(isImage ? '![' + alt + '](' + url + ')' : '[' + alt + '](' + url + ')');
						});
					});
				}
			}
		})[0];
		initWikiAutocompleteForInputs(mountEl, {cursorEnd: %t, currentFile: %s}, '.overtype-input');`,
		jsEscapeString(content), toolbar, spellcheck, configmanager.WikiLinkCursorEnd.Get(), jsEscapeString(filePath))
}

// jsOverTypeFormSubmit wires up the form submit listener to prepend any stashed YAML front
// matter before writing the editor content to the hidden field.
func jsOverTypeFormSubmit(frontMatter string) string {
	return fmt.Sprintf(`
		var frontMatter = %s;
		document.querySelector('.file-form').addEventListener('submit', function() {
			var body = editor.getValue();
			document.getElementById('editor-content').value = frontMatter ? frontMatter + body : body;
		});`, jsEscapeString(frontMatter))
}

// getOverTypeEditorScript assembles all JS helpers into a single <script> block.
func getOverTypeEditorScript(content, frontMatter, filePath string) string {
	return "<script>\n(function() {" +
		jsUploadMediaBlob() +
		jsOverTypeInit(content, filePath) +
		jsOverTypeFormSubmit(frontMatter) +
		"\n})();\n</script>"
}

// RenderOverTypeEditorForm renders an OverType editor form for file creation/editing.
// Strips YAML front matter before passing content to the editor and re-prepends on save.
// prefillPath pre-populates the file path input for new files (ignored when editing).
func RenderOverTypeEditorForm(filePath, prefillPath string, editor ...string) string {
	content := ""
	frontMatter := ""
	isEdit := filePath != ""

	if isEdit {
		fullPath := pathutils.ToDocsPath(filePath)
		rawContent, err := contentStorage.ReadFile(fullPath)
		if err == nil {
			fm, body := parser.StripFrontMatterBytes(rawContent)
			if fm != nil {
				frontMatter = "---\n" + string(fm) + "\n---\n"
				content = string(body)
			} else {
				content = string(rawContent)
			}
		}
	}

	action := "/api/files/save"
	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	var currentEditor string
	if len(editor) > 0 {
		currentEditor = editor[0]
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

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			%s
			<div class="form-group">
				<div id="overtype-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				<div id="editor-status"></div>
			</div>
		</form>
		%s`,
		action,
		filepathInput,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		getOverTypeEditorScript(content, frontMatter, filePath))
}

// RenderOverTypeSectionEditorForm renders an OverType editor form for editing a single section.
func RenderOverTypeSectionEditorForm(filePath, sectionID string) string {
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
		<form hx-post="/api/files/section/save" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="sectionid" value="%s" readonly />
			</div>
			<div class="form-group">
				<div id="overtype-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
				<input type="hidden" name="filepath" value="%s" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				<div id="editor-status"></div>
			</div>
		</form>
		%s`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		getOverTypeEditorScript(content, "", filePath))
}
