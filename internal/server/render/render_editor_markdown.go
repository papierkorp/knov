// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// getToastUIEditorScript returns the common ToastUI editor JavaScript initialization
func getToastUIEditorScript(content string) string {
	return fmt.Sprintf(`
		<script>
			(function() {
				const editor = new toastui.Editor({
					el: document.querySelector('#markdown-editor'),
					height: '500px',
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
						'Insert Image': 'Insert File',
						'Insert image': 'Insert file',
						'image': 'file',
						'Image': 'File',
						'Choose a file': 'Choose a file',
						'No file selected': 'No file selected'
					},
					hooks: {
						addImageBlobHook: function(blob, callback, source) {
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
								return false;
							}

							const formData = new FormData();
							formData.append('file', blob);
							formData.append('context_path', contextPath);

							const uploadMessage = document.createElement('div');
							const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';
							uploadMessage.className = 'upload-notification';
							uploadMessage.style.cssText = 'position: fixed; top: 10px; right: 10px; padding: 12px 16px; border-radius: 6px; z-index: 9999; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.15);';
							uploadMessage.style.backgroundColor = isDarkMode ? '#374151' : '#0ea5e9';
							uploadMessage.style.color = isDarkMode ? '#f9fafb' : '#ffffff';
							uploadMessage.textContent = 'uploading image...';
							document.body.appendChild(uploadMessage);

							fetch('/api/media/upload', {
								method: 'POST',
								body: formData,
								headers: { 'Accept': 'application/json' }
							})
							.then(response => {
								if (!response.ok) {
									return response.text().then(errorText => {
										throw new Error(errorText || 'upload failed: ' + response.statusText);
									});
								}
								return response.json();
							})
							.then(data => {
								if (document.body.contains(uploadMessage)) {
									document.body.removeChild(uploadMessage);
								}
								const filePath = 'media/' + data.path;
								callback(filePath, data.filename || 'uploaded file');
							})
							.catch(error => {
								if (document.body.contains(uploadMessage)) {
									document.body.removeChild(uploadMessage);
								}
								alert('failed to upload file: ' + error.message);
								callback('', '');
							});

							return false;
						}
					}
				});

				window.showMediaSelector = function(editor) {
					const modal = document.createElement('div');
					modal.className = 'media-selector-modal';
					modal.style.cssText = 'position: fixed; top: 0; left: 0; width: 100%%; height: 100%%; background: rgba(0,0,0,0.5); z-index: 10000; display: flex; align-items: center; justify-content: center;';

					const popup = document.createElement('div');
					popup.className = 'media-selector-popup';
					popup.style.cssText = 'background: white; border-radius: 8px; width: 600px; max-height: 500px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.3);';

					const isDarkMode = document.body.getAttribute('data-dark-mode') === 'true';
					if (isDarkMode) {
						popup.style.backgroundColor = '#374151';
						popup.style.color = '#f9fafb';
					}

					const header = document.createElement('div');
					header.style.cssText = 'padding: 16px; border-bottom: 1px solid #eee; display: flex; justify-content: space-between; align-items: center;';
					if (isDarkMode) {
						header.style.borderBottomColor = '#4b5563';
					}
					header.innerHTML = '<h3 style="margin: 0;">select media file</h3><button onclick="closeMediaSelector()" style="background: none; border: none; font-size: 20px; cursor: pointer;">&times;</button>';

					const content = document.createElement('div');
					content.style.cssText = 'padding: 16px; max-height: 400px; overflow-y: auto;';
					content.innerHTML = 'loading media files...';

					popup.appendChild(header);
					popup.appendChild(content);
					modal.appendChild(popup);
					document.body.appendChild(modal);

					fetch('/api/media/list?mode=select', {
						headers: { 'Accept': 'text/html' }
					})
					.then(response => response.text())
					.then(html => {
						content.innerHTML = html;
					})
					.catch(error => {
						content.innerHTML = 'error loading media files';
					});
				};

				window.insertMediaLink = function(mediaURL, filename) {
					const editor = window.currentEditor;
					if (editor) {
						const markdownLink = '![' + filename + '](' + mediaURL + ')';
						editor.insertText(markdownLink);
						const hiddenField = document.getElementById('editor-content');
						if (hiddenField) {
						    hiddenField.value = editor.getMarkdown();
						}
						const form = document.querySelector('.file-form');
						if (form) {
							htmx.trigger(form, 'submit');
						}
					}
					closeMediaSelector();
				};

				window.insertMediaIntoEditor = function(element) {
					const mediaPath = element.querySelector('.media-path').value;
					const filename = element.querySelector('.media-filename').value;
					const mediaURL = 'media/' + mediaPath;

					const editor = window.currentEditor;
					if (editor) {
						const markdownLink = '![' + filename + '](' + mediaURL + ')';
						editor.insertText(markdownLink);
						const hiddenField = document.getElementById('editor-content');
						if (hiddenField) {
						    hiddenField.value = editor.getMarkdown();
						}
						const form = document.querySelector('.file-form');
						if (form) {
							htmx.trigger(form, 'submit');
						}
					}
					closeMediaSelector();
				};

				window.closeMediaSelector = function() {
					const modal = document.querySelector('.media-selector-modal');
					if (modal) {
						modal.remove();
					}
				};

				window.currentEditor = editor;

				document.querySelector('.file-form').addEventListener('submit', function(e) {
					document.getElementById('editor-content').value = editor.getMarkdown();
				});
			})();
		</script>`, jsEscapeString(content))
}

