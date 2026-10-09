// Package render - HTMX HTML rendering functions for media components
package render

import (
	"fmt"
	stdhtml "html"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/translation"
	"knov/internal/types"
	"knov/internal/utils"
)

// RenderMediaPreview renders a preview of a media file
func RenderMediaPreview(mediaPath, contentType string) string {
	// ensure media path is relative (remove media/ prefix if present)
	relativePath := strings.TrimPrefix(mediaPath, "media/")

	mediaURL := pathutils.ToMediaURL(relativePath)

	switch {
	case contentType == "":
		return fmt.Sprintf(`<div class="media-preview">%s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "unknown file type"))
	case contentType[:6] == "image/":
		return fmt.Sprintf(`<div class="media-preview"><img src="%s" alt="media preview" style="max-width: 300px; max-height: 300px;"></div>`, mediaURL)
	case contentType[:6] == "video/":
		return fmt.Sprintf(`<div class="media-preview"><video controls style="max-width: 300px; max-height: 300px;"><source src="%s" type="%s"></video></div>`, mediaURL, stdhtml.EscapeString(contentType))
	case contentType == "application/pdf":
		return fmt.Sprintf(`<div class="media-preview"><iframe src="%s" style="width: 300px; height: 400px;"></iframe></div>`, mediaURL)
	case contentType[:5] == "text/":
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="%s" target="_blank">%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "text file"),
			mediaURL,
			translation.SprintfForRequest(configmanager.GetLanguage(), "view"))
	default:
		return fmt.Sprintf(`<div class="media-preview">%s: <a href="%s" download>%s</a></div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "file"),
			mediaURL,
			translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
	}
}

