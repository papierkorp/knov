// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"encoding/json"
	"fmt"
	htmlpkg "html"
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

// AutocompleteItem is a single suggestion in the shared autocomplete dropdown
// (files, media, headers, folder paths). Value is what gets inserted, Label is
// the primary display text, Detail the secondary one.
type AutocompleteItem struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

// RenderAutocompleteList renders suggestions as the shared dropdown list partial
// consumed by static/wiki-autocomplete.js. Empty input renders nothing (the
// dropdown hides itself).
func RenderAutocompleteList(items []AutocompleteItem) string {
	if len(items) == 0 {
		return ""
	}
	var htmlBuilder strings.Builder
	htmlBuilder.WriteString(`<ul class="autocomplete-list">`)
	for _, item := range items {
		fmt.Fprintf(&htmlBuilder,
			`<li class="autocomplete-item" data-value="%s"><span class="autocomplete-item-label">%s</span><span class="autocomplete-item-detail">%s</span></li>`,
			htmlpkg.EscapeString(item.Value), htmlpkg.EscapeString(item.Label), htmlpkg.EscapeString(item.Detail))
	}
	htmlBuilder.WriteString(`</ul>`)
	return htmlBuilder.String()
}