// RenderMarkdownEditorForm renders a markdown editor form for file creation/editing
func RenderMarkdownEditorForm(filePath string, filetype ...string) string {
	content := ""
	isEdit := filePath != ""

	if isEdit {
		fullPath := pathutils.ToDocsPath(filePath)
		rawContent, err := contentStorage.ReadFile(fullPath)
		if err == nil {
			content = string(rawContent)
		}
	}

	action := "/api/files/save"
	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filePath)
	}

	var currentFiletype string
	if len(filetype) > 0 {
		currentFiletype = filetype[0]
	}

	filepathInput := ""
	if !isEdit {
		if currentFiletype == "fleeting" {
			filepathInput = fmt.Sprintf(`<input type="hidden" name="filetype" value="%s" />`, currentFiletype)
		} else {
			filepathInput = fmt.Sprintf(`
				<div class="form-group">
					<label for="filepath-input">%s</label>
					<input type="text" id="filepath-input" name="filepath" required placeholder="%s" class="form-input" list="folder-suggestions" />
					<datalist id="folder-suggestions" hx-get="/api/files/folder-suggestions" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
				</div>`,
				translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
				translation.SprintfForRequest(configmanager.GetLanguage(), "my-file.md"))

			if currentFiletype != "" {
				filepathInput += fmt.Sprintf(`<input type="hidden" name="filetype" value="%s" />`, currentFiletype)
			}
		}
	} else {
		filepathInput = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, filePath)
	}

	saveButtonText := translation.SprintfForRequest(configmanager.GetLanguage(), "save file")
	if currentFiletype == "fleeting" && !isEdit {
		saveButtonText = translation.SprintfForRequest(configmanager.GetLanguage(), "save note")
	}

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" class="file-form">
			%s
			<div class="form-group">
				<div id="markdown-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
			</div>
			<div id="editor-status"></div>
		</form>
		%s`, action, filepathInput, saveButtonText, cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		getToastUIEditorScript(content))
}

// RenderMarkdownSectionEditorForm renders a markdown editor form for editing a specific section
func RenderMarkdownSectionEditorForm(filePath, sectionID string) string {
	content := ""

	if filePath != "" && sectionID != "" {
		handler := contentHandler.GetHandler("markdown")
		includeSubheaders := configmanager.GetSectionEditIncludeSubheaders()
		sectionContent, err := handler.ExtractSection(filePath, sectionID, includeSubheaders)
		if err == nil {
			content = sectionContent
		}
	}

	action := "/api/files/section/save"
	cancelURL := fmt.Sprintf("/files/%s", filePath)

	return fmt.Sprintf(`
		<form hx-post="%s" hx-target="#editor-status" class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="sectionid" value="%s" readonly />
			</div>
			<div class="form-group">
				<div id="markdown-editor"></div>
				<input type="hidden" name="content" id="editor-content" />
				<input type="hidden" name="filepath" value="%s" />
			</div>
			<div class="form-actions">
				<button type="submit" class="btn-primary">%s</button>
				<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
			</div>
			<div id="editor-status"></div>
		</form>
		%s`, action,
		translation.SprintfForRequest(configmanager.GetLanguage(), "section"),
		sectionID,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save section"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		getToastUIEditorScript(content))
}

// RenderTextareaEditorComponent renders a textarea editor component with save/cancel buttons
func RenderTextareaEditorComponent(filepath, content string) string {
	cancelURL := "/"
	if filepath != "" {
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	// check if file is DokuWiki to show convert button
	var convertButton string
	if filepath != "" {
		fullPath := pathutils.ToDocsPath(filepath)
		handler := parser.GetParserRegistry().GetHandler(fullPath)
		if handler != nil && handler.Name() == "dokuwiki" {
			convertButton = fmt.Sprintf(`
				<button type="button"
						hx-post="/api/files/convert-to-markdown"
						hx-vals='{"filepath": "%s"}'
						hx-target="#editor-status"
						class="btn-secondary">
					%s
				</button>`,
				filepath,
				translation.SprintfForRequest(configmanager.GetLanguage(), "convert to markdown"))
		}
	}

	return fmt.Sprintf(`
		<div id="component-textarea-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">%s</button>
					<button type="button" onclick="location.href='%s'" class="btn-secondary">%s</button>
					%s
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		cancelURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		convertButton)
}

// jsEscapeString escapes a string for safe use in JavaScript
func jsEscapeString(s string) string {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		logging.LogError("failed to marshal string for javascript: %v", err)
		return `""`
	}
	return string(jsonBytes)
}
