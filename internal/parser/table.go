package parser

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TableData represents parsed table structure
type TableData struct {
	Headers []TableHeader
	Rows    [][]TableCell
	Total   int
}

// TableHeader represents a column header with metadata
type TableHeader struct {
	Content   string
	DataType  string
	Align     string
	Sortable  bool
	ColumnIdx int
}

// TableCell represents a single table cell with metadata
type TableCell struct {
	Content  string
	DataType string
	Align    string
	RawValue string
}

func PaginateTable(data *TableData, page, size int) *TableData {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 25
	}

	start := (page - 1) * size
	end := start + size

	if start >= len(data.Rows) {
		return &TableData{
			Headers: data.Headers,
			Rows:    [][]TableCell{},
			Total:   data.Total,
		}
	}

	if end > len(data.Rows) {
		end = len(data.Rows)
	}

	return &TableData{
		Headers: data.Headers,
		Rows:    data.Rows[start:end],
		Total:   data.Total,
	}
}

func SortTable(data *TableData, column int, order string) *TableData {
	if column < 0 || column >= len(data.Headers) {
		return data
	}

	sortedRows := make([][]TableCell, len(data.Rows))
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

	return &TableData{
		Headers: data.Headers,
		Rows:    sortedRows,
		Total:   data.Total,
	}
}

func SearchTable(data *TableData, query string) *TableData {
	if query == "" {
		return data
	}

	query = strings.ToLower(query)
	var filteredRows [][]TableCell

	for _, row := range data.Rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell.Content), query) {
				filteredRows = append(filteredRows, row)
				break
			}
		}
	}

	return &TableData{
		Headers: data.Headers,
		Rows:    filteredRows,
		Total:   len(filteredRows),
	}
}

func parseNumber(s string) float64 {
	s = regexp.MustCompile(`[$€£¥,\s]`).ReplaceAllString(s, "")
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

func RenderTableHTML(data *TableData, filepath string, page, size int, sortCol int, sortOrder string, searchQuery string) string {
	var html string

	totalPages := (data.Total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page-1)*size + 1
	end := start + len(data.Rows) - 1

	baseParams := fmt.Sprintf("filepath=%s&size=%d", filepath, size)
	if sortCol >= 0 {
		baseParams += fmt.Sprintf("&sort=%d&order=%s", sortCol, sortOrder)
	}
	if searchQuery != "" {
		baseParams += fmt.Sprintf("&search=%s", searchQuery)
	}

	html += `<div class="table-container">`
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

	html += `<div class="table-wrapper">`
	html += `<table class="dokuwiki-table" data-sortable="true" data-searchable="true">`

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

	html += `<div class="pagination">`

	if page > 1 {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=1" hx-target="#table-wrapper" class="page-btn">First</button>`, baseParams)
	} else {
		html += `<button class="page-btn" disabled>First</button>`
	}

	if page > 1 {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Prev</button>`, baseParams, page-1)
	} else {
		html += `<button class="page-btn" disabled>Prev</button>`
	}

	html += fmt.Sprintf(`<span class="page-info">Page %d of %d</span>`, page, totalPages)

	if page < totalPages {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Next</button>`, baseParams, page+1)
	} else {
		html += `<button class="page-btn" disabled>Next</button>`
	}

	if page < totalPages {
		html += fmt.Sprintf(`<button hx-get="/api/components/table?%s&page=%d" hx-target="#table-wrapper" class="page-btn">Last</button>`, baseParams, totalPages)
	} else {
		html += `<button class="page-btn" disabled>Last</button>`
	}

	html += `</div>`

	if data.Total > 0 {
		html += fmt.Sprintf(`<div class="table-info">Showing %d-%d of %d rows</div>`, start, end, data.Total)
	} else {
		html += `<div class="table-info">No results found</div>`
	}

	html += `</div>`

	return html
}