// RenderMediaListCompact renders a compact list of media files for narrow panels.
// linkTarget controls where each item links: "detail" → /media/x?mode=detail, "view" → /media/x
func RenderMediaListCompact(mediaFiles []files.File, linkTarget string) string {
	var html strings.Builder
	html.WriteString(`<div class="media-list-compact">`)

	if len(mediaFiles) == 0 {
		fmt.Fprintf(&html, `<div class="media-compact-empty">%s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "no media files found"))
		html.WriteString(`</div>`)
		return html.String()
	}

	for _, file := range mediaFiles {
		relativePath := strings.TrimPrefix(file.Path, "media/")
		fileExt := strings.ToLower(filepath.Ext(relativePath))
		filename := filepath.Base(relativePath)
		mediaURL := pathutils.ToMediaURL(relativePath)
		escFilename := stdhtml.EscapeString(filename)

		href := mediaURL
		if linkTarget == "detail" {
			href += "?mode=detail"
		}

		fmt.Fprintf(&html, `<a class="media-compact-item" href="%s">`, href)

		if files.IsImageFile(fileExt) {
			fmt.Fprintf(&html, `<img src="%s" alt="%s" class="media-compact-thumb" loading="lazy">`, mediaURL, escFilename)
		} else {
			icon := files.GetFileTypeIcon(fileExt)
			fmt.Fprintf(&html, `<span class="media-compact-icon"><i class="fas %s"></i></span>`, icon)
		}

		fmt.Fprintf(&html, `<span class="media-compact-name">%s</span>`, escFilename)
		html.WriteString(`</a>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderMediaList renders a grid of media files with previews and filter controls
func RenderMediaList(mediaFiles []files.File, filter string, totalCount, orphanedCount, hiddenCount int) string {
	var html strings.Builder

	// wrapper for htmx target
	html.WriteString(`<div id="component-media-content">`)

	// filter controls
	html.WriteString(`<div id="component-media-filter" class="component-media-filter">`)
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

	// swap target for delete errors (HX-Retarget), keeps the grid intact
	html.WriteString(`<div id="component-media-error"></div>`)

	// hidden-by-settings warning
	if hiddenCount > 0 {
		fmt.Fprintf(&html, `<div class="media-hidden-warning"><i class="fa fa-eye-slash"></i> %s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "%d files not shown due to hide settings", hiddenCount))
	}

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
	html.WriteString(`<div id="component-media-grid" class="component-media-grid">`)

	// get orphaned media for badge display
	orphanedMedia, _ := files.GetOrphanedMediaFromCache()

	for _, file := range mediaFiles {
		// check if this media is orphaned
		isOrphaned := slices.Contains(orphanedMedia, file.Path)

		// ensure media path is relative (remove media/ prefix)
		relativePath := strings.TrimPrefix(file.Path, "media/")
		fileExt := strings.ToLower(filepath.Ext(relativePath))
		filename := filepath.Base(relativePath)
		mediaURL := pathutils.ToMediaURL(relativePath)
		escFilename := stdhtml.EscapeString(filename)

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
			fmt.Fprintf(&html, `<a href="%s" target="_blank"><img src="%s" alt="%s" loading="lazy" class="media-thumbnail"></a>`,
				mediaURL, mediaURL, escFilename)
		} else if files.IsVideoFile(fileExt) {
			fmt.Fprintf(&html, `<div class="media-video-preview"><video preload="none" class="media-thumbnail" poster=""><source src="%s" type="video/%s"></video><div class="video-overlay"><i class="fas fa-play"></i></div></div>`,
				mediaURL, strings.TrimPrefix(fileExt, "."))
		} else {
			icon := files.GetFileTypeIcon(fileExt)
			fmt.Fprintf(&html, `<div class="media-icon"><i class="fas %s"></i></div>`, icon)
		}
		html.WriteString(`</div>`)

		// media info
		html.WriteString(`<div class="media-info">`)
		fmt.Fprintf(&html, `<div class="media-filename" title="%s">%s</div>`, escFilename, escFilename)

		// show file size if available in metadata
		if file.Metadata != nil && file.Metadata.Size > 0 {
			sizeStr := utils.FormatFileSize(file.Metadata.Size)
			fmt.Fprintf(&html, `<div class="media-filesize">%s</div>`, sizeStr)
		}
		html.WriteString(`</div>`)

		// media actions
		html.WriteString(`<div class="media-actions">`)
		fmt.Fprintf(&html, `<a href="%s?mode=detail" class="btn btn-sm btn-primary"><i class="fas fa-info-circle"></i> %s</a>`,
			mediaURL, translation.SprintfForRequest(configmanager.GetLanguage(), "details"))
		fmt.Fprintf(&html, `<a href="%s" download class="btn btn-sm btn-secondary"><i class="fas fa-download"></i> %s</a>`,
			mediaURL, translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
		fmt.Fprintf(&html, `<button type="button" class="btn btn-sm btn-danger" hx-delete="%s" hx-confirm="%s" hx-target="#component-media-content" hx-trigger="click"><i class="fas fa-trash"></i> %s</button>`,
			pathutils.ToRouteURL("/api/media/", relativePath),
			translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "delete"))
		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // close media-item
	}

	html.WriteString(`</div>`) // close media-grid
	html.WriteString(`</div>`) // close component-media-content
	return html.String()
}

// RenderMediaPathDisplay renders the read-only path row with an inline edit button.
// Used as the hx-swap target after a successful rename or cancel.
func RenderMediaPathDisplay(relativePath string) string {
	escPath := stdhtml.EscapeString(relativePath)
	return fmt.Sprintf(`<dt>%s</dt>
<dd id="media-path-display" class="media-path-row">
	<span>%s</span>
	<button class="btn-icon"
		hx-get="/api/media/rename-form/%s"
		hx-target="#media-path-display"
		hx-swap="outerHTML"
		title="%s">
		<i class="fas fa-pen"></i>
	</button>
</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path"),
		escPath,
		escPath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "rename"))
}

// RenderMediaRenameForm renders the inline rename input form.
// Replaces the path display row when the edit button is clicked.
func RenderMediaRenameForm(relativePath string) string {
	escPath := stdhtml.EscapeString(relativePath)
	return fmt.Sprintf(`<dd id="media-path-display" class="media-path-row">
	<form hx-post="/api/media/rename/%s"
		hx-target="#media-path-display"
		hx-swap="outerHTML">
		<input type="text" name="newpath" value="%s" class="form-input" required autofocus />
		<button type="submit" class="btn-icon" title="%s">
			<i class="fas fa-check"></i>
		</button>
		<button type="button" class="btn-icon"
			hx-get="/api/media/path-display/%s"
			hx-target="#media-path-display"
			hx-swap="outerHTML"
			title="%s">
			<i class="fas fa-times"></i>
		</button>
	</form>
</dd>`,
		escPath,
		escPath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		escPath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"))
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
	mediaURL := pathutils.ToMediaURL(relativePath)
	escPath := stdhtml.EscapeString(relativePath)

	var html strings.Builder
	html.WriteString(`<div id="component-media-detail" class="component-media-detail">`)

	// media preview section
	html.WriteString(`<div class="media-preview-large">`)
	if files.IsImageFile(fileExt) {
		fmt.Fprintf(&html, `<img src="%s" alt="%s" class="media-preview-image">`,
			mediaURL, stdhtml.EscapeString(filename))
	} else if files.IsVideoFile(fileExt) {
		fmt.Fprintf(&html, `<video controls class="media-preview-video">
			<source src="%s" type="video/%s">
			%s
		</video>`, mediaURL, strings.TrimPrefix(fileExt, "."),
			translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support video playback"))
	} else if files.IsAudioFile(fileExt) {
		fmt.Fprintf(&html, `<audio controls class="media-preview-audio">
			<source src="%s" type="audio/%s">
			%s
		</audio>`, mediaURL, strings.TrimPrefix(fileExt, "."),
			translation.SprintfForRequest(configmanager.GetLanguage(), "your browser does not support audio playback"))
	} else {
		icon := files.GetFileTypeIcon(fileExt)
		fmt.Fprintf(&html, `<div class="media-preview-icon">
			<i class="fas %s"></i>
			<p>%s</p>
		</div>`, icon, stdhtml.EscapeString(filename))
	}
	html.WriteString(`</div>`)

	// metadata section
	html.WriteString(`<div class="media-metadata">`)
	fmt.Fprintf(&html, `<h2>%s</h2>`, stdhtml.EscapeString(filename))

	html.WriteString(`<dl class="media-info">`)

	// inline-editable path row
	html.WriteString(RenderMediaPathDisplay(relativePath))

	fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "type"), types.ServeMimeType(fileExt))

	if metadata.Size > 0 {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "size"), utils.FormatFileSize(metadata.Size))
	}

	if !metadata.CreatedAt.IsZero() {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "created"),
			configmanager.FormatDateTime(metadata.CreatedAt))
	}

	if !metadata.LastEdited.IsZero() {
		fmt.Fprintf(&html, `<dt>%s</dt><dd>%s</dd>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "last modified"),
			configmanager.FormatDateTime(metadata.LastEdited))
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
			displayText := GetLinkDisplayText(pathutils.ToWithPrefix(link))
			fmt.Fprintf(&html, `<li><a href="%s" title="%s">%s</a></li>`, pathutils.ToFileURL(pathutils.DocsPath(linkPath)), linkPath, displayText)
		}
		html.WriteString(`</ul>`)
	}
	html.WriteString(`</div>`)

	// editable metadata fields
	tagsStr := strings.Join(metadata.Tags, ", ")
	parentsStr := strings.Join(metadata.Parents, ", ")

	html.WriteString(`<div class="media-edit-fields">`)
	html.WriteString(`<div class="form-field">`)
	fmt.Fprintf(&html, `<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "tags"))
	html.WriteString(GenerateTagChipsInputWithSave("media-tags", "tags", tagsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add tags"),
		"/api/metadata/tags?format=options", metadata.Path, "/api/metadata/tags"))
	html.WriteString(`</div>`)

	html.WriteString(`<div class="form-field">`)
	fmt.Fprintf(&html, `<label>%s</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "parents"))
	html.WriteString(GenerateTagChipsInputWithSave("media-parents", "parents", parentsStr,
		translation.SprintfForRequest(configmanager.GetLanguage(), "add parent files"),
		"/api/files/list?format=options", metadata.Path, "/api/metadata/parents"))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	// actions
	html.WriteString(`<div class="media-actions">`)
	fmt.Fprintf(&html, `<a href="%s" download class="btn btn-primary">
		<i class="fas fa-download"></i> %s
	</a>`, mediaURL, translation.SprintfForRequest(configmanager.GetLanguage(), "download"))
	fmt.Fprintf(&html, `<a href="%s" target="_blank" class="btn btn-secondary">
		<i class="fas fa-external-link-alt"></i> %s
	</a>`, mediaURL, translation.SprintfForRequest(configmanager.GetLanguage(), "open in new tab"))
	mdPrefix := ""
	if files.IsImageFile(fileExt) {
		mdPrefix = "!"
	}
	copyLink := fmt.Sprintf("%s[%s](media/%s)", mdPrefix, filename, relativePath)
	fmt.Fprintf(&html, `<button type="button" class="btn btn-secondary" data-copy-link="%s" onclick="copyMediaLink(this)">
		<i class="fas fa-copy"></i> %s
	</button>`, stdhtml.EscapeString(copyLink), translation.SprintfForRequest(configmanager.GetLanguage(), "copy link"))
	fmt.Fprintf(&html, `<button type="button" class="btn btn-danger"
		hx-delete="/api/media/%s"
		hx-confirm="%s"
		hx-target="#component-media-detail"
		hx-trigger="click">
		<i class="fas fa-trash"></i> %s
	</button>`, escPath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "are you sure you want to delete this file?"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "delete"))
	html.WriteString(`</div>`)

	fmt.Fprintf(&html, `<script>if(!window.copyMediaLink){window.copyMediaLink=function(btn){navigator.clipboard.writeText(btn.dataset.copyLink).then(function(){document.body.dispatchEvent(new CustomEvent('notify',{detail:{type:'success',message:%s}}));});};}</script>`,
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "link copied to clipboard")))

	html.WriteString(`</div>`) // close media-metadata
	html.WriteString(`</div>`) // close media-detail

	return html.String()
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
	imageClickBehavior := configmanager.GetImageClickBehavior()
	mediaURL := pathutils.ToMediaURL(relativePath)
	detailURL := mediaURL + "?mode=detail"

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

	switch {
	case configmanager.IsImageExtension(ext):
		// for images, use CSS to constrain size
		img := fmt.Sprintf(`<img src="%s"
				     alt="%s"
				     class="media-preview-image"
				     style="max-width: %dpx; max-height: %dpx; width: auto; height: auto;"
				     loading="lazy" />`,
			mediaURL, stdhtml.EscapeString(filename), size, size)

		var imgElement string
		switch imageClickBehavior {
		case "enlarge":
			imgElement = fmt.Sprintf(`
				<button type="button" class="preview-trigger" onclick="openMediaLightbox(this)" data-lightbox-src="%s" data-lightbox-alt="%s">
					%s
				</button>`, mediaURL, stdhtml.EscapeString(filename), img)
		case "detail":
			imgElement = fmt.Sprintf(`
				<a href="%s" target="_blank" class="preview-link">
					%s
				</a>`, detailURL, img)
		default:
			imgElement = img
		}

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, imgElement, stdhtml.EscapeString(filename))
		} else {
			content = imgElement
		}

	case configmanager.IsVideoExtension(ext):
		// for videos, use CSS to constrain size
		videoElement := fmt.Sprintf(`
			<video controls style="max-width: %dpx; max-height: %dpx;">
				<source src="%s" type="%s">
			</video>`, size, size, mediaURL, types.MimeTypeByExtension(ext))

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, videoElement, stdhtml.EscapeString(filename))
		} else {
			content = videoElement
		}

	case types.MimeTypeByExtension(ext) == "application/pdf":
		// for PDFs, use fixed iframe size
		pdfElement := fmt.Sprintf(`
			<iframe src="%s" style="width: %dpx; height: %dpx;"></iframe>`,
			mediaURL, size, int(float64(size)*1.4)) // taller aspect ratio for PDFs

		if showCaption {
			content = fmt.Sprintf(`
				<div class="preview-content">
					%s
					<div class="preview-caption">%s</div>
				</div>`, pdfElement, stdhtml.EscapeString(filename))
		} else {
			content = pdfElement
		}

	default:
		// non-image previews (files, video, pdf) always link to the detail
		// page - imageClickBehavior only governs image previews
		content = fmt.Sprintf(`
			<a href="%s" target="_blank" class="file-link">
				<i class="fa fa-file"></i>
				<span>%s</span>
			</a>`, detailURL, stdhtml.EscapeString(filename))
	}

	return fmt.Sprintf(`<div class="%s">%s</div>`, containerClass, content)
}

// RenderMediaStorageStats renders storage statistics for the admin dashboard, one row per
// media category plus a total row
func RenderMediaStorageStats(stats *files.MediaStorageStats) string {
	lang := configmanager.GetLanguage()
	var html strings.Builder
	fmt.Fprintf(&html, `<div id="storage-stats" class="storage-stats-table"><table><thead><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		translation.SprintfForRequest(lang, "type"),
		translation.SprintfForRequest(lang, "total media files"),
		translation.SprintfForRequest(lang, "used media files"),
		translation.SprintfForRequest(lang, "orphaned media files"))
	for _, c := range stats.Categories {
		writeMediaStatsRow(&html, "", mediaCategoryLabel(c.Category), c)
	}
	writeMediaStatsRow(&html, "stats-total", translation.SprintfForRequest(lang, "total"), stats.MediaCategoryStats)
	html.WriteString(`</tbody></table></div>`)
	return html.String()
}

