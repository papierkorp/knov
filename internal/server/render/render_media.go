// Package render - HTMX HTML rendering functions for media components
package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
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
	// ensure media path is relative (remove media/ prefix if present)
	relativePath := strings.TrimPrefix(mediaPath, "media/")

	switch {
	case contentType == "":
		return fmt.Sprintf(`<div class="media-preview">%s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "unknown file type"))
	case contentType[:6] == "image/":
		return fmt.Sprintf(`<div class="media-preview"><img src="/media/%s" alt="media preview" style="max-width: 300px; max-height: 300px;"></div>`, relativePath)
	case contentType[:6] == "video/":
		return fmt.Sprintf(`<div class="media-preview"><video controls style="max-width: 300px; max-height: 300px;"><source src="/media/%s" type="%s"></video></div>`, relativePath, contentType)
	case contentType == "application/pdf":
		return fmt.Sprintf(`<div class="media-preview"><iframe src="/media/%s" style="width: 300px; height: 400px;"></iframe></div>`, relativePath)
	case contentType[:5] == "text/":
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="/media/%s" target="_blank">%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "text file"),
			relativePath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "view"))
	default:
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="/media/%s" download>%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "file"),
			relativePath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
	}
}

// RenderMediaList renders a grid of media files with previews
func RenderMediaList(mediaFiles []files.File) string {
	if len(mediaFiles) == 0 {
		return `<div id="component-no-media" class="component-no-media">` +
			translation.SprintfForRequest(configmanager.GetLanguage(), "no media files found") +
			`</div>`
	}

	var html strings.Builder
	html.WriteString(`<div id="component-media-grid" class="media-grid">`)

	for _, file := range mediaFiles {
		// ensure media path is relative (remove media/ prefix)
		relativePath := strings.TrimPrefix(file.Path, "media/")
		fileExt := strings.ToLower(filepath.Ext(relativePath))
		filename := filepath.Base(relativePath)

		html.WriteString(`<div class="media-item">`)

		// media preview/thumbnail
		html.WriteString(`<div class="media-preview">`)
		if files.IsImageFile(fileExt) {
			html.WriteString(fmt.Sprintf(`<a href="/media/%s" target="_blank">
				<img src="/media/%s" alt="%s" loading="lazy" class="media-thumbnail">
			</a>`, relativePath, relativePath, filename))
		} else if files.IsVideoFile(fileExt) {
			html.WriteString(fmt.Sprintf(`<div class="media-video-preview">
				<video preload="none" class="media-thumbnail" poster="">
					<source src="/media/%s" type="video/%s">
				</video>
				<div class="video-overlay">
					<i class="fas fa-play"></i>
				</div>
			</div>`, relativePath, strings.TrimPrefix(fileExt, ".")))
		} else {
			icon := files.GetFileTypeIcon(fileExt)
			html.WriteString(fmt.Sprintf(`<div class="media-icon">
				<i class="fas %s"></i>
			</div>`, icon))
		}
		html.WriteString(`</div>`)

		// media info
		html.WriteString(`<div class="media-info">`)
		html.WriteString(fmt.Sprintf(`<div class="media-filename" title="%s">%s</div>`, filename, filename))

		// show file size if available in metadata
		if file.Metadata != nil && file.Metadata.Size > 0 {
			sizeStr := formatFileSize(file.Metadata.Size)
			html.WriteString(fmt.Sprintf(`<div class="media-filesize">%s</div>`, sizeStr))
		}
		html.WriteString(`</div>`)

		// media actions
		html.WriteString(`<div class="media-actions">`)
		html.WriteString(fmt.Sprintf(`<a href="/media/%s" class="btn btn-sm btn-primary" target="_blank">
			<i class="fas fa-eye"></i> %s
		</a>`, relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "view")))
		html.WriteString(fmt.Sprintf(`<a href="/media/%s" download class="btn btn-sm btn-secondary">
			<i class="fas fa-download"></i> %s
		</a>`, relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "download")))
		html.WriteString(fmt.Sprintf(`<button type="button" class="btn btn-sm btn-danger"
			hx-delete="/api/media/%s"
			hx-confirm="%s"
			hx-target="#component-media-grid"
			hx-trigger="click">
			<i class="fas fa-trash"></i> %s
		</button>`, relativePath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "delete")))
		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // close media-item
	}

	html.WriteString(`</div>`) // close media-grid
	return html.String()
}

// RenderMediaDetail renders detailed view of a media file with metadata
func RenderMediaDetail(metadata *files.Metadata) string {
	if metadata == nil {
		return `<div id="component-error" class="component-error">` +
			translation.SprintfForRequest(configmanager.GetLanguage(), "media file not found") +
			`</div>`
	}

	relativePath := strings.TrimPrefix(metadata.Path, "media/")
	fileExt := strings.ToLower(filepath.Ext(relativePath))
	filename := filepath.Base(relativePath)

	var html strings.Builder
	html.WriteString(`<div id="component-media-detail" class="media-detail">`)

	// media preview section
	html.WriteString(`<div class="media-preview-large">`)
	if files.IsImageFile(fileExt) {
		html.WriteString(fmt.Sprintf(`<img src="/media/%s" alt="%s" class="media-preview-image">`,
			relativePath, filename))
	} else if files.IsVideoFile(fileExt) {
		html.WriteString(fmt.Sprintf(`<video controls class="media-preview-video">
			<source src="/media/%s" type="video/%s">
			`+translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support video playback")+`
		</video>`, relativePath, strings.TrimPrefix(fileExt, ".")))
	} else if files.IsAudioFile(fileExt) {
		html.WriteString(fmt.Sprintf(`<audio controls class="media-preview-audio">
			<source src="/media/%s" type="audio/%s">
			`+translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support audio playback")+`
		</audio>`, relativePath, strings.TrimPrefix(fileExt, ".")))
	} else {
		icon := files.GetFileTypeIcon(fileExt)
		html.WriteString(fmt.Sprintf(`<div class="media-preview-icon">
			<i class="fas %s"></i>
			<p>%s</p>
		</div>`, icon, filename))
	}
	html.WriteString(`</div>`)

	// metadata section
	html.WriteString(`<div class="media-metadata">`)
	html.WriteString(fmt.Sprintf(`<h2>%s</h2>`, filename))

	html.WriteString(`<dl class="media-info">`)
	html.WriteString(fmt.Sprintf(`<dt>%s</dt><dd>%s</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path"), relativePath))
	html.WriteString(fmt.Sprintf(`<dt>%s</dt><dd>%s</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "type"), metadata.FileType))

	if metadata.Size > 0 {
		html.WriteString(fmt.Sprintf(`<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "size"), formatFileSize(metadata.Size)))
	}

	if !metadata.CreatedAt.IsZero() {
		html.WriteString(fmt.Sprintf(`<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "created"),
			metadata.CreatedAt.Format("2006-01-02 15:04")))
	}

	if !metadata.LastEdited.IsZero() {
		html.WriteString(fmt.Sprintf(`<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "last modified"),
			metadata.LastEdited.Format("2006-01-02 15:04")))
	}

	html.WriteString(`</dl>`)

	// actions
	html.WriteString(`<div class="media-actions">`)
	html.WriteString(fmt.Sprintf(`<a href="/media/%s" download class="btn btn-primary">
		<i class="fas fa-download"></i> %s
	</a>`, relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "download")))
	html.WriteString(fmt.Sprintf(`<button type="button" class="btn btn-danger"
		hx-delete="/api/media/%s"
		hx-confirm="%s"
		hx-target="#component-media-detail"
		hx-trigger="click">
		<i class="fas fa-trash"></i> %s
	</button>`, relativePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "delete")))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`) // close media-metadata
	html.WriteString(`</div>`) // close media-detail

	return html.String()
}

// formatFileSize formats file size in human readable format
func formatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
}
