package server

import (
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/types"
)

// @Summary Get paginated table
// @Description Returns paginated, sortable, searchable table HTML fragment for a markdown file
// @Tags components
// @Param filepath query string true "File path"
// @Param tableindex query int false "Table index (0-based)" default(0)
// @Param page query int false "Page number" default(1)
// @Param size query int false "Items per page" default(25)
// @Param sort query int false "Column index to sort by" default(-1)
// @Param order query string false "Sort order (asc/desc)" default(asc)
// @Param search query string false "Search query"
// @Param filter query []string false "Column filter, repeatable, format '<columnIndex>:<value>'"
// @Produce text/html
// @Success 200 {string} string "table html fragment"
// @Failure 400 {string} string "invalid parameters"
// @Failure 500 {string} string "failed to process table"
// @Router /api/components/table [get]
func handleAPIGetTable(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath parameter required"))
		return
	}

	tableIndex := 0
	if idxStr := r.URL.Query().Get("tableindex"); idxStr != "" {
		if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 {
			tableIndex = idx
		}
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	size := configmanager.GetTablePageSize()
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			size = s
		}
	}

	sortCol := -1
	if sortStr := r.URL.Query().Get("sort"); sortStr != "" {
		if s, err := strconv.Atoi(sortStr); err == nil && s >= 0 {
			sortCol = s
		}
	}

	sortOrder := "asc"
	if order := r.URL.Query().Get("order"); order == "desc" {
		sortOrder = "desc"
	}

	searchQuery := r.URL.Query().Get("search")

	activeFilters := map[int]string{}
	for _, raw := range r.URL.Query()["filter"] {
		colStr, value, ok := strings.Cut(raw, ":")
		if !ok || value == "" {
			continue
		}
		col, err := strconv.Atoi(colStr)
		if err != nil || col < 0 {
			continue
		}
		activeFilters[col] = value
	}

	handler := contentHandler.GetHandler("markdown")
	headers, rows, err := handler.ExtractTable(filepath, tableIndex)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to extract table from %s: %v", filepath, err)
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "no table found in file"))
		return
	}

	// keep a reference to the full, unfiltered table so filter dropdown
	// options stay complete regardless of the currently active search/filters
	fullTableData := simpleToTableData(headers, rows)

	tableData := fullTableData

	if searchQuery != "" {
		tableData = parser.SearchTable(tableData, searchQuery)
	}

	for col, value := range activeFilters {
		tableData = parser.FilterTable(tableData, col, value)
	}

	if sortCol >= 0 {
		tableData = parser.SortTable(tableData, sortCol, sortOrder)
	}

	paginatedData := parser.PaginateTable(tableData, page, size)

	html := render.RenderTableComponent(paginatedData, fullTableData, filepath, tableIndex, page, size, sortCol, sortOrder, searchQuery, activeFilters)

	writeResponse(w, r, nil, html)
}

// simpleToTableData converts plain string table data into the typed TableData structure
// used by the sort/search/paginate helpers.
func simpleToTableData(headers []string, rows [][]string) *types.TableData {
	tHeaders := make([]types.TableHeader, len(headers))
	for i, h := range headers {
		tHeaders[i] = types.TableHeader{Content: parser.RenderInlineMarkdown(h), DataType: "text", Align: "left", Sortable: true, ColumnIdx: i}
	}
	tRows := make([][]types.TableCell, len(rows))
	for i, row := range rows {
		tRow := make([]types.TableCell, len(row))
		for j, cell := range row {
			tRow[j] = types.TableCell{Content: parser.RenderInlineMarkdown(cell), DataType: "text", Align: "left", RawValue: cell}
		}
		tRows[i] = tRow
	}
	return &types.TableData{Headers: tHeaders, Rows: tRows, Total: len(rows)}
}
