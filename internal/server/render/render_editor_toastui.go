// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to marshal string for javascript: %v", err)
		return `""`
	}
	return string(jsonBytes)
}

// jsEditorInit returns the ToastUI editor constructor call.
// Binds the upload hook so blob uploads go through uploadMediaBlob.
func jsEditorInit(content string) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	initialView := configmanager.ToastuiInitialView.Get()
	if initialView == "" {
		initialView = "markdown"
	}
	previewStyle := configmanager.ToastuiPreviewStyle.Get()
	if previewStyle == "" {
		previewStyle = "tab"
	}
	spellcheck := "false"
	if configmanager.SpellCheck.Get() {
		spellcheck = "true"
	}
	hideModeSwitch := "false"
	if !configmanager.ToastuiShowModeSwitch.Get() {
		hideModeSwitch = "true"
	}
	// when toolbar is hidden pass an empty array; otherwise use the configured items
	toolbarItemsJS := fmt.Sprintf(`[
				['heading', 'bold', 'italic', 'strike'],
				['hr', 'quote'],
				['ul', 'ol', 'task', 'indent', 'outdent'],
				['table', 'image', 'link'],
				[{
					name: 'selectMedia',
					tooltip: %s,
					el: (() => {
						const button = document.createElement('button');
						button.className = 'toastui-editor-toolbar-icons';
						button.style.backgroundImage = 'none';
						button.style.margin = '0';
						button.innerHTML = '<i class="fa-solid fa-file-arrow-up"></i>';
						button.addEventListener('click', () => showMediaSelector(editor));
						return button;
					})()
				}, {
					name: 'selectWikiFile',
					tooltip: %s,
					el: (() => {
						const button = document.createElement('button');
						button.className = 'toastui-editor-toolbar-icons';
						button.style.backgroundImage = 'none';
						button.style.margin = '0';
						button.innerHTML = '<i class="fa-solid fa-file-lines"></i>';
						button.addEventListener('click', () => showWikiFileSelector(editor));
						return button;
					})()
				}],
				['code', 'codeblock']
			]`, jsEscapeString(t("Select Media")), jsEscapeString(t("Insert Wiki File Link")))
	if !configmanager.ToastuiShowToolbar.Get() {
		toolbarItemsJS = "[]"
	}
	return fmt.Sprintf(`
		// override built-in locale to rename 'Insert Image' to 'Insert Media'
		toastui.Editor.setLanguage('en-US', {
			'Insert image': 'Insert Media',
			'Insert Image': 'Insert Media',
			'image': 'media',
			'Image': 'Media',
		});
		const editor = new toastui.Editor({
			el: document.querySelector('#toastui-editor'),
			height: (function() {
				var el = document.querySelector('#toastui-editor');
				var rect = el.getBoundingClientRect();
				var actions = document.querySelector('.file-form .form-actions');
				var actionsH = actions ? actions.offsetHeight + 48 : 80;
				var available = window.innerHeight - rect.top - actionsH;
				return Math.max(300, available) + 'px';
			})(),
			initialEditType: '%s',
			previewStyle: '%s',
			initialValue: %s,
			hideModeSwitch: %s,
			theme: document.body.getAttribute('data-dark-mode') === 'true' ? 'dark' : 'default',
			language: 'en-US',
			toolbarItems: %s,
			i18n: {
				'File': 'File',
				'URL': 'URL',
				'Select image': 'Select file',
				'Select Image': 'Select File',
				'File URL': 'File URL',
				'Image URL': 'File URL',
				'Description': 'Description',
				'OK': 'OK',
				'Cancel': 'Cancel',
				'Insert Image': 'Insert Media',
				'Insert image': 'Insert media',
				'image': 'media',
				'Image': 'Media',
				'Choose a file': 'Choose a file',
				'No file selected': 'No file selected'
			},
			hooks: {
				addImageBlobHook: function(blob, callback) {
					uploadMediaBlob(blob, function(url, alt) {
						if (!url) { callback('', alt); return; }
						// non-image files: insert as a plain link so ToastUI does not
						// wrap the result in ![]() image syntax
						if (!blob.type.startsWith('image/')) {
							editor.insertText('[' + alt + '](' + url + ')');
							callback('', ''); // empty → suppresses ToastUI own insertion
						} else {
							callback(url, alt);
						}
					});
					return false;
				}
			}
		});
		document.querySelectorAll('#toastui-editor [contenteditable]').forEach(function(el) {
			el.setAttribute('spellcheck', '%s');
		});
		(document.querySelector('#toastui-editor .toastui-editor-toolbar') || {style:{}}).style.display = '%s';`, initialView, previewStyle, jsEscapeString(content), hideModeSwitch, toolbarItemsJS, spellcheck, func() string {
		if !configmanager.ToastuiShowToolbar.Get() {
			return "none"
		}
		return ""
	}())
}

