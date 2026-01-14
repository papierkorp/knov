// Package render - HTMX HTML rendering functions for media components
package render

import (
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/translation"
)

// RenderMediaUploadComponent renders a media upload component
func RenderMediaUploadComponent(contextPath string, allowedTypes []string) string {
	allowedTypesStr := ""
	if len(allowedTypes) > 0 {
		for i, mimeType := range allowedTypes {
			if i > 0 {
				allowedTypesStr += ", "
			}
			allowedTypesStr += mimeType
		}
	}

	return fmt.Sprintf(`
		<div id="component-media-upload" class="media-upload-component">
			<form hx-post="/api/media/upload" hx-encoding="multipart/form-data" hx-target="#upload-status">
				<div class="form-group">
					<label for="media-file">%s:</label>
					<input type="file" name="file" id="media-file" accept="%s" required class="form-input">
					<input type="hidden" name="context_path" value="%s">
				</div>
				<div class="form-actions">
					<button type="submit" class="btn-primary">%s</button>
				</div>
			</form>
			<div id="upload-status"></div>
		</div>
	`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "select file"),
		allowedTypesStr,
		contextPath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "upload"))
}

// RenderMediaPreview renders a preview of a media file
func RenderMediaPreview(mediaPath, contentType string) string {
	switch {
	case contentType == "":
		return fmt.Sprintf(`<div class="media-preview">%s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "unknown file type"))
	case contentType[:6] == "image/":
		return fmt.Sprintf(`<div class="media-preview"><img src="/media/%s" alt="media preview" style="max-width: 300px; max-height: 300px;"></div>`, mediaPath)
	case contentType[:6] == "video/":
		return fmt.Sprintf(`<div class="media-preview"><video controls style="max-width: 300px; max-height: 300px;"><source src="/media/%s" type="%s"></video></div>`, mediaPath, contentType)
	case contentType == "application/pdf":
		return fmt.Sprintf(`<div class="media-preview"><iframe src="/media/%s" style="width: 300px; height: 400px;"></iframe></div>`, mediaPath)
	case contentType[:5] == "text/":
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="/media/%s" target="_blank">%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "text file"),
			mediaPath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "view"))
	default:
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="/media/%s" download>%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "file"),
			mediaPath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
	}
}
