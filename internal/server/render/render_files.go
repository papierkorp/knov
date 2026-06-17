// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/translation"
)

// RenderFilesOptions renders file list as select options
func RenderFilesOptions(allFiles []files.File) string {
	var html strings.Builder
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select a file...") + `</option>`)
	for _, file := range allFiles {
		path := strings.TrimPrefix(file.Path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, path, path))
	}
	return html.String()
}

// RenderFilesOptionsFromPaths renders file paths as select options
func RenderFilesOptionsFromPaths(filePaths []string) string {
	var html strings.Builder
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select a file...") + `</option>`)
	for _, path := range filePaths {
		displayPath := strings.TrimPrefix(path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, displayPath, displayPath))
	}
	return html.String()
}

// RenderFilesDatalist renders files as datalist options
func RenderFilesDatalist(allFiles []files.File) string {
	var html strings.Builder
	for _, file := range allFiles {
		path := strings.TrimPrefix(file.Path, "data/")
		html.WriteString(fmt.Sprintf(`<option value="%s">`, path))
	}
	return html.String()
}

// RenderFilesList renders files as list with direct navigation links.
// If deletable is true, each row includes a hover-revealed delete button.
func RenderFilesList(allFiles []files.File, deletable bool) string {
	var html strings.Builder
	if deletable {
		html.WriteString(`<ul class="browse-list-deletable">`)
	} else {
		html.WriteString("<ul>")
	}
	deleteLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "delete file")
	for _, file := range allFiles {
		displayText := GetLinkDisplayText(file.Path)
		relPath := strings.TrimPrefix(file.Path, "docs/")
		if deletable {
			confirmMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "delete") + " " + displayText + "?"
			html.WriteString(fmt.Sprintf(`
				<li class="browse-item-row">
					<a href="%s">%s</a>
					<button class="btn-danger-icon browse-delete-btn"
					        hx-delete="/api/files/delete/%s"
					        hx-confirm="%s"
					        hx-target="closest li"
					        hx-swap="outerHTML"
					        title="%s"><i class="fa fa-trash"></i></button>
				</li>`,
				file.ViewURL(), displayText, url.PathEscape(relPath), confirmMsg, deleteLabel))
		} else {
			html.WriteString(fmt.Sprintf(`
				<li>
				  <a href="%s">%s</a>
				</li>`,
				file.ViewURL(), displayText))
		}
	}
	html.WriteString("</ul>")
	return html.String()
}

// RenderFilteredFiles renders filtered files list with count - reuses RenderFileList
func RenderFilteredFiles(filteredFiles []files.File) string {
	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>%s</p>", translation.SprintfForRequest(configmanager.GetLanguage(), "found %d files", len(filteredFiles))))
	html.WriteString(RenderFileList(filteredFiles))
	return html.String()
}

// RenderFileHeader renders file header with breadcrumb
func RenderFileHeader(filepath string) string {
	return fmt.Sprintf(`<hr/><div id="current-file-breadcrumb"><a href="/files/%s">→ %s</a></div>`, filepath, filepath)
}