// writeMediaStatsRow writes one row of the storage statistics table
func writeMediaStatsRow(html *strings.Builder, class, label string, c files.MediaCategoryStats) {
	fmt.Fprintf(html, `<tr class="%s"><td>%s</td><td>%d <span class="stats-size">(%s)</span></td><td>%d <span class="stats-size">(%s)</span></td><td>%d <span class="stats-size">(%s)</span></td></tr>`,
		class, label,
		c.TotalFiles, utils.FormatFileSize(c.TotalSize),
		c.UsedFiles, utils.FormatFileSize(c.UsedSize),
		c.OrphanedFiles, utils.FormatFileSize(c.OrphanedSize))
}

// mediaCategoryLabel returns the translated label of a types.MediaCategory
func mediaCategoryLabel(category string) string {
	lang := configmanager.GetLanguage()
	switch category {
	case types.MediaCategoryImage:
		return translation.SprintfForRequest(lang, "images")
	case types.MediaCategoryVideo:
		return translation.SprintfForRequest(lang, "videos")
	case types.MediaCategoryAudio:
		return translation.SprintfForRequest(lang, "audio files")
	case types.MediaCategoryDocument:
		return translation.SprintfForRequest(lang, "documents")
	case types.MediaCategoryArchive:
		return translation.SprintfForRequest(lang, "archives")
	case types.MediaCategoryText:
		return translation.SprintfForRequest(lang, "text files")
	case types.MediaCategoryFont:
		return translation.SprintfForRequest(lang, "fonts")
	case types.MediaCategoryProgram:
		return translation.SprintfForRequest(lang, "programs")
	}
	return translation.SprintfForRequest(lang, "other")
}