// jsFileInputAcceptAll patches the built-in image popup: file input accepts all types and
// multi-select, and OK is intercepted (capture phase, ahead of ToastUI's own click handler on
// the same button) so multi-file uploads only happen once OK is pressed, not on selection.
// ToastUI's own OK handler only ever inserts files[0], so single-file selection is untouched.
func jsFileInputAcceptAll() string {
	return `
		// patch built-in image popup: allow multi-select, upload all files on OK press.
		// The click listener sits on the popup (an ancestor of the OK button) in the
		// capture phase, so for multi-file selections it runs and stops the event before
		// ToastUI's own click handler (bound directly to the OK button) ever sees it.
		function patchImagePopup(popupEl) {
			const input = popupEl.querySelector('input[type="file"]');
			if (!input || input.dataset.multiUploadPatched) return;
			input.dataset.multiUploadPatched = 'true';
			input.setAttribute('accept', '*');
			input.setAttribute('multiple', 'multiple');
			popupEl.addEventListener('click', function(e) {
				if (!e.target.closest('.toastui-editor-ok-button')) return;
				const files = Array.from(input.files || []);
				if (files.length <= 1) return;
				e.preventDefault();
				e.stopPropagation();
				files.forEach(function(file) {
					uploadMediaBlob(file, function(url, alt) {
						if (!url) return;
						const isImage = file.type.startsWith('image/');
						editor.insertText(isImage ? '![' + alt + '](' + url + ')' : '[' + alt + '](' + url + ')');
					});
				});
				input.value = '';
				const closeButton = popupEl.querySelector('.toastui-editor-close-button');
				if (closeButton) closeButton.click();
			}, true);
		}

		setTimeout(function() {
			document.querySelectorAll('.toastui-editor-popup-add-image').forEach(patchImagePopup);
		}, 500);

		// also patch lazily-rendered popups via MutationObserver. ToastUI reuses a single
		// hidden popup wrapper for image/table/link popups, swapping its class and content
		// each time rather than creating a fresh node, so the wrapper itself is never an
		// "added" node after the first use — closest() finds it via the newly-added children.
		const popupObserver = new MutationObserver(function(mutations) {
			mutations.forEach(function(mutation) {
				mutation.addedNodes.forEach(function(node) {
					if (node.nodeType !== 1) return;
					const popupEl = (node.closest && node.closest('.toastui-editor-popup-add-image'))
						|| (node.querySelector && node.querySelector('.toastui-editor-popup-add-image'));
					if (popupEl) patchImagePopup(popupEl);
				});
			});
		});
		popupObserver.observe(document.body, { childList: true, subtree: true });`
}

// jsDragAndDrop adds drag-and-drop support for all media file types onto the editor element.
// Images are inserted as ![alt](url), other files as [alt](url).
func jsDragAndDrop() string {
	return `
		// drag-and-drop: accept all file types, insert as markdown image or link
		// captured before ToastUI's own drop handler so multi-file drops aren't
		// truncated to the single file ToastUI's built-in handling supports
		const editorEl = document.querySelector('#toastui-editor');
		editorEl.addEventListener('dragover', function(e) {
			e.preventDefault();
			e.dataTransfer.dropEffect = 'copy';
		}, true);
		editorEl.addEventListener('drop', function(e) {
			e.preventDefault();
			e.stopPropagation();
			const files = e.dataTransfer.files;
			if (!files || files.length === 0) return;
			Array.from(files).forEach(function(file) {
				uploadMediaBlob(file, function(url, alt) {
					if (!url) return;
					const isImage = file.type.startsWith('image/');
					const markdown = isImage
						? '![' + alt + '](' + url + ')'
						: '[' + alt + '](' + url + ')';
					editor.insertText(markdown);
				});
			});
		}, true);`
}