// RenderBrowseFilesHTML renders browsed files as list.
// If deletable is true, each row includes a hover-revealed delete button.
func RenderBrowseFilesHTML(files []files.File, deletable bool) string {
	if len(files) == 0 {
		return "<p>" + translation.SprintfForRequest(configmanager.GetLanguage(), "no files found") + "</p>"
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>%s</p>", translation.SprintfForRequest(configmanager.GetLanguage(), "found %d files", len(files))))
	html.WriteString(RenderFilesList(files, deletable))
	return html.String()
}

// RenderFileForm renders a simple file creation/editing form
func RenderFileForm(filePath string) string {
	return fmt.Sprintf(`
		<form class="file-form">
			<div class="form-group">
				<label>%s:</label>
				<input type="text" name="filepath" value="%s" placeholder="%s" />
			</div>
			<div class="form-group">
				<label>%s:</label>
				<textarea name="content" rows="10" placeholder="%s"></textarea>
			</div>
			<button type="submit">%s</button>
		</form>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file path"),
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "path/to/file.md"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "content"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "file content here..."),
		translation.SprintfForRequest(configmanager.GetLanguage(), "save file"))
}

// FolderEntry represents a folder or file entry
type FolderEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// RenderFolderContent renders folder structure with folders and files.
// target is the CSS selector HTMX should swap into when navigating sub-folders (e.g. "#folder-content").
func RenderFolderContent(currentPath string, folders []FolderEntry, filesInDir []FolderEntry, target string) string {
	var html strings.Builder
	encodedTarget := url.QueryEscape(target)

	html.WriteString(`<div class="folder-content">`)

	// folders section
	if len(folders) > 0 || currentPath != "" {
		html.WriteString(`<div class="folders-list">`)
		html.WriteString(fmt.Sprintf(`<h3>%s</h3>`, translation.SprintfForRequest(configmanager.GetLanguage(), "folders")))
		html.WriteString(`<ul>`)

		// add parent folder link if not at root
		if currentPath != "" {
			parentPath := filepath.Dir(currentPath)
			if parentPath == "." {
				parentPath = ""
			}
			html.WriteString(fmt.Sprintf(`
				<li class="folder-item folder-parent">
					<a href="#" hx-get="/api/files/folder?path=%s&target=%s" hx-target="%s">
						/ ..
					</a>
				</li>`,
				parentPath, encodedTarget, target))
		}

		for _, folder := range folders {
			html.WriteString(fmt.Sprintf(`
				<li class="folder-item">
					<a href="#" hx-get="/api/files/folder?path=%s&target=%s" hx-target="%s">
						/ %s
					</a>
				</li>`,
				folder.Path, encodedTarget, target, folder.Name))
		}
		html.WriteString(`</ul></div>`)
	}

	// files section
	if len(filesInDir) > 0 {
		html.WriteString(`<div class="files-list">`)
		html.WriteString(fmt.Sprintf(`<h3>%s</h3>`, translation.SprintfForRequest(configmanager.GetLanguage(), "files")))
		html.WriteString(`<ul>`)
		for _, file := range filesInDir {
			html.WriteString(fmt.Sprintf(`
				<li class="file-item">
					<a href="/files/%s">
						() %s
					</a>
				</li>`,
				file.Path, GetLinkDisplayText(file.Path)))
		}
		html.WriteString(`</ul></div>`)
	}

	if len(folders) == 0 && len(filesInDir) == 0 {
		html.WriteString(fmt.Sprintf(`<p>%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "folder is empty")))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// renderTreeChildren recursively renders a TreeNode's children as nested HTML lists
func renderTreeChildren(html *strings.Builder, node *files.TreeNode, deletable bool, pathPrefix string) {
	if len(node.Children) == 0 {
		return
	}
	html.WriteString(`<ul class="fp-tree-list">`)
	for _, child := range node.Children {
		html.WriteString(`<li>`)
		if child.IsDir {
			dirPath := pathPrefix + child.Name
			if deletable {
				renameLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "rename")
				fmt.Fprintf(html, `<span class="browse-item-row"><button class="fp-tree-dir" draggable="true" data-path="%s" data-type="folder" onclick="this.closest('li').classList.toggle('fp-tree-collapsed')"><i class="fa fa-folder"></i> %s</button><button class="browse-rename-btn" data-path="%s" data-type="folder" title="%s"><i class="fa fa-pen"></i></button></span>`, dirPath, child.Name, dirPath, renameLabel)
			} else {
				fmt.Fprintf(html, `<button class="fp-tree-dir" draggable="true" data-path="%s" data-type="folder" onclick="this.closest('li').classList.toggle('fp-tree-collapsed')"><i class="fa fa-folder"></i> %s</button>`, dirPath, child.Name)
			}
			renderTreeChildren(html, child, deletable, dirPath+"/")
		} else {
			if deletable {
				relPath := strings.TrimPrefix(child.Path, "docs/")
				renameLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "rename")
				deleteLabel := translation.SprintfForRequest(configmanager.GetLanguage(), "delete file")
				confirmMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "delete") + " " + child.Name + "?"
				fmt.Fprintf(html, `<span class="browse-item-row" draggable="true" data-path="%s" data-type="file"><a class="fp-tree-file" href="/files/%s">%s</a><button class="browse-rename-btn" data-path="%s" data-type="file" title="%s"><i class="fa fa-pen"></i></button><button class="btn-danger-icon browse-delete-btn" hx-delete="/api/files/delete/%s" hx-confirm="%s" hx-target="closest li" hx-swap="outerHTML" title="%s"><i class="fa fa-trash"></i></button></span>`,
					relPath, child.Path, GetLinkDisplayText(child.Path), relPath, renameLabel, url.PathEscape(relPath), confirmMsg, deleteLabel)
			} else {
				relPath := strings.TrimPrefix(child.Path, "docs/")
				fmt.Fprintf(html, `<a class="fp-tree-file" draggable="true" data-path="%s" data-type="file" href="/files/%s">%s</a>`,
					relPath, child.Path, GetLinkDisplayText(child.Path))
			}
		}
		html.WriteString(`</li>`)
	}
	html.WriteString(`</ul>`)
}

// RenderTreeOverview renders a pre-built file tree as indented HTML.
// If deletable is true, file rows include a hover-revealed delete button.
func RenderTreeOverview(root *files.TreeNode, deletable bool) string {
	var html strings.Builder
	html.WriteString(`<div class="fp-tree">`)
	renderTreeChildren(&html, root, deletable, "")
	html.WriteString(`</div>`)
	return html.String()
}
