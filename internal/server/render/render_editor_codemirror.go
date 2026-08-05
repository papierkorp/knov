package render

import (
	"fmt"
	"html"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

type codeMirrorToolbarBtn struct {
	cmd, arg, label, titleKey string
}

// codeMirrorToolbarGroups defines the formatting toolbar buttons, grouped with a separator between groups.
func codeMirrorToolbarGroups() [][]codeMirrorToolbarBtn {
	return [][]codeMirrorToolbarBtn{
		{
			{"heading", "1", "H1", "heading 1"},
			{"heading", "2", "H2", "heading 2"},
			{"heading", "3", "H3", "heading 3"},
		},
		{
			{"bold", "", "<strong>B</strong>", "bold"},
			{"italic", "", "<em>I</em>", "italic"},
			{"strikethrough", "", "<s>S</s>", "strikethrough"},
			{"inlineCode", "", "&lt;/&gt;", "inline code"},
		},
		{
			{"quote", "", `<i class="fa fa-quote-left"></i>`, "quote"},
			{"bulletList", "", `<i class="fa fa-list-ul"></i>`, "bullet list"},
			{"orderedList", "", `<i class="fa fa-list-ol"></i>`, "numbered list"},
		},
		{
			{"link", "", "Link", "link"},
			{"codeBlock", "", "{ }", "code block"},
			{"hr", "", "&mdash;", "horizontal rule"},
		},
		{
			{"uploadFile", "", `<i class="fa fa-upload"></i>`, "upload file"},
			{"insertMedia", "", `<i class="fa fa-image"></i>`, "insert media"},
			{"insertWikiLink", "", `<i class="fa fa-file-lines"></i>`, "insert wiki link"},
		},
	}
}

// codeMirrorToolbarHTML renders the formatting toolbar shown above the CodeMirror editor,
// ending with the editor settings menu button (see codeMirrorSettingsMenuHTML).
func codeMirrorToolbarHTML(lang string) string {
	if !configmanager.CodeMirrorShowToolbar.Get() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<div id="component-codemirror-toolbar">`)
	for i, group := range codeMirrorToolbarGroups() {
		if i > 0 {
			sb.WriteString(`<span class="cm-toolbar-sep"></span>`)
		}
		for _, btn := range group {
			argAttr := ""
			if btn.arg != "" {
				argAttr = fmt.Sprintf(` data-arg="%s"`, btn.arg)
			}
			fmt.Fprintf(&sb, `<button type="button" data-cmd="%s"%s title="%s">%s</button>`,
				btn.cmd, argAttr, translation.SprintfForRequest(lang, btn.titleKey), btn.label)
		}
	}
	sb.WriteString(`<span class="cm-toolbar-sep"></span>`)
	sb.WriteString(codeMirrorSettingsMenuHTML(lang))
	sb.WriteString(`</div>`)
	return sb.String()
}

// jsCodeMirrorToolbar wires the formatting toolbar buttons to window.mdCommands.
func jsCodeMirrorToolbar() string {
	if !configmanager.CodeMirrorShowToolbar.Get() {
		return ""
	}
	return `
	document.getElementById('component-codemirror-toolbar').addEventListener('click', function(e) {
		var btn = e.target.closest('button[data-cmd]');
		if (!btn) return;
		if (btn.dataset.cmd === 'uploadFile') {
			document.getElementById('codemirror-file-input').click();
			return;
		}
		if (btn.dataset.cmd === 'insertMedia') {
			view.cmInsertMedia();
			return;
		}
		if (btn.dataset.cmd === 'insertWikiLink') {
			view.cmInsertWikiLink();
			return;
		}
		var arg = btn.dataset.arg ? Number(btn.dataset.arg) : undefined;
		window.mdCommands[btn.dataset.cmd](view, arg);
	});`
}

// codeMirrorSettingsMenuHTML renders the gear-menu button, placed last in the toolbar, that
// exposes the CodeMirror (Group: GroupCodeMirror) settings directly in the editor so they can
// be toggled without leaving the page. CodeMirrorShowToolbar is excluded — it stays an
// app-wide /settings-only setting since it controls whether this toolbar (and menu) exists at
// all. Reuses renderSettingItem, the same per-setting htmx form used on the /settings page, so
// persistence and markup stay identical there.
func codeMirrorSettingsMenuHTML(lang string) string {
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
	var sb strings.Builder
	sb.WriteString(`<div id="component-codemirror-settings" class="fp-menu-wrap">`)
	fmt.Fprintf(&sb, `<button type="button" class="fp-menu-btn" onclick="toggleFpFileMenu(this)" title="%s"><i class="fa fa-gear"></i></button>`,
		t("editor settings"))
	sb.WriteString(`<div class="fp-menu" hidden>`)
	for _, s := range configmanager.SettingsBySection(configmanager.SectionEditor) {
		if s.GetMeta().Group == configmanager.GroupCodeMirror && s.Key() != configmanager.CodeMirrorShowToolbar.Key() {
			sb.WriteString(renderSettingItem(s, t))
		}
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// jsCodeMirrorSettingsMenu wires the editor settings menu: each boolean toggle persists via
// its own htmx form (see renderSettingItem) and is additionally applied live by reconfiguring
// the running CodeMirror view via reinitCodeMirror.
func jsCodeMirrorSettingsMenu() string {
	if !configmanager.CodeMirrorShowToolbar.Get() {
		return ""
	}
	return `
	document.getElementById('component-codemirror-settings').addEventListener('change', function(e) {
		var cb = e.target.closest('input[type=checkbox][name^="codeMirror"]');
		if (!cb) return;
		var opt = cb.name.slice('codeMirror'.length);
		opt = opt.charAt(0).toLowerCase() + opt.slice(1);
		var patch = {};
		patch[opt] = cb.checked;
		reinitCodeMirror(patch);
	});`
}

// jsCodeMirrorFileUpload wires multi-file upload (toolbar button + drag-and-drop) into the
// editor. Uploaded files are inserted as markdown image links (images) or plain links (other
// files) at the current cursor position. Relies on uploadMediaBlob (render_editor_shared.go).
func jsCodeMirrorFileUpload() string {
	return `
	function insertMarkdownAtCursor(markdown) {
		var pos = view.state.selection.main.to;
		view.dispatch({changes: {from: pos, to: pos, insert: markdown}, selection: {anchor: pos + markdown.length}});
		view.focus();
	}
	function uploadFilesToEditor(fileList) {
		Array.from(fileList).forEach(function(file) {
			uploadMediaBlob(file, function(url, alt) {
				if (!url) return;
				var markdown = file.type.startsWith('image/') ? '![' + alt + '](' + url + ')' : '[' + alt + '](' + url + ')';
				insertMarkdownAtCursor(markdown);
			});
		});
	}
	document.getElementById('codemirror-file-input').addEventListener('change', function(e) {
		uploadFilesToEditor(e.target.files);
		e.target.value = '';
	});
	el.addEventListener('dragover', function(e) {
		e.preventDefault();
		e.dataTransfer.dropEffect = 'copy';
		el.classList.add('cm-dragover');
	});
	el.addEventListener('dragleave', function(e) {
		el.classList.remove('cm-dragover');
	});
	el.addEventListener('drop', function(e) {
		e.preventDefault();
		el.classList.remove('cm-dragover');
		if (e.dataTransfer.files && e.dataTransfer.files.length) {
			uploadFilesToEditor(e.dataTransfer.files);
		}
	});`
}

// codeMirrorFileInputHTML renders the hidden multi-file input used by the upload toolbar button.
func codeMirrorFileInputHTML() string {
	return `<input type="file" id="codemirror-file-input" multiple hidden />`
}

// codeMirrorInitScript renders the JS that boots the CodeMirror view, wires up
// autocomplete/toolbar/upload, and defines reinitCodeMirror — used by the settings
// menu (jsCodeMirrorSettingsMenu) to apply toggled options to the live editor by
// rebuilding the view in place, without losing unsaved content or reloading the page.
func codeMirrorInitScript(content, filePath string) string {
	jsBool := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf(`
	var cmOptions = {
		vimMode:                        %s,
		lineNumbers:                    %s,
		relativeLineNumbers:            %s,
		foldGutter:                     %s,
		bracketMatching:                %s,
		autoBrackets:                   %s,
		highlightSelection:             %s,
		highlightSelectionWholeWord:    %s,
		wysiwyg:                        %s
	};
	var wikiOpts = {cursorEnd: %t, currentFile: %s};
	var view = createCodeMirror(el, %s, cmOptions);
	view.contentDOM.setAttribute('spellcheck', '%s');
	initWikiAutocompleteForCodeMirror(view, wikiOpts);

	function reinitCodeMirror(patch) {
		Object.assign(cmOptions, patch);
		var pos = Math.min(view.state.selection.main.anchor, view.state.doc.length);
		var newContent = view.state.doc.toString();
		view.destroy();
		el.innerHTML = '';
		view = createCodeMirror(el, newContent, cmOptions);
		view.contentDOM.setAttribute('spellcheck', '%s');
		view.dispatch({selection: {anchor: pos}});
		initWikiAutocompleteForCodeMirror(view, wikiOpts);
		view.focus();
	}

	%s
	%s
	%s
	%s`,
		jsBool(configmanager.CodeMirrorVimMode.Get()),
		jsBool(configmanager.CodeMirrorLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorRelativeLineNumbers.Get()),
		jsBool(configmanager.CodeMirrorFoldGutter.Get()),
		jsBool(configmanager.CodeMirrorBracketMatching.Get()),
		jsBool(configmanager.CodeMirrorAutoBrackets.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelection.Get()),
		jsBool(configmanager.CodeMirrorHighlightSelectionWholeWord.Get()),
		jsBool(configmanager.CodeMirrorWysiwyg.Get()),
		configmanager.WikiLinkCursorEnd.Get(),
		jsEscapeString(filePath),
		jsEscapeString(content),
		jsBool(configmanager.SpellCheck.Get()),
		jsBool(configmanager.SpellCheck.Get()),
		jsCodeMirrorToolbar(),
		jsCodeMirrorSettingsMenu(),
		jsUploadMediaBlob(),
		jsCodeMirrorFileUpload())
}

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
	%s
	document.querySelector('.file-form').addEventListener('submit', function() {
		document.getElementById('editor-content').value = view.state.doc.toString();
	});
})();
</script>`, codeMirrorInitScript(content, filePath))

	return fmt.Sprintf(`
		<form hx-post="/api/files/section/save" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="sectionid" value="%s" readonly />
			</div>
			<div class="form-group">
				%s
				<div id="codemirror-editor"></div>
				%s
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
		codeMirrorToolbarHTML(configmanager.GetLanguage()),
		codeMirrorFileInputHTML(),
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
	%s
	document.querySelector('.file-form').addEventListener('submit', function() {
		document.getElementById('editor-content').value = view.state.doc.toString();
	});
})();
</script>`, codeMirrorInitScript(content, filePath))

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" hx-swap="innerHTML" class="file-form">
			%s
			<div class="form-group">
				%s
				<div id="codemirror-editor"></div>
				%s
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
		codeMirrorToolbarHTML(configmanager.GetLanguage()),
		codeMirrorFileInputHTML(),
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		script)
}
