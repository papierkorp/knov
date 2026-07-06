// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"knov/internal/parser"
	"knov/internal/types"
)

// RenderTableComponent renders a paginated, sortable, searchable, filterable table HTML fragment.
// fullData is the unfiltered table, used to compute the per-column filter dropdown options.
func RenderTableComponent(tableData, fullData *types.TableData, filepath string, tableIndex, page, size, sortCol int, sortOrder, searchQuery string, activeFilters map[int]string) string {
	return parser.RenderTableHTML(tableData, fullData, filepath, tableIndex, page, size, sortCol, sortOrder, searchQuery, activeFilters)
}