// RenderMisplacedMedia renders the result of ScanMisplacedMedia as a table of planned
// moves (docs path -> media path) with checkboxes (all selected) and a button relocating the selected ones.
// Files without a target aren't an allowed media type and are listed as staying in place.
func RenderMisplacedMedia(items []files.MisplacedMedia) string {
	lang := configmanager.GetLanguage()
	var html strings.Builder
	html.WriteString(`<div id="component-misplaced-media">`)

	if len(items) == 0 {
		fmt.Fprintf(&html, `<p class="no-items">%s</p></div>`, translation.SprintfForRequest(lang, "no misplaced media files found"))
		return html.String()
	}

	fmt.Fprintf(&html, `<form hx-post="/api/media/misplaced/relocate" hx-target="#misplaced-media-result" hx-swap="innerHTML" hx-status:4xx="swap:none" hx-status:5xx="swap:none" hx-confirm="%s">`,
		stdhtml.EscapeString(translation.SprintfForRequest(lang, "Move the selected files to the media folder and update all links?")))
	fmt.Fprintf(&html, `<table class="misplaced-media-table"><thead><tr><th><input type="checkbox" checked onclick="%s"></th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		toggleAllCheckboxesJS,
		translation.SprintfForRequest(lang, "file"),
		translation.SprintfForRequest(lang, "detected as"),
		translation.SprintfForRequest(lang, "new path"))

	movable := 0
	for _, item := range items {
		escaped := stdhtml.EscapeString(item.Path)
		checkbox := ""
		target := translation.SprintfForRequest(lang, "not an allowed media type, stays in place")
		if item.Target != "" {
			checkbox = fmt.Sprintf(`<input type="checkbox" name="path" value="%s" checked>`, escaped)
			target = stdhtml.EscapeString("media/" + item.Target)
			movable++
		}
		detectedBy := translation.SprintfForRequest(lang, "by content")
		if item.DetectedBy == "extension" {
			detectedBy = translation.SprintfForRequest(lang, "by extension")
		}
		fmt.Fprintf(&html, `<tr><td>%s</td><td>%s</td><td>%s (%s)</td><td>%s</td></tr>`, checkbox, escaped, stdhtml.EscapeString(item.DetectedAs), detectedBy, target)
	}
	html.WriteString(`</tbody></table>`)

	if movable > 0 {
		fmt.Fprintf(&html, `<button type="submit" class="btn-danger"><i class="fa fa-folder-open"></i> %s</button>`,
			translation.SprintfForRequest(lang, "Move Selected to Media Folder"))
	}
	html.WriteString(`</form></div>`)
	return html.String()
}

// RenderOrphanedMedia renders the orphaned media files as a table of checkboxes (all selected)
// with a button deleting the selected ones.
func RenderOrphanedMedia(paths []string) string {
	lang := configmanager.GetLanguage()
	var html strings.Builder
	html.WriteString(`<div id="component-orphaned-media">`)

	if len(paths) == 0 {
		fmt.Fprintf(&html, `<p class="no-items">%s</p></div>`, translation.SprintfForRequest(lang, "no orphaned media files found"))
		return html.String()
	}

	fmt.Fprintf(&html, `<form hx-post="/api/media/cleanup-orphaned" hx-target="#orphaned-media-result" hx-swap="innerHTML" hx-status:4xx="swap:none" hx-status:5xx="swap:none" hx-confirm="%s">`,
		stdhtml.EscapeString(translation.SprintfForRequest(lang, "Delete the selected orphaned media files? This cannot be undone.")))
	fmt.Fprintf(&html, `<table class="orphaned-media-table"><thead><tr><th><input type="checkbox" checked onclick="%s"></th><th>%s</th></tr></thead><tbody>`,
		toggleAllCheckboxesJS, translation.SprintfForRequest(lang, "file"))
	for _, p := range paths {
		escaped := stdhtml.EscapeString(p)
		fmt.Fprintf(&html, `<tr><td><input type="checkbox" name="path" value="%s" checked></td><td>%s</td></tr>`, escaped, escaped)
	}
	html.WriteString(`</tbody></table>`)

	fmt.Fprintf(&html, `<button type="submit" class="btn-danger"><i class="fa fa-trash"></i> %s</button>`,
		translation.SprintfForRequest(lang, "Delete Selected"))
	html.WriteString(`</form></div>`)
	return html.String()
}
