// Package renderer handles table HTML rendering
package renderer

import (
	"fmt"

	"knov/internal/parser"
)

// RenderTableHTML generates complete table HTML with pagination controls
func RenderTableHTML(data *parser.TableData, filepath string, page, size int, sortCol int, sortOrder string, searchQuery string) string {
	var html string

	totalPages := (data.Total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page-1)*size + 1
	end := start + len(data.Rows) - 1

	// build base URL params
	baseParams := fmt.Sprintf("filepath=%s&size=%d", filepath, size)
	if sortCol >= 0 {
		baseParams += fmt.Sprintf("&sort=%d&order=%s", sortCol, sortOrder)
	}
	if searchQuery != "" {
		baseParams += fmt.Sprintf("&search=%s", searchQuery)
	}

	html += `<div class="table-container">`

	// search bar
	html += `<div class="table-controls">`
	html += fmt.Sprintf(`
		<input type="text" 
		       class="table-search" 
		       placeholder="Search table..." 
		       value="%s"
		       hx-get="/api/components/table?%s&page=1"
		       hx-trigger="keyup changed delay:300ms"
		       hx-target="#table-wrapper"
		       hx-include="this"
		       name="search">
	`, searchQuery, baseParams)
	html += `</div>`

	// table wrapper
	html += `<div class="table-wrapper">`
	html += `<table class="dokuwiki-table" data-sortable="true" data-searchable="true">`

	// headers
	html += `<thead><tr>`
	for _, header := range data.Headers {
		nextOrder := "asc"
		sortIndicator := ""
		if sortCol == header.ColumnIdx {
			if sortOrder == "asc" {
				nextOrder = "desc"
				sortIndicator = " ↑"
			} else {
				nextOrder = "asc"
				sortIndicator = " ↓"
			}
		}

		headerParams := fmt.Sprintf("filepath=%s&size=%d&page=1&sort=%d&order=%s",
			filepath, size, header.ColumnIdx, nextOrder)
		if searchQuery != "" {
			headerParams += fmt.Sprintf("&search=%s", searchQuery)
		}

		html += fmt.Sprintf(`<th data-type="%s" data-align="%s" hx-get="/api/components/table?%s" hx-target="#table-wrapper" class="sortable">%s%s</th>`,
			header.DataType, header.Align, headerParams, header.Content, sortIndicator)
	}
	html += `</tr></thead>`

	// body
	html += `<tbody>`
	for _, row := range data.Rows {
		html += `<tr>`
		for _, cell := range row {
			html += fmt.Sprintf(`<td data-type="%s" data-align="%s">%s</td>`,
				cell.DataType, cell.Align, cell.Content)
		}
		html += `</tr>`
	}
	html += `</tbody>`

	html += `</table>`
	html += `</div>` // close table-wrapper

	// pagination controls
	html += `<div class="pagination">`

	// first button
	if page > 1 {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=1" hx-target="#table-wrapper" class="page-btn">First</button>`,
			baseParams)
	} else {
		html += `<button class="page-btn" disabled>First</button>`
	}

	// prev button
	if page > 1 {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Prev</button>`,
			baseParams, page-1)
	} else {
		html += `<button class="page-btn" disabled>Prev</button>`
	}

	// page info
	html += fmt.Sprintf(`<span class="page-info">Page %d of %d</span>`, page, totalPages)

	// next button
	if page < totalPages {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Next</button>`,
			baseParams, page+1)
	} else {
		html += `<button class="page-btn" disabled>Next</button>`
	}

	// last button
	if page < totalPages {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Last</button>`,
			baseParams, totalPages)
	} else {
		html += `<button class="page-btn" disabled>Last</button>`
	}

	html += `</div>` // close pagination

	// row info
	if data.Total > 0 {
		html += fmt.Sprintf(`<div class="table-info">Showing %d-%d of %d rows</div>`,
			start, end, data.Total)
	} else {
		html += `<div class="table-info">No results found</div>`
	}

	html += `</div>` // close table-container

	return html
}
