// Package render - HTMX HTML rendering functions for media components
package render

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
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

// RenderMediaList renders a grid of media files with previews and filter controls
func RenderMediaList(mediaFiles []files.File, filter string, totalCount, orphanedCount int) string {
	var html strings.Builder

	// wrapper for htmx target
	html.WriteString(`<div id="component-media-content">`)

	// filter controls
	html.WriteString(`<div id="component-media-filter" class="media-filter">`)
	fmt.Fprintf(&html, `<div class="filter-label">%s:</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "show"))
	html.WriteString(`<div class="filter-buttons">`)

	// all button
	activeAll := ""
	if filter == "all" {
		activeAll = " active"
	}
	fmt.Fprintf(&html, `<button class="filter-btn%s" hx-get="/api/media/list?filter=all" hx-target="#component-media-content" hx-swap="innerHTML">%s (%d)</button>`,
		activeAll,
		translation.SprintfForRequest(configmanager.GetLanguage(), "all"),
		totalCount)

	// used button
	activeUsed := ""
	if filter == "used" {
		activeUsed = " active"
	}
	usedCount := totalCount - orphanedCount
	fmt.Fprintf(&html, `<button class="filter-btn%s" hx-get="/api/media/list?filter=used" hx-target="#component-media-content" hx-swap="innerHTML">%s (%d)</button>`,
		activeUsed,
		translation.SprintfForRequest(configmanager.GetLanguage(), "used"),
		usedCount)

	// orphaned button
	activeOrphaned := ""
	if filter == "orphaned" {
		activeOrphaned = " active"
	}
	fmt.Fprintf(&html, `<button class="filter-btn%s" hx-get="/api/media/list?filter=orphaned" hx-target="#component-media-content" hx-swap="innerHTML">%s (%d)</button>`,
		activeOrphaned,
		translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned"),
		orphanedCount)

	html.WriteString(`</div>`) // close filter-buttons
	html.WriteString(`</div>`) // close media-filter

	// empty state
	if len(mediaFiles) == 0 {
		var emptyMsg string
		switch filter {
		case "orphaned":
			emptyMsg = translation.SprintfForRequest(configmanager.GetLanguage(), "no orphaned media files")
		case "used":
			emptyMsg = translation.SprintfForRequest(configmanager.GetLanguage(), "no used media files")
		default:
			emptyMsg = translation.SprintfForRequest(configmanager.GetLanguage(), "no media files found")
		}
		fmt.Fprintf(&html, `<div id="component-no-media" class="component-no-media">%s</div>`, emptyMsg)
		html.WriteString(`</div>`) // close component-media-content
		return html.String()
	}

	// media grid
	html.WriteString(`<div id="component-media-grid" class="media-grid">`)

	// get orphaned media for badge display
	orphanedMedia, _ := files.GetOrphanedMediaFromCache()

	for _, file := range mediaFiles {
		// check if this media is orphaned
		isOrphaned := slices.Contains(orphanedMedia, file.Path)

		// ensure media path is relative (remove media/ prefix)
		relativePath := strings.TrimPrefix(file.Path, "media/")
		fileExt := strings.ToLower(filepath.Ext(relativePath))
		filename := filepath.Base(relativePath)

		orphanedClass := ""
		if isOrphaned {
			orphanedClass = " media-orphaned"
		}

		fmt.Fprintf(&html, `<div class="media-item%s">`, orphanedClass)

		// orphaned badge
		if isOrphaned {
			fmt.Fprintf(&html, `<div class="media-badge orphaned-badge" title="%s"><i class="fas fa-unlink"></i> %s</div>`,
				translation.SprintfForRequest(configmanager.GetLanguage(), "not used in any files"),
				translation.SprintfForRequest(configmanager.GetLanguage(), "unused"))
		}

		// media preview/thumbnail
		html.WriteString(`<div class="media-preview">`)
		if files.IsImageFile(fileExt) {
			fmt.Fprintf(&html, `<a href="/media/%s" target="_blank"><img src="/media/%s" alt="%s" loading="lazy" class="media-thumbnail"></a>`,
				relativePath, relativePath, filename)
		} else if files.IsVideoFile(fileExt) {
			fmt.Fprintf(&html, `<div class="media-video-preview"><video preload="none" class="media-thumbnail" poster=""><source src="/media/%s" type="video/%s"></video><div class="video-overlay"><i class="fas fa-play"></i></div></div>`,
				relativePath, strings.TrimPrefix(fileExt, "."))
		} else {
			icon := files.GetFileTypeIcon(fileExt)
			fmt.Fprintf(&html, `<div class="media-icon"><i class="fas %s"></i></div>`, icon)
		}
		html.WriteString(`</div>`)

		// media info
		html.WriteString(`<div class="media-info">`)
		fmt.Fprintf(&html, `<div class="media-filename" title="%s">%s</div>`, filename, filename)

		// show file size if available in metadata
		if file.Metadata != nil && file.Metadata.Size > 0 {
			sizeStr := formatFileSize(file.Metadata.Size)
			fmt.Fprintf(&html, `<div class="media-filesize">%s</div>`, sizeStr)
		}
		html.WriteString(`</div>`)

		// media actions
		html.WriteString(`<div class="media-actions">`)
		fmt.Fprintf(&html, `<a href="/media/%s?mode=detail" class="btn btn-sm btn-primary"><i class="fas fa-info-circle"></i> %s</a>`,
			relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "details"))
		fmt.Fprintf(&html, `<a href="/media/%s" download class="btn btn-sm btn-secondary"><i class="fas fa-download"></i> %s</a>`,
			relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
		fmt.Fprintf(&html, `<button type="button" class="btn btn-sm btn-danger" hx-delete="/api/media/%s" hx-confirm="%s" hx-target="#component-media-content" hx-trigger="click"><i class="fas fa-trash"></i> %s</button>`,
			relativePath,
			translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "delete"))
		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // close media-item
	}

	html.WriteString(`</div>`) // close media-grid
	html.WriteString(`</div>`) // close component-media-content
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
		fmt.Fprintf(&html, `<img src="/media/%s" alt="%s" class="media-preview-image">`,
			relativePath, filename)
	} else if files.IsVideoFile(fileExt) {
		fmt.Fprintf(&html, `<video controls class="media-preview-video">
			<source src="/media/%s" type="video/%s">
			%s
		</video>`, relativePath, strings.TrimPrefix(fileExt, "."),
			translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support video playback"))
	} else if files.IsAudioFile(fileExt) {
		fmt.Fprintf(&html, `<audio controls class="media-preview-audio">
			<source src="/media/%s" type="audio/%s">
			%s
		</audio>`, relativePath, strings.TrimPrefix(fileExt, "."),
			translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support audio playback"))
	} else {
		icon := files.GetFileTypeIcon(fileExt)
		fmt.Fprintf(&html, `<div class="media-preview-icon">
			<i class="fas %s"></i>
			<p>%s</p>
		</div>`, icon, filename)
	}
	html.WriteString(`</div>`)

	// metadata section
	html.WriteString(`<div class="media-metadata">`)
	fmt.Fprintf(&html, `<h2>%s</h2>`, filename)

	html.WriteString(`<dl class="media-info">`)
	fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path"), relativePath)
	fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "type"), metadata.FileType)

	if metadata.Size > 0 {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "size"), formatFileSize(metadata.Size))
	}

	if !metadata.CreatedAt.IsZero() {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "created"),
			metadata.CreatedAt.Format("2006-01-02 15:04"))
	}

	if !metadata.LastEdited.IsZero() {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "last modified"),
			metadata.LastEdited.Format("2006-01-02 15:04"))
	}

	html.WriteString(`</dl>`)

	// used in section
	html.WriteString(`<div class="media-used-in">`)
	fmt.Fprintf(&html, `<h3>%s</h3>`, translation.SprintfForRequest(configmanager.GetLanguage(), "used in"))

	if len(metadata.LinksToHere) == 0 {
		html.WriteString(`<p class="media-used-empty">`)
		fmt.Fprintf(&html, `%s`, translation.SprintfForRequest(configmanager.GetLanguage(), "not used in any files"))
		html.WriteString(`</p>`)
	} else {
		html.WriteString(`<ul class="media-used-list">`)
		for _, link := range metadata.LinksToHere {
			linkPath := pathutils.ToRelative(link)
			displayText := GetLinkDisplayText(linkPath)
			fmt.Fprintf(&html, `<li><a href="/files/%s" title="%s">%s</a></li>`, linkPath, linkPath, displayText)
		}
		html.WriteString(`</ul>`)
	}
	html.WriteString(`</div>`)

	// actions
	html.WriteString(`<div class="media-actions">`)
	fmt.Fprintf(&html, `<a href="/media/%s" download class="btn btn-primary">
		<i class="fas fa-download"></i> %s
	</a>`, relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
	fmt.Fprintf(&html, `<a href="/media/%s" target="_blank" class="btn btn-secondary">
		<i class="fas fa-external-link-alt"></i> %s
	</a>`, relativePath, translation.SprintfForRequest(configmanager.GetLanguage(), "open in new tab"))
	fmt.Fprintf(&html, `<button type="button" class="btn btn-danger"
		hx-delete="/api/media/%s"
		hx-confirm="%s"
		hx-target="#component-media-detail"
		hx-trigger="click">
		<i class="fas fa-trash"></i> %s
	</button>`, relativePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "delete"))
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

// RenderMediaPreviewWithSize renders a CSS-constrained preview of a media file with custom size
func RenderMediaPreviewWithSize(mediaPath string, size int) string {
	if !configmanager.GetPreviewsEnabled() {
		return fmt.Sprintf(`<span>%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "previews disabled"))
	}

	// validate size
	if size <= 0 {
		size = configmanager.GetDefaultPreviewSize()
	}

	// ensure media path is relative (remove media/ prefix if present)
	relativePath := strings.TrimPrefix(mediaPath, "media/")

	// get display settings
	displayMode := configmanager.GetDisplayMode()
	borderStyle := configmanager.GetBorderStyle()
	showCaption := configmanager.GetShowCaption()
	clickToEnlarge := configmanager.GetClickToEnlarge()

	// get media link mode from theme settings (direct or detail)
	mediaLinkMode := configmanager.GetThemeSetting(configmanager.GetTheme(), "mediaLinkMode")
	var mediaURL string
	if mediaLinkMode == "detail" {
		mediaURL = fmt.Sprintf("/media/%s?mode=detail", relativePath)
	} else {
		// default to direct mode
		mediaURL = fmt.Sprintf("/media/%s", relativePath)
	}

	// determine file type from extension
	ext := strings.ToLower(filepath.Ext(relativePath))

	// build CSS classes for styling
	var containerClasses []string
	containerClasses = append(containerClasses, "media-preview")
	containerClasses = append(containerClasses, "display-"+displayMode)
	containerClasses = append(containerClasses, "border-"+borderStyle)

	containerClass := strings.Join(containerClasses, " ")

	var content string
	filename := filepath.Base(relativePath)

	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		// for images, use CSS to constrain size
		var imgElement string
		if clickToEnlarge {
			imgElement = fmt.Sprintf(`
				<a href="%s" target="_blank" class="preview-link">
					<img src="/media/%s"
					     alt="%s"
					     class="media-preview-image"
					     style="max-width: %dpx; max-height: %dpx; width: auto; height: auto;"
					     loading="lazy" />
				</a>`, mediaURL, relativePath, filename, size, size)
		} else {
			imgElement = fmt.Sprintf(`
				<img src="/media/%s"
				     alt="%s"
				     class="media-preview-image"
				     style="max-width: %dpx; max-height: %dpx; width: auto; height: auto;"
				     loading="lazy" />`, relativePath, filename, size, size)
		}

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, imgElement, filename)
		} else {
			content = imgElement
		}

	case ".mp4", ".webm", ".ogg":
		// for videos, use CSS to constrain size
		videoElement := fmt.Sprintf(`
			<video controls style="max-width: %dpx; max-height: %dpx;">
				<source src="/media/%s" type="video/%s">
			</video>`, size, size, relativePath, strings.TrimPrefix(ext, "."))

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, videoElement, filename)
		} else {
			content = videoElement
		}

	case ".pdf":
		// for PDFs, use fixed iframe size
		pdfElement := fmt.Sprintf(`
			<iframe src="/media/%s" style="width: %dpx; height: %dpx;"></iframe>`,
			relativePath, size, int(float64(size)*1.4)) // taller aspect ratio for PDFs

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, pdfElement, filename)
		} else {
			content = pdfElement
		}

	default:
		// for other files, show file icon with link
		var linkElement string
		if clickToEnlarge {
			linkElement = fmt.Sprintf(`
				<a href="%s" target="_blank" class="file-link">
					<i class="fa fa-file"></i>
					<span>%s</span>
				</a>`, mediaURL, filename)
		} else {
			linkElement = fmt.Sprintf(`
				<div class="file-icon">
					<i class="fa fa-file"></i>
					<span>%s</span>
				</div>`, filename)
		}
		content = linkElement
	}

	return fmt.Sprintf(`<div class="%s">%s</div>`, containerClass, content)
}
