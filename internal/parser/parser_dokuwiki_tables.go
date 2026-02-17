package parser

import (
	"fmt"
	"strings"

	"knov/internal/types"
)

// renderTableAsMarkdown converts table element to Markdown table
func (h *DokuwikiHandler) renderTableAsMarkdown(element DokuWikiElement) string {
	var result []string

	// If we have headers, render them
	if len(element.Headers) > 0 {
		headerRow := "| " + strings.Join(element.Headers, " | ") + " |"
		result = append(result, headerRow)

		// Add separator row
		separator := "|"
		for range element.Headers {
			separator += " --- |"
		}
		result = append(result, separator)
	}

	// Render data rows
	for _, row := range element.Rows {
		if len(row) > 0 {
			rowStr := "| " + strings.Join(row, " | ") + " |"
			result = append(result, rowStr)
		}
	}

	// If no headers and we have rows, add a separator after first row
	if len(element.Headers) == 0 && len(element.Rows) > 0 {
		if len(element.Rows) == 1 {
			// Single row, treat first row as header
			separator := "|"
			for range element.Rows[0] {
				separator += " --- |"
			}
			result = append(result, separator)
		} else {
			// Multiple rows, add separator after first row
			separator := "|"
			for range element.Rows[0] {
				separator += " --- |"
			}
			result = []string{result[0], separator}
			result = append(result, result[2:]...)
		}
	}

	return strings.Join(result, "\n")
}

// renderTableAsHTML converts table element to HTML table
func (h *DokuwikiHandler) renderTableAsHTML(element DokuWikiElement) string {
	var result []string
	result = append(result, "<table>")

	// If we have headers, render them
	if len(element.Headers) > 0 {
		result = append(result, "  <thead>")
		result = append(result, "    <tr>")
		for _, header := range element.Headers {
			result = append(result, fmt.Sprintf("      <th>%s</th>", header))
		}
		result = append(result, "    </tr>")
		result = append(result, "  </thead>")
	}

	// Render data rows
	if len(element.Rows) > 0 {
		result = append(result, "  <tbody>")
		for _, row := range element.Rows {
			result = append(result, "    <tr>")
			for _, cell := range row {
				result = append(result, fmt.Sprintf("      <td>%s</td>", cell))
			}
			result = append(result, "    </tr>")
		}
		result = append(result, "  </tbody>")
	}

	result = append(result, "</table>")
	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------------

// ProcessTables handles table conversion for both HTML and Markdown
func (h *DokuwikiHandler) ProcessTables(content string, outputFormat string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var tableLines []string
	var inTable bool

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "^") || strings.HasPrefix(trimmedLine, "|") {
			if !inTable {
				inTable = true
				tableLines = []string{}
			}
			tableLines = append(tableLines, line)
		} else {
			if inTable {
				// Process the complete table
				tableElement := h.parseTableLines(tableLines)
				result = append(result, h.renderElement(tableElement, outputFormat))
				inTable = false
				tableLines = []string{}
			}
			result = append(result, line)
		}
	}

	// Handle table at end of content
	if inTable {
		tableElement := h.parseTableLines(tableLines)
		result = append(result, h.renderElement(tableElement, outputFormat))
	}

	return strings.Join(result, "\n")
}

// parseTableLines converts table lines into a DokuWikiElement
func (h *DokuwikiHandler) parseTableLines(lines []string) DokuWikiElement {
	var headers []string
	var rows [][]string
	var firstRow = true

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Remove leading/trailing delimiters
		line = strings.TrimPrefix(line, "^")
		line = strings.TrimPrefix(line, "|")
		line = strings.TrimSuffix(line, "^")
		line = strings.TrimSuffix(line, "|")

		cells := h.splitMixedDelimiters(line)

		// Clean up cells
		for i, cell := range cells {
			cells[i] = strings.TrimSpace(cell)
		}

		// First row or header row (if it contains ^ delimiters)
		if firstRow && h.isMajorityHeaderCells(line) {
			headers = cells
			firstRow = false
		} else {
			rows = append(rows, cells)
			if firstRow {
				firstRow = false
			}
		}
	}

	return DokuWikiElement{
		Type:    "table",
		Headers: headers,
		Rows:    rows,
	}
}

// splitMixedDelimiters splits table cells by ^ or | delimiters
func (h *DokuwikiHandler) splitMixedDelimiters(line string) []string {
	var cells []string
	var currentCell strings.Builder

	for _, char := range line {
		if char == '^' || char == '|' {
			if currentCell.Len() > 0 || len(cells) > 0 {
				cells = append(cells, currentCell.String())
				currentCell.Reset()
			}
		} else {
			currentCell.WriteRune(char)
		}
	}

	if currentCell.Len() > 0 {
		cells = append(cells, currentCell.String())
	}

	return cells
}

// isMajorityHeaderCells determines if a line is primarily header cells (^)
func (h *DokuwikiHandler) isMajorityHeaderCells(line string) bool {
	headerCount := strings.Count(line, "^")
	normalCount := strings.Count(line, "|")
	return headerCount > normalCount
}

// GetFirstTable extracts the first table from content as TableData
func (h *DokuwikiHandler) GetFirstTable(content string) (*types.TableData, error) {
	lines := strings.Split(content, "\n")
	var tableLines []string

	// find first table lines
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "^") || strings.HasPrefix(trimmedLine, "|") {
			tableLines = append(tableLines, line)
		} else if len(tableLines) > 0 {
			// end of first table
			break
		}
	}

	if len(tableLines) == 0 {
		return &types.TableData{
			Headers: []types.TableHeader{},
			Rows:    [][]types.TableCell{},
			Total:   0,
		}, nil
	}

	element := h.parseTableLines(tableLines)

	// convert to TableData structure
	var headers []types.TableHeader
	for i, header := range element.Headers {
		headers = append(headers, types.TableHeader{
			Content:   header,
			DataType:  "text",
			Align:     "left",
			Sortable:  true,
			ColumnIdx: i,
		})
	}

	var rows [][]types.TableCell
	for _, row := range element.Rows {
		var cells []types.TableCell
		for _, cell := range row {
			cells = append(cells, types.TableCell{
				Content:  cell,
				DataType: "text",
				Align:    "left",
				RawValue: cell,
			})
		}
		rows = append(rows, cells)
	}

	return &types.TableData{
		Headers: headers,
		Rows:    rows,
		Total:   len(rows),
	}, nil
}
