package parser

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/translation"
	"knov/internal/types"
)

func PaginateTable(data *types.TableData, page, size int) *types.TableData {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 25
	}

	start := (page - 1) * size
	end := start + size

	if start >= len(data.Rows) {
		return &types.TableData{
			Headers: data.Headers,
			Rows:    [][]types.TableCell{},
			Total:   data.Total,
		}
	}

	if end > len(data.Rows) {
		end = len(data.Rows)
	}

	return &types.TableData{
		Headers: data.Headers,
		Rows:    data.Rows[start:end],
		Total:   data.Total,
	}
}

func SortTable(data *types.TableData, column int, order string) *types.TableData {
	if column < 0 || column >= len(data.Headers) {
		return data
	}

	sortedRows := make([][]types.TableCell, len(data.Rows))
	copy(sortedRows, data.Rows)
	header := data.Headers[column]

	sort.SliceStable(sortedRows, func(i, j int) bool {
		if column >= len(sortedRows[i]) || column >= len(sortedRows[j]) {
			return false
		}

		cellI := sortedRows[i][column]
		cellJ := sortedRows[j][column]

		var less bool
		switch header.DataType {
		case "number", "currency":
			numI := parseNumber(cellI.RawValue)
			numJ := parseNumber(cellJ.RawValue)
			less = numI < numJ
		case "date":
			dateI := parseDate(cellI.RawValue)
			dateJ := parseDate(cellJ.RawValue)
			less = dateI < dateJ
		default:
			less = strings.ToLower(cellI.RawValue) < strings.ToLower(cellJ.RawValue)
		}

		if order == "desc" {
			return !less
		}
		return less
	})

	return &types.TableData{
		Headers: data.Headers,
		Rows:    sortedRows,
		Total:   data.Total,
	}
}

func SearchTable(data *types.TableData, query string) *types.TableData {
	if query == "" {
		return data
	}

	query = strings.ToLower(query)
	var filteredRows [][]types.TableCell

	for _, row := range data.Rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell.Content), query) {
				filteredRows = append(filteredRows, row)
				break
			}
		}
	}

	return &types.TableData{
		Headers: data.Headers,
		Rows:    filteredRows,
		Total:   len(filteredRows),
	}
}

func parseNumber(s string) float64 {
	s = regexp.MustCompile(`[$â‚¬Â£Â¥,\s]`).ReplaceAllString(s, "")
	num, _ := strconv.ParseFloat(s, 64)
	return num
}

func parseDate(s string) int64 {
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s); matched {
		s = strings.ReplaceAll(s, "-", "")
		num, _ := strconv.ParseInt(s, 10, 64)
		return num
	}
	if matched, _ := regexp.MatchString(`^\d{2}\.\d{2}\.\d{4}$`, s); matched {
		parts := strings.Split(s, ".")
		if len(parts) == 3 {
			s = parts[2] + parts[1] + parts[0]
			num, _ := strconv.ParseInt(s, 10, 64)
			return num
		}
	}
	return 0
}

func RenderTableHTML(data *types.TableData, filepath string, tableIndex, page, size int, sortCol int, sortOrder string, searchQuery string) string {
	var html string

	totalPages := (data.Total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page-1)*size + 1
	end := start + len(data.Rows) - 1

	targetID := fmt.Sprintf("table-component-%d", tableIndex)

	// search is NOT included in baseParams — it is injected at request time
	// via hx-include so pagination and sort always reflect the live input value
	baseParams := fmt.Sprintf("filepath=%s&tableindex=%d&size=%d", filepath, tableIndex, size)
	if sortCol >= 0 {
		baseParams += fmt.Sprintf("&sort=%d&order=%s", sortCol, sortOrder)
	}
	searchInclude := fmt.Sprintf("#table-search-%d", tableIndex)

	ts := configmanager.GetTableSettings()

	html += fmt.Sprintf(`<div id="%s" class="table-container">`, targetID)

	if ts.ShowSearch {
		searchInputID := fmt.Sprintf("table-search-%d", tableIndex)
		html += `<div class="table-controls">`
		html += fmt.Sprintf(`
			<input type="text"
			       id="%s"
			       class="table-search"
			       placeholder="Search table..."
			       value="%s"
			       hx-get="/api/components/table?%s&page=1"
			       hx-trigger="keyup changed delay:300ms"
			       hx-target="#%s"
			       hx-swap="outerHTML"
			       hx-include="this"
			       hx-preserve
			       name="search">
		`, searchInputID, searchQuery, baseParams, targetID)
		// note: hx-include="this" sends name="search" — baseParams has no search param
		// hx-preserve keeps the existing DOM node on swap so focus/cursor are not lost
		html += `</div>`
	}

	html += `<div class="table-wrapper">`
	html += `<table>`

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

		headerParams := fmt.Sprintf("filepath=%s&tableindex=%d&size=%d&page=1&sort=%d&order=%s",
			filepath, tableIndex, size, header.ColumnIdx, nextOrder)

		html += fmt.Sprintf(`<th data-type="%s" data-align="%s" hx-get="/api/components/table?%s" hx-target="#%s" hx-swap="outerHTML" hx-include="%s" class="sortable">%s%s</th>`,
			header.DataType, header.Align, headerParams, targetID, searchInclude, header.Content, sortIndicator)
	}
	html += `</tr></thead>`

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
	html += `</div>`

	html += `<div class="table-footer">`

	html += fmt.Sprintf(`<a href="/files/edittable/%s?tableindex=%d" class="btn-table-edit"><i class="fa fa-edit"></i> %s</a>`,
		filepath, tableIndex, translation.SprintfForRequest(configmanager.GetLanguage(), "edit table"))

	html += `<div class="table-footer-right">`

	if ts.ShowInfo {
		if data.Total > 0 {
			html += fmt.Sprintf(`<span class="table-info">Showing %d-%d of %d rows</span>`, start, end, data.Total)
		} else {
			html += `<span class="table-info">No results found</span>`
		}
	}

	if ts.ShowPaging {
		html += `<div class="pagination">`

		if page > 1 {
			html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=1" hx-target="#%s" hx-swap="outerHTML" hx-include="%s" class="page-btn">First</button>`, baseParams, targetID, searchInclude)
		} else {
			html += `<button class="page-btn" disabled>First</button>`
		}

		if page > 1 {
			html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#%s" hx-swap="outerHTML" hx-include="%s" class="page-btn">Prev</button>`, baseParams, page-1, targetID, searchInclude)
		} else {
			html += `<button class="page-btn" disabled>Prev</button>`
		}

		html += fmt.Sprintf(`<span class="page-info">Page %d of %d</span>`, page, totalPages)

		if page < totalPages {
			html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#%s" hx-swap="outerHTML" hx-include="%s" class="page-btn">Next</button>`, baseParams, page+1, targetID, searchInclude)
		} else {
			html += `<button class="page-btn" disabled>Next</button>`
		}

		if page < totalPages {
			html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#%s" hx-swap="outerHTML" hx-include="%s" class="page-btn">Last</button>`, baseParams, totalPages, targetID, searchInclude)
		} else {
			html += `<button class="page-btn" disabled>Last</button>`
		}

		html += `</div>`
	}

	html += `</div>` // table-footer-right
	html += `</div>` // table-footer

	html += `</div>`

	return html
}