// jsUploadMediaBlob defines the shared upload helper used by the blob hook and drag-and-drop.
// Derives the context path from the current URL, shows an upload notification, then POSTs
// to /api/media/upload and calls callback(url, alt) on success.
func jsUploadMediaBlob() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	return fmt.Sprintf(`
		// shared upload helper: derives context from URL, uploads, calls callback(url, alt)
		function uploadMediaBlob(blob, callback) {
			const currentPath = window.location.pathname;
			let contextPath = null;

			if (currentPath.startsWith('/files/edit/')) {
				contextPath = currentPath.substring('/files/edit/'.length);
			} else if (currentPath.startsWith('/files/')) {
				contextPath = currentPath.substring('/files/'.length);
			}

			if (!contextPath) {
				alert(%s);
				callback('', '');
				return;
			}

			const formData = new FormData();
			formData.append('file', blob);
			formData.append('context_path', contextPath);

			const uploadMessage = document.createElement('div');
			uploadMessage.className = 'upload-notification';
			uploadMessage.style.cssText = 'position:fixed;top:10px;right:10px;padding:12px 16px;border-radius:6px;z-index:9999;font-weight:500;box-shadow:0 4px 12px color-mix(in srgb, var(--text) 15%%, transparent);background:var(--primary);color:var(--surface);';
			uploadMessage.textContent = %s;
			document.body.appendChild(uploadMessage);

			fetch('/api/media/upload', {
				method: 'POST',
				body: formData,
				headers: { 'Accept': 'application/json' }
			})
			.then(function(response) {
				if (!response.ok) {
					return response.text().then(function(t) {
						throw new Error(t || (%s + response.statusText));
					});
				}
				return response.json();
			})
			.then(function(data) {
				if (document.body.contains(uploadMessage)) document.body.removeChild(uploadMessage);
				callback('media/' + data.path, data.filename || blob.name || %s);
			})
			.catch(function(error) {
				if (document.body.contains(uploadMessage)) document.body.removeChild(uploadMessage);
				alert(%s + error.message);
				callback('', '');
			});
		}`,
		jsEscapeString(t("please save the document first to enable file uploads.")),
		jsEscapeString(t("uploading...")),
		jsEscapeString(t("upload failed: ")),
		jsEscapeString(t("uploaded file")),
		jsEscapeString(t("failed to upload file: ")),
	)
}

// jsMediaSelector defines the media browser modal (showMediaSelector / closeMediaSelector).
// Drives the #media-selector-modal popover (see renderMediaSelectorModalHTML) via the native
// Popover API, and fetches /api/media/list?mode=select as HTML into its body.
func jsMediaSelector() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	return fmt.Sprintf(`
		// media browser modal — opened by the toolbar "Select Media" button
		window.showMediaSelector = function(editor) {
			const modal = document.getElementById('media-selector-modal');
			const body = document.getElementById('media-selector-body');
			const input = document.getElementById('media-selector-search-input');
			if (input) input.value = '';
			body.innerHTML = %s;

			fetch('/api/media/list?mode=select', { headers: { 'Accept': 'text/html' } })
				.then(function(r) { return r.text(); })
				.then(function(html) { body.innerHTML = html; })
				.catch(function() { body.innerHTML = %s; });

			modal.showPopover();
			if (input) setTimeout(function() { input.focus(); }, 50);
		};

		window.closeMediaSelector = function() {
			document.getElementById('media-selector-modal').hidePopover();
		};

		window.filterMediaSelectorList = function(query) {
			const q = query.toLowerCase();
			document.querySelectorAll('#media-selector-modal .media-select-item').forEach(function(item) {
				const name = (item.querySelector('.media-select-name') || {}).textContent || '';
				item.style.display = name.toLowerCase().includes(q) ? '' : 'none';
			});
		};`,
		jsEscapeString(t("loading media files...")),
		jsEscapeString(t("error loading media files")),
	)
}

// renderMediaSelectorModalHTML renders the (initially closed) media browser popover markup.
// Uses the app's existing native [popover] modal idiom (see base.gohtml's rail-modals) instead
// of building the modal chrome in JS, so show/hide/backdrop/Escape all come from the browser
// for free and every string here is server-rendered/translated rather than JS-built.
func renderMediaSelectorModalHTML() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	return fmt.Sprintf(`
		<div id="media-selector-modal" popover>
			<div class="modal-content media-selector-content">
				<div class="media-selector-header">
					<h3>%s</h3>
					<button type="button" popovertarget="media-selector-modal" popovertargetaction="hide" class="btn-icon">&times;</button>
				</div>
				<input type="text" id="media-selector-search-input" class="form-input" placeholder="%s" oninput="filterMediaSelectorList(this.value)" />
				<div id="media-selector-body"></div>
			</div>
		</div>`,
		t("select media file"),
		t("filter..."),
	)
}

