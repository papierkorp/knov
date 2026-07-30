// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
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

// jsUploadMediaBlob defines the shared upload helper used by editor drag-and-drop/paste hooks.
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

// WikiFileResult is a single autocomplete match, shared between the JSON response and the
// server-rendered wiki-file autocomplete list (see handleAPIFilesAutocomplete in the server
// package).
type WikiFileResult struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

// RenderWikiFileAutocompleteList renders file search results as HTML for the wiki-link
// autocomplete (see static/wiki-autocomplete.js).
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
