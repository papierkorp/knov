// Package render - table editor rendering
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/translation"
)

// TableData represents the structure for Handsontable
type TableData struct {
	Headers    []string   `json:"headers"`
	Rows       [][]string `json:"rows"`
	TableIndex int        `json:"tableIndex"`
}

// RenderTableEditorForm renders the complete table editor form
func RenderTableEditorForm(filePath string) string {
	fullPath := contentStorage.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filePath, err)
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
	}

	// extract first table from markdown
	tableData, tableIndex := extractTableFromMarkdown(string(content))
	if tableData == nil {
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no table found in file"))
	}

	tableData.TableIndex = tableIndex

	// convert to JSON
	tableJSON, err := json.Marshal(tableData)
	if err != nil {
		logging.LogError("failed to marshal table data: %v", err)
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to process table"))
	}

	html := fmt.Sprintf(`
<div class="table-editor-toolbar">
	<button type="button" onclick="saveTable()" class="btn-primary">
		<i class="fa fa-save"></i> %s
	</button>
	<button type="button" onclick="cancelTableEdit()" class="btn-secondary">
		%s
	</button>
</div>
<div id="table-editor-container">
	<div id="handsontable-container"></div>
</div>
<div id="table-editor-status"></div>

<script>
const tableData = %s;
const filePath = '%s';

const container = document.getElementById('handsontable-container');
const hot = new Handsontable(container, {
	data: tableData.rows,
	colHeaders: tableData.headers,
	rowHeaders: true,
	contextMenu: true,
	manualRowMove: true,
	manualColumnMove: true,
	navigableHeaders: true,
	tabNavigation: true,
	multiColumnSorting: true,
	headerClassName: 'htLeft',
	themeName: 'ht-theme-main-dark-auto',
	licenseKey: 'non-commercial-and-evaluation',
	minSpareRows: 1,
	height: 'auto',
	width: '100%%',
	cells: function(row, col) {
		return {
			readOnly: false,
		};
	},
	afterChange: function(changes, source) {
		if (source !== 'loadData') {
			console.log('table changed');
		}
	}
});

hot.addHook('afterOnCellMouseDown', function (event, coords, TD) {
  if (coords.row === -1) {
    // Column header row
    if (event.detail === 2) {
      // Double click
      const colIndex = coords.col;
      const currentHeader = hot.getColHeader(colIndex);
      const newHeader = prompt('Edit column header:', currentHeader);

      if (newHeader !== null && newHeader !== currentHeader && newHeader.trim() !== '') {
        // Get current headers
        const currentHeaders = hot.getSettings().colHeaders;
        const newHeaders = [...currentHeaders]; // Create a copy
        newHeaders[colIndex] = newHeader;

        // Update the table with new headers
        hot.updateSettings({ colHeaders: newHeaders });

        // Also update the tableData.headers to keep in sync
        tableData.headers = newHeaders;
      }
    }
  }
});


function saveTable() {
	const data = hot.getSourceData();
	const headers = tableData.headers;
	const tableIndex = tableData.tableIndex;

	const formData = new FormData();
	formData.append('filepath', filePath);
	formData.append('headers', JSON.stringify(headers));
	formData.append('rows', JSON.stringify(data));
	formData.append('tableIndex', tableIndex.toString());

	fetch('/api/editor/tableeditor', {
		method: 'POST',
		body: formData
	})
	.then(response => response.text())
	.then(html => {
		document.getElementById('table-editor-status').innerHTML = html;
	})
	.catch(error => {
		document.getElementById('table-editor-status').innerHTML =
			'<div class="status-error">%s: ' + error + '</div>';
	});
}

function cancelTableEdit() {
	window.location.href = '/files/' + filePath;
}
</script>
`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		string(tableJSON),
		jsEscape(filePath),
		translation.SprintfForRequest(configmanager.GetLanguage(), "error saving table"),
	)

	return html
}

// ReplaceTableInMarkdown replaces a table in markdown content
func ReplaceTableInMarkdown(content string, headers []string, rows [][]string, tableIndex int) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inTable bool
	var currentTable int
	var tableStartIdx int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect table
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "-") && !inTable {
			currentTable++
			if currentTable-1 == tableIndex {
				inTable = true
				tableStartIdx = i - 1 // include header
				continue
			}
		}

		if inTable {
			if strings.HasPrefix(trimmed, "|") {
				continue // skip table rows
			} else {
				// table ended, insert new table
				newTable := generateMarkdownTable(headers, rows)
				result = append(result[:tableStartIdx], newTable...)
				result = append(result, line)
				inTable = false
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// extractTableFromMarkdown extracts the first markdown table
func extractTableFromMarkdown(content string) (*TableData, int) {
	lines := strings.Split(content, "\n")
	var tableLines []string
	var inTable bool
	var tableIndex int
	currentTable := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect table start (header separator line)
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "-") && !inTable {
			inTable = true
			currentTable++
			// add the header line (previous line)
			if len(tableLines) > 0 {
				tableLines = tableLines[len(tableLines)-1:]
			}
			tableLines = append(tableLines, line)
			tableIndex = currentTable - 1
			continue
		}

		if inTable {
			if strings.HasPrefix(trimmed, "|") {
				tableLines = append(tableLines, line)
			} else {
				// table ended
				break
			}
		} else if strings.HasPrefix(trimmed, "|") {
			// potential table header
			tableLines = []string{line}
		}
	}

	if len(tableLines) < 2 {
		return nil, 0
	}

	return parseMarkdownTable(tableLines), tableIndex
}

// parseMarkdownTable parses markdown table lines into TableData
func parseMarkdownTable(lines []string) *TableData {
	if len(lines) < 2 {
		return nil
	}

	// parse header
	headerLine := strings.Trim(lines[0], " ")
	headers := parseTableRow(headerLine)

	// skip separator line (index 1)
	var rows [][]string
	for i := 2; i < len(lines); i++ {
		row := parseTableRow(lines[i])
		if len(row) > 0 {
			// pad or trim to match header length
			for len(row) < len(headers) {
				row = append(row, "")
			}
			if len(row) > len(headers) {
				row = row[:len(headers)]
			}
			rows = append(rows, row)
		}
	}

	return &TableData{
		Headers: headers,
		Rows:    rows,
	}
}

// parseTableRow parses a single markdown table row
func parseTableRow(line string) []string {
	// remove leading/trailing pipes and whitespace
	line = strings.Trim(line, " |")

	// split by pipe
	cells := strings.Split(line, "|")

	var result []string
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}

	return result
}

// generateMarkdownTable creates markdown table from data
func generateMarkdownTable(headers []string, rows [][]string) []string {
	var lines []string

	// header row
	headerRow := "| " + strings.Join(headers, " | ") + " |"
	lines = append(lines, headerRow)

	// separator row
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	sepRow := "| " + strings.Join(separators, " | ") + " |"
	lines = append(lines, sepRow)

	// data rows
	for _, row := range rows {
		// ensure row matches header length
		for len(row) < len(headers) {
			row = append(row, "")
		}
		if len(row) > len(headers) {
			row = row[:len(headers)]
		}

		// skip empty rows
		allEmpty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		dataRow := "| " + strings.Join(row, " | ") + " |"
		lines = append(lines, dataRow)
	}

	return lines
}

// jsEscape escapes a string for use in JavaScript
func jsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}