// jsInsertMedia defines insertMediaIntoEditor and insertMediaLink — called by the media
// selector list items to insert the chosen file into the editor as a markdown link.
func jsInsertMedia() string {
	return `
		// insert selected media from the browser modal into the editor
		window.insertMediaIntoEditor = function(element) {
			const mediaPath = element.querySelector('.media-path').value;
			const filename  = element.querySelector('.media-filename').value;
			const editor    = window.currentEditor;
			if (editor) {
				editor.insertText('![' + filename + '](media/' + mediaPath + ')');
			}
			closeMediaSelector();
		};

		window.insertMediaLink = function(mediaURL, filename) {
			const editor = window.currentEditor;
			if (editor) {
				editor.insertText('![' + filename + '](' + mediaURL + ')');
			}
			closeMediaSelector();
		};`
}

// jsInsertWikiFile defines insertWikiFileIntoEditor — called by the wiki-file selector list
// items to insert the chosen file into the editor as a markdown link. Builds the "](...)"
// target via wiki-autocomplete.js's buildTarget so paths get the same encoding (spaces, "#",
// etc.) as the inline [[wikilink]] autocomplete instead of duplicating that logic here.
func jsInsertWikiFile() string {
	return `
		// insert selected wiki file from the browser modal into the editor
		window.insertWikiFileIntoEditor = function(element) {
			const path     = element.querySelector('.wiki-file-path').value;
			const filename = element.querySelector('.wiki-file-filename').value;
			const label    = filename.replace(/\.[^.]+$/, '');
			const editor   = window.currentEditor;
			if (editor) {
				editor.insertText('[' + label + '](' + buildTarget(path) + ')');
			}
			closeWikiFileSelector();
		};`
}

// jsFormSubmit wires up the form submit listener to prepend any stashed YAML front matter
// before writing the editor content to the hidden field.
func jsFormSubmit(frontMatter string) string {
	return fmt.Sprintf(`
		// on submit: prepend stashed YAML front matter (if any) before saving
		const frontMatter = %s;
		document.querySelector('.file-form').addEventListener('submit', function() {
			const body = editor.getMarkdown();
			document.getElementById('editor-content').value = frontMatter ? frontMatter + body : body;
		});`, jsEscapeString(frontMatter))
}

// jsWikiFileSelector defines the wiki-file browser modal (showWikiFileSelector /
// closeWikiFileSelector). Drives the #wiki-file-selector-modal popover (see
// renderWikiFileSelectorModalHTML) via the native Popover API, and fetches
// /api/files/autocomplete as HTML into its body — same endpoint the inline [[wikilink]]
// autocomplete uses, just requesting text/html instead of json (see wiki-autocomplete.js).
func jsWikiFileSelector() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	return fmt.Sprintf(`
		window.showWikiFileSelector = function(editor) {
			const modal = document.getElementById('wiki-file-selector-modal');
			const body = document.getElementById('wiki-file-selector-body');
			const input = document.getElementById('wiki-file-selector-search-input');
			if (input) input.value = '';

			var debounceTimer;
			function fetchFiles(q) {
				clearTimeout(debounceTimer);
				debounceTimer = setTimeout(function() {
					fetch('/api/files/autocomplete?q=' + encodeURIComponent(q), { headers: { 'Accept': 'text/html' } })
						.then(function(r) { return r.text(); })
						.then(function(html) { body.innerHTML = html; })
						.catch(function() { body.innerHTML = %s; });
				}, 120);
			}

			if (input) input.oninput = function() { fetchFiles(this.value); };
			fetchFiles('');
			modal.showPopover();
			if (input) setTimeout(function() { input.focus(); }, 50);
		};

		window.closeWikiFileSelector = function() {
			document.getElementById('wiki-file-selector-modal').hidePopover();
		};`,
		jsEscapeString(t("error loading files")),
	)
}

// renderWikiFileSelectorModalHTML renders the (initially closed) wiki-file-link popover markup.
// Same native [popover] idiom as renderMediaSelectorModalHTML.
func renderWikiFileSelectorModalHTML() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	return fmt.Sprintf(`
		<div id="wiki-file-selector-modal" popover>
			<div class="modal-content media-selector-content">
				<div class="media-selector-header">
					<h3>%s</h3>
					<button type="button" popovertarget="wiki-file-selector-modal" popovertargetaction="hide" class="btn-icon">&times;</button>
				</div>
				<input type="text" id="wiki-file-selector-search-input" class="form-input" placeholder="%s" />
				<div id="wiki-file-selector-body"></div>
			</div>
		</div>`,
		t("insert wiki file link"),
		t("filter..."),
	)
}

