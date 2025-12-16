// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"knov/internal/parser"
)

// RenderTableComponent renders a paginated, sortable, searchable table HTML fragment
func RenderTableComponent(tableData *parser.TableData, filepath string, page, size, sortCol int, sortOrder, searchQuery string) string {
	return parser.RenderTableHTML(tableData, filepath, page, size, sortCol, sortOrder, searchQuery)
}

// RenderTableEditor renders the jspreadsheet-based table editor
func RenderTableEditor(tableData *parser.TableData, filepath string) string {
	return parser.RenderTableEditorHTML(tableData, filepath)
}
