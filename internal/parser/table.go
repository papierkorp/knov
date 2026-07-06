package parser

import (
	"fmt"
	htmlescape "html"
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

// FilterTable returns only the rows whose value in the given column matches
// value exactly (case-insensitive). An empty value or out-of-range column
// leaves the data unchanged.
func FilterTable(data *types.TableData, column int, value string) *types.TableData {
	if value == "" || column < 0 || column >= len(data.Headers) {
		return data
	}

	value = strings.ToLower(value)
	var filteredRows [][]types.TableCell

	for _, row := range data.Rows {
		if column >= len(row) {
			continue
		}
		if strings.ToLower(stripHTMLTags(row[column].RawValue)) == value {
			filteredRows = append(filteredRows, row)
		}
	}

	return &types.TableData{
		Headers: data.Headers,
		Rows:    filteredRows,
		Total:   len(filteredRows),
	}
}

// columnFilterNumericStrip matches currency symbols, thousands separators and
// whitespace so numeric-looking cell values can be recognized regardless of
// formatting (e.g. "1.500,50 €").
var columnFilterNumericStrip = regexp.MustCompile(`[$€£¥,\s]`)

// isNumericValue reports whether s looks like a number once common currency
// formatting is stripped.
func isNumericValue(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(columnFilterNumericStrip.ReplaceAllString(s, ""), 64)
	return err == nil
}

// ColumnFilterValues returns the sorted, lowercased, de-duplicated set of
// values found in the given column of the (unfiltered) table. It returns nil
// when the column isn't worth offering as a filter:
//   - there are fewer than 2 distinct values (nothing to group),
//   - the column is mostly numeric (quantities/prices are better sorted than
//     filtered by exact value),
//   - or most values are unique, e.g. free-text descriptions, where a filter
//     wouldn't meaningfully narrow anything down.
func ColumnFilterValues(data *types.TableData, column int) []string {
	if column < 0 || column >= len(data.Headers) {
		return nil
	}

	seen := make(map[string]bool)
	var values []string
	numericCount := 0
	nonEmptyCount := 0

	for _, row := range data.Rows {
		if column >= len(row) {
			continue
		}
		raw := strings.TrimSpace(stripHTMLTags(row[column].RawValue))
		if raw == "" {
			continue
		}
		nonEmptyCount++
		if isNumericValue(raw) {
			numericCount++
		}

		v := strings.ToLower(raw)
		if seen[v] {
			continue
		}
		seen[v] = true
		values = append(values, v)
	}

	if nonEmptyCount == 0 || len(values) <= 1 {
		return nil
	}
	if float64(numericCount)/float64(nonEmptyCount) > 0.8 {
		return nil
	}
	if float64(len(values))/float64(nonEmptyCount) > 0.5 {
		return nil
	}

	sort.Strings(values)
	return values
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

func RenderTableHTML(data, fullData *types.TableData, filepath string, tableIndex, page, size int, sortCol int, sortOrder string, searchQuery string, activeFilters map[int]string) string {
	var html string

	totalPages := (data.Total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page-1)*size + 1
	end := start + len(data.Rows) - 1

	targetID := fmt.Sprintf("table-component-%d", tableIndex)

	// search and filters are NOT included in baseParams — they are injected at
	// request time via hx-include so pagination and sort always reflect the
	// live input values
	baseParams := fmt.Sprintf("filepath=%s&tableindex=%d&size=%d", filepath, tableIndex, size)
	if sortCol >= 0 {
		baseParams += fmt.Sprintf("&sort=%d&order=%s", sortCol, sortOrder)
	}
	// includes the search input plus every column filter select, all of which
	// live inside the table container and carry their own name attribute
	searchInclude := fmt.Sprintf("#%s [name]", targetID)

	// compute which columns are eligible for a filter dropdown, based on the
	// full (unfiltered) table so the option list stays stable while filtering
	var filterOptions [][]string
	hasAnyFilter := false
	if configmanager.ShowColumnFilters.Get() {
		filterOptions = make([][]string, len(data.Headers))
		for i, header := range data.Headers {
			values := ColumnFilterValues(fullData, header.ColumnIdx)
			filterOptions[i] = values
			if values != nil {
				hasAnyFilter = true
			}
		}
	}

	html += fmt.Sprintf(`<div id="%s" class="table-container">`, targetID)

	if configmanager.ShowSearch.Get() {
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
			       hx-include="%s"
			       hx-preserve
			       name="search">
		`, searchInputID, searchQuery, baseParams, targetID, searchInclude)
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
	html += `</tr>`

	if hasAnyFilter {
		html += `<tr class="table-filter-row">`
		for i, header := range data.Headers {
			values := filterOptions[i]
			if values == nil {
				html += `<th></th>`
				continue
			}

			current := activeFilters[header.ColumnIdx]

			selectClass := "table-filter"
			if current != "" {
				selectClass += " table-filter-active"
			}

			html += fmt.Sprintf(`<th><select name="filter" class="%s" hx-get="/api/components/table?%s&page=1" hx-target="#%s" hx-swap="outerHTML" hx-include="%s">`,
				selectClass, baseParams, targetID, searchInclude)

			allSelected := ""
			if current == "" {
				allSelected = " selected"
			}
			html += fmt.Sprintf(`<option value=""%s>%s</option>`, allSelected,
				translation.SprintfForRequest(configmanager.GetLanguage(), "all"))

			for _, value := range values {
				selected := ""
				if current == value {
					selected = " selected"
				}
				escaped := htmlescape.EscapeString(value)
				html += fmt.Sprintf(`<option value="%d:%s"%s>%s</option>`, header.ColumnIdx, escaped, selected, escaped)
			}

			html += `</select></th>`
		}
		html += `</tr>`
	}

	html += `</thead>`

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

	if configmanager.ShowInfo.Get() {
		if data.Total > 0 {
			html += fmt.Sprintf(`<span class="table-info">Showing %d-%d of %d rows</span>`, start, end, data.Total)
		} else {
			html += `<span class="table-info">No results found</span>`
		}
	}

	if configmanager.ShowPaging.Get() {
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