// WikiFileResult is a single autocomplete match, shared between the JSON response and the
// server-rendered wiki-file-selector list (see handleAPIFilesAutocomplete in the server
// package / jsWikiFileSelector above).
type WikiFileResult struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

// RenderWikiFileAutocompleteList renders file search results as HTML for the wiki-file-link
// modal body. Mirrors RenderMediaListSelect: each item stores its insertion data in hidden
// inputs and calls insertWikiFileIntoEditor (jsInsertWikiFile) on click.
func RenderWikiFileAutocompleteList(results []WikiFileResult) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	if len(results) == 0 {
		return fmt.Sprintf(`<p class="no-media">%s</p>`, t("no files found"))
	}

	var htmlBuilder strings.Builder
	htmlBuilder.WriteString(`<div class="wiki-file-select-list">`)
	for _, f := range results {
		htmlBuilder.WriteString(`<div class="wiki-file-select-item" onclick="insertWikiFileIntoEditor(this)">`)
		fmt.Fprintf(&htmlBuilder, `<input type="hidden" class="wiki-file-path" value="%s">`, f.Path)
		fmt.Fprintf(&htmlBuilder, `<input type="hidden" class="wiki-file-filename" value="%s">`, f.Filename)
		fmt.Fprintf(&htmlBuilder, `<span class="wiki-file-select-name">%s</span>`, f.Path)
		htmlBuilder.WriteString(`</div>`)
	}
	htmlBuilder.WriteString(`</div>`)
	return htmlBuilder.String()
}

// jsRegisterEditor stores the editor instance on window so media helpers can reach it.
func jsRegisterEditor() string {
	return `
		// expose editor instance globally for media selector callbacks
		window.currentEditor = editor;`
}

// jsPreventEmptyUndo blocks Ctrl+Z from undoing past the initially loaded content.
// ToastUI 3.x calls setMarkdown(initialValue) after creating the editor with an empty CM6
// state, so the empty document becomes the undo baseline. We intercept keydown in the
// capture phase (before CM6 sees it) and suppress the undo when the content is already
// at the initial loaded state.
func jsPreventEmptyUndo() string {
	return `
		var initialMarkdown = editor.getMarkdown();
		document.querySelector('#toastui-editor').addEventListener('keydown', function(e) {
			if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
				if (editor.getMarkdown() === initialMarkdown) {
					e.stopPropagation();
					e.preventDefault();
				}
			}
		}, true);`
}

// getToastUIEditorScript assembles all JS helpers into a single <script> block.
func getToastUIEditorScript(content, frontMatter, filePath string) string {
	parts := []string{
		jsEditorInit(content),
		jsPreventEmptyUndo(),
		jsFileInputAcceptAll(),
		jsDragAndDrop(),
		jsUploadMediaBlob(),
		jsMediaSelector(),
		jsInsertMedia(),
		jsWikiFileSelector(),
		jsInsertWikiFile(),
		jsRegisterEditor(),
		fmt.Sprintf(`initWikiAutocompleteToastUI(editor, {cursorEnd: %t, currentFile: %s});`, configmanager.WikiLinkCursorEnd.Get(), jsEscapeString(filePath)),
		jsFormSubmit(frontMatter),
	}

	return "<script>\n(function() {" +
		strings.Join(parts, "\n") +
		"\n})();\n</script>"
}

// RenderToastUIEditorForm renders a ToastUI editor form for file creation/editing.
// Strips YAML front matter before passing content to the editor and re-prepends on save.
// prefillPath pre-populates the file path input for new files (ignored when editing).
func RenderToastUIEditorForm(filePath, prefillPath string, editor ...string) string {
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
				<div id="toastui-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				<div id="editor-status"></div>
			</div>
		</form>
		%s
		%s
		%s`,
		action,
		filepathInput,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		renderMediaSelectorModalHTML(),
		renderWikiFileSelectorModalHTML(),
		getToastUIEditorScript(content, frontMatter, filePath))
}

// RenderToastUISectionEditorForm renders a ToastUI editor form for editing a single section.
func RenderToastUISectionEditorForm(filePath, sectionID string) string {
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
				<div id="toastui-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
				<input type="hidden" name="filepath" value="%s" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
				<div id="editor-status"></div>
			</div>
		</form>
		%s
		%s
		%s`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		renderMediaSelectorModalHTML(),
		renderWikiFileSelectorModalHTML(),
		getToastUIEditorScript(content, "", filePath))
}
