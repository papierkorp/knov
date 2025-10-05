// Package parser handles table parsing and manipulation
package parser

import (
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

// PaginateTable returns subset of rows for current page
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

// SortTable sorts rows by column index and order
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

// SearchTable filters rows containing search term
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
