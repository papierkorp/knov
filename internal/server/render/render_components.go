// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"knov/internal/filetype"
)

// RenderTableComponent renders a paginated, sortable, searchable table HTML fragment
func RenderTableComponent(tableData *filetype.TableData, filepath string, page, size, sortCol int, sortOrder, searchQuery string) string {
	return filetype.RenderTableHTML(tableData, filepath, page, size, sortCol, sortOrder, searchQuery)
}

// RenderEditorComponent renders a file editor component with save/cancel buttons
func RenderEditorComponent(filepath, content string) string {
	return fmt.Sprintf(`
		<div id="component-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">save</button>
					<button type="button" onclick="location.reload()" class="btn-secondary">cancel</button>
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content)
}
