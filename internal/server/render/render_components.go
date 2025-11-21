// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"knov/internal/filetype"
)

// RenderTableComponent renders a paginated, sortable, searchable table HTML fragment
func RenderTableComponent(tableData *filetype.TableData, filepath string, page, size, sortCol int, sortOrder, searchQuery string) string {
	return filetype.RenderTableHTML(tableData, filepath, page, size, sortCol, sortOrder, searchQuery)
}
