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
		logging.LogError("failed to marshal string for javascript: %v", err)
		return `""`
	}
	return string(jsonBytes)
}

// jsEditorInit returns the ToastUI editor constructor call.
// Binds the upload hook so blob uploads go through uploadMediaBlob.
func jsEditorInit(content string) string {
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
			initialEditType: 'markdown',
			previewStyle: 'tab',
			initialValue: %s,
			theme: document.body.getAttribute('data-dark-mode') === 'true' ? 'dark' : 'default',
			language: 'en-US',
			toolbarItems: [
				['heading', 'bold', 'italic', 'strike'],
				['hr', 'quote'],
				['ul', 'ol', 'task', 'indent', 'outdent'],
				['table', 'image', 'link'],
				[{
					name: 'selectMedia',
					tooltip: 'Select Media',
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
					tooltip: 'Insert Wiki File Link',
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
			],
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
		});`, jsEscapeString(content))
}

// jsFileInputAcceptAll patches the built-in image popup file input to accept all file types.
// ToastUI sets accept="image/*" by default. We override it on init and via MutationObserver
// for the lazy-rendered popup.
func jsFileInputAcceptAll() string {
	return `
		// patch built-in image popup file input to accept all file types
		// also rename the image button tooltip to "Insert Media"
		setTimeout(function() {
			document.querySelectorAll('.toastui-editor-popup input[type="file"]').forEach(function(input) {
				input.setAttribute('accept', '*');
			});

		}, 500);

		// also patch lazily-rendered popups via MutationObserver
		const popupObserver = new MutationObserver(function(mutations) {
			mutations.forEach(function(mutation) {
				mutation.addedNodes.forEach(function(node) {
					if (node.nodeType !== 1) return;
					const fileInput = node.querySelector && node.querySelector('input[type="file"]');
					if (fileInput) fileInput.setAttribute('accept', '*');
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
		const editorEl = document.querySelector('#toastui-editor');
		editorEl.addEventListener('dragover', function(e) {
			e.preventDefault();
			e.dataTransfer.dropEffect = 'copy';
		});
		editorEl.addEventListener('drop', function(e) {
			e.preventDefault();
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
		});`
}

// jsUploadMediaBlob defines the shared upload helper used by the blob hook and drag-and-drop.
// Derives the context path from the current URL, shows an upload notification, then POSTs
// to /api/media/upload and calls callback(url, alt) on success.
func jsUploadMediaBlob() string {
	return `
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
				alert('please save the document first to enable file uploads.');
				callback('', '');
				return;
			}

			const formData = new FormData();
			formData.append('file', blob);
			formData.append('context_path', contextPath);

			const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';
			const uploadMessage = document.createElement('div');
			uploadMessage.className = 'upload-notification';
			uploadMessage.style.cssText = 'position:fixed;top:10px;right:10px;padding:12px 16px;border-radius:6px;z-index:9999;font-weight:500;box-shadow:0 4px 12px rgba(0,0,0,0.15);';
			uploadMessage.style.backgroundColor = isDarkMode ? '#374151' : '#0ea5e9';
			uploadMessage.style.color = isDarkMode ? '#f9fafb' : '#ffffff';
			uploadMessage.textContent = 'uploading...';
			document.body.appendChild(uploadMessage);

			fetch('/api/media/upload', {
				method: 'POST',
				body: formData,
				headers: { 'Accept': 'application/json' }
			})
			.then(function(response) {
				if (!response.ok) {
					return response.text().then(function(t) {
						throw new Error(t || 'upload failed: ' + response.statusText);
					});
				}
				return response.json();
			})
			.then(function(data) {
				if (document.body.contains(uploadMessage)) document.body.removeChild(uploadMessage);
				callback('media/' + data.path, data.filename || blob.name || 'uploaded file');
			})
			.catch(function(error) {
				if (document.body.contains(uploadMessage)) document.body.removeChild(uploadMessage);
				alert('failed to upload file: ' + error.message);
				callback('', '');
			});
		}`
}

// jsMediaSelector defines the media browser modal (showMediaSelector / closeMediaSelector).
// Opens a modal that fetches /api/media/list?mode=select as HTML.
func jsMediaSelector() string {
	return `
		// media browser modal — opened by the toolbar "Select Media" button
		window.showMediaSelector = function(editor) {
			const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';

			const modal = document.createElement('div');
			modal.className = 'media-selector-modal';
			modal.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:10000;display:flex;align-items:center;justify-content:center;';

			const popup = document.createElement('div');
			popup.className = 'media-selector-popup';
			popup.style.cssText = 'background:white;border-radius:8px;width:600px;max-height:560px;overflow:hidden;box-shadow:0 4px 12px rgba(0,0,0,0.3);display:flex;flex-direction:column;';
			if (isDarkMode) {
				popup.style.backgroundColor = '#374151';
				popup.style.color = '#f9fafb';
			}

			const header = document.createElement('div');
			header.style.cssText = 'padding:12px 16px;border-bottom:1px solid ' + (isDarkMode ? '#4b5563' : '#eee') + ';display:flex;justify-content:space-between;align-items:center;flex-shrink:0;';
			header.innerHTML = '<h3 style="margin:0;font-size:1em;">select media file</h3><button onclick="closeMediaSelector()" style="background:none;border:none;font-size:20px;cursor:pointer;">&times;</button>';

			const search = document.createElement('div');
			search.style.cssText = 'padding:8px 16px;border-bottom:1px solid ' + (isDarkMode ? '#4b5563' : '#eee') + ';flex-shrink:0;';
			search.innerHTML = '<input type="text" placeholder="filter..." style="width:100%;padding:6px 10px;border:1px solid ' + (isDarkMode ? '#4b5563' : '#d1d5db') + ';border-radius:4px;background:' + (isDarkMode ? '#1f2937' : '#fff') + ';color:' + (isDarkMode ? '#f9fafb' : '#111') + ';font-size:0.9em;box-sizing:border-box;" oninput="filterMediaSelectorList(this.value)">';

			const body = document.createElement('div');
			body.style.cssText = 'padding:12px 16px;overflow-y:auto;flex:1;';
			body.innerHTML = 'loading media files...';

			popup.appendChild(header);
			popup.appendChild(search);
			popup.appendChild(body);
			modal.appendChild(popup);
			document.body.appendChild(modal);

			fetch('/api/media/list?mode=select', { headers: { 'Accept': 'text/html' } })
				.then(function(r) { return r.text(); })
				.then(function(html) { body.innerHTML = html; })
				.catch(function() { body.innerHTML = 'error loading media files'; });

			// focus search after items load
			setTimeout(function() {
				const input = modal.querySelector('input[type="text"]');
				if (input) input.focus();
			}, 150);
		};

		window.closeMediaSelector = function() {
			const modal = document.querySelector('.media-selector-modal');
			if (modal) modal.remove();
		};

		window.filterMediaSelectorList = function(query) {
			const q = query.toLowerCase();
			document.querySelectorAll('.media-selector-modal .media-select-item').forEach(function(item) {
				const name = (item.querySelector('.media-select-name') || {}).textContent || '';
				item.style.display = name.toLowerCase().includes(q) ? '' : 'none';
			});
		};`
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

// jsWikiFileSelector defines a modal for browsing wiki files and inserting a full markdown link.
// Uses the /api/files/autocomplete endpoint as datasource.
func jsWikiFileSelector() string {
	return `
		window.showWikiFileSelector = function(editor) {
			const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';

			const modal = document.createElement('div');
			modal.className = 'wiki-file-selector-modal';
			modal.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:10000;display:flex;align-items:center;justify-content:center;';

			const popup = document.createElement('div');
			popup.style.cssText = 'background:white;border-radius:8px;width:600px;max-height:560px;overflow:hidden;box-shadow:0 4px 12px rgba(0,0,0,0.3);display:flex;flex-direction:column;';
			if (isDarkMode) {
				popup.style.backgroundColor = '#374151';
				popup.style.color = '#f9fafb';
			}

			const header = document.createElement('div');
			header.style.cssText = 'padding:12px 16px;border-bottom:1px solid ' + (isDarkMode ? '#4b5563' : '#eee') + ';display:flex;justify-content:space-between;align-items:center;flex-shrink:0;';
			header.innerHTML = '<h3 style="margin:0;font-size:1em;">insert wiki file link</h3><button onclick="closeWikiFileSelector()" style="background:none;border:none;font-size:20px;cursor:pointer;">&times;</button>';

			const search = document.createElement('div');
			search.style.cssText = 'padding:8px 16px;border-bottom:1px solid ' + (isDarkMode ? '#4b5563' : '#eee') + ';flex-shrink:0;';
			const searchInput = document.createElement('input');
			searchInput.type = 'text';
			searchInput.placeholder = 'filter...';
			searchInput.style.cssText = 'width:100%;padding:6px 10px;border:1px solid ' + (isDarkMode ? '#4b5563' : '#d1d5db') + ';border-radius:4px;background:' + (isDarkMode ? '#1f2937' : '#fff') + ';color:' + (isDarkMode ? '#f9fafb' : '#111') + ';font-size:0.9em;box-sizing:border-box;';
			search.appendChild(searchInput);

			const body = document.createElement('div');
			body.style.cssText = 'padding:12px 16px;overflow-y:auto;flex:1;';

			popup.appendChild(header);
			popup.appendChild(search);
			popup.appendChild(body);
			modal.appendChild(popup);
			document.body.appendChild(modal);

			var debounceTimer;
			function fetchFiles(q) {
				clearTimeout(debounceTimer);
				debounceTimer = setTimeout(function() {
					fetch('/api/files/autocomplete?q=' + encodeURIComponent(q))
						.then(function(r) { return r.json(); })
						.then(function(results) {
							body.innerHTML = '';
							if (!results || results.length === 0) {
								body.innerHTML = '<span style="color:' + (isDarkMode ? '#9ca3af' : '#888') + ';font-size:0.9em;">no files found</span>';
								return;
							}
							results.forEach(function(f) {
								const item = document.createElement('div');
								item.style.cssText = 'padding:6px 8px;cursor:pointer;border-radius:4px;font-size:0.9em;';
								item.textContent = f.path;
								item.addEventListener('mouseenter', function() { item.style.background = isDarkMode ? '#4b5563' : '#f3f4f6'; });
								item.addEventListener('mouseleave', function() { item.style.background = ''; });
								item.addEventListener('click', function() {
									const label = f.filename.replace(/\.[^.]+$/, '');
									editor.insertText('[' + label + '](/files/' + f.path + ')');
									closeWikiFileSelector();
								});
								body.appendChild(item);
							});
						})
						.catch(function() { body.innerHTML = 'error loading files'; });
				}, 120);
			}

			searchInput.addEventListener('input', function() { fetchFiles(this.value); });
			fetchFiles('');
			setTimeout(function() { searchInput.focus(); }, 50);
		};

		window.closeWikiFileSelector = function() {
			const modal = document.querySelector('.wiki-file-selector-modal');
			if (modal) modal.remove();
		};`
}

// jsRegisterEditor stores the editor instance on window so media helpers can reach it.
func jsRegisterEditor() string {
	return `
		// expose editor instance globally for media selector callbacks
		window.currentEditor = editor;`
}

// getToastUIEditorScript assembles all JS helpers into a single <script> block.
func getToastUIEditorScript(content, frontMatter string) string {
	parts := []string{
		jsEditorInit(content),
		jsFileInputAcceptAll(),
		jsDragAndDrop(),
		jsUploadMediaBlob(),
		jsMediaSelector(),
		jsInsertMedia(),
		jsWikiFileSelector(),
		jsRegisterEditor(),
		`initWikiAutocomplete(editor);`,
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
					<input type="text" id="filepath-input" name="filepath" required value="%s" placeholder="%s" class="form-input" list="folder-suggestions" />
					<datalist id="folder-suggestions" hx-get="/api/files/folder-suggestions" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
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
			</div>
			<div id="editor-status"></div>
		</form>
		%s`,
		action,
		filepathInput,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		getToastUIEditorScript(content, frontMatter))
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
		getToastUIEditorScript(content, ""))
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
				<input type="text" id="filepath-input" name="filepath" required placeholder="%s" class="form-input" list="folder-suggestions" />
				<datalist id="folder-suggestions" hx-get="/api/files/folder-suggestions" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
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
		</div>`,
		filepathField,
		content,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		convertButton)
}
