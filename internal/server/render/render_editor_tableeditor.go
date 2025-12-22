// Package render - HTMX HTML rendering functions for table editor
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/translation"
)

// TableData represents a parsed markdown table for fg-grid
type TableData struct {
	Headers []string   `json:"headers"`
	Data    [][]string `json:"data"`
}

// ParseMarkdownTable extracts table data from markdown content
func ParseMarkdownTable(markdownContent string) (*TableData, error) {
	lines := strings.Split(markdownContent, "\n")

	var tableStart, tableEnd int = -1, -1

	// find table boundaries
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, "<!--") {
			if tableStart == -1 {
				tableStart = i
			}
			tableEnd = i
		} else if tableStart != -1 && trimmed == "" {
			break
		}
	}

	if tableStart == -1 {
		return nil, fmt.Errorf("no table found in markdown")
	}

	tableLines := lines[tableStart : tableEnd+1]

	// parse table
	var headers []string
	var data [][]string

	for i, line := range tableLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, "|") {
			continue
		}

		// skip separator line (usually contains --- or === )
		if strings.Contains(trimmed, "-") && strings.Count(trimmed, "-") > 2 {
			continue
		}

		// parse row
		cells := strings.Split(trimmed, "|")
		var cleanCells []string

		for _, cell := range cells {
			clean := strings.TrimSpace(cell)
			if clean != "" {
				cleanCells = append(cleanCells, clean)
			}
		}

		if len(cleanCells) == 0 {
			continue
		}

		if i == 0 || len(headers) == 0 {
			headers = cleanCells
		} else {
			data = append(data, cleanCells)
		}
	}

	return &TableData{
		Headers: headers,
		Data:    data,
	}, nil
}

// ConvertTableDataToMarkdown converts table data back to markdown format
func ConvertTableDataToMarkdown(tableData *TableData) string {
	if tableData == nil || len(tableData.Headers) == 0 {
		return ""
	}

	var md strings.Builder

	// write headers
	md.WriteString("|")
	for _, header := range tableData.Headers {
		md.WriteString(" ")
		md.WriteString(header)
		md.WriteString(" |")
	}
	md.WriteString("\n")

	// write separator
	md.WriteString("|")
	for range tableData.Headers {
		md.WriteString("---|")
	}
	md.WriteString("\n")

	// write data rows
	for _, row := range tableData.Data {
		md.WriteString("|")
		for i := range tableData.Headers {
			md.WriteString(" ")
			if i < len(row) {
				md.WriteString(row[i])
			}
			md.WriteString(" |")
		}
		md.WriteString("\n")
	}

	return md.String()
}

// ReplaceTableInMarkdown replaces the first table found in markdown with new table data
func ReplaceTableInMarkdown(originalMarkdown string, newTableData *TableData) string {
	if newTableData == nil {
		return originalMarkdown
	}

	lines := strings.Split(originalMarkdown, "\n")
	var tableStart, tableEnd int = -1, -1

	// find table boundaries
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, "<!--") {
			if tableStart == -1 {
				tableStart = i
			}
			tableEnd = i
		} else if tableStart != -1 && trimmed == "" {
			break
		}
	}

	if tableStart == -1 {
		// no table found, append at end
		return originalMarkdown + "\n\n" + ConvertTableDataToMarkdown(newTableData)
	}

	// replace existing table
	beforeTable := lines[:tableStart]
	afterTable := lines[tableEnd+1:]

	newTable := strings.TrimSuffix(ConvertTableDataToMarkdown(newTableData), "\n")

	var result strings.Builder

	// add content before table
	for i, line := range beforeTable {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(line)
	}

	// add new table
	if len(beforeTable) > 0 {
		result.WriteString("\n")
	}
	result.WriteString(newTable)

	// add content after table
	for _, line := range afterTable {
		result.WriteString("\n")
		result.WriteString(line)
	}

	return result.String()
}

// RenderTableEditor renders the complete table editor HTML
func RenderTableEditor(tableData *TableData) string {
	var html strings.Builder

	// main container
	html.WriteString(`<div class="table-editor">`)

	// toolbar
	html.WriteString(`<div class="table-toolbar">`)
	html.WriteString(`<button onclick="addRow()" class="btn-secondary" title="` + translation.SprintfForRequest(configmanager.GetLanguage(), "add row") + `">+ ` + translation.SprintfForRequest(configmanager.GetLanguage(), "row") + `</button>`)
	html.WriteString(`<button onclick="addColumn()" class="btn-secondary" title="` + translation.SprintfForRequest(configmanager.GetLanguage(), "add column") + `">+ ` + translation.SprintfForRequest(configmanager.GetLanguage(), "column") + `</button>`)
	html.WriteString(`<button onclick="deleteLastRow()" class="btn-secondary" title="` + translation.SprintfForRequest(configmanager.GetLanguage(), "delete last row") + `">- ` + translation.SprintfForRequest(configmanager.GetLanguage(), "row") + `</button>`)
	html.WriteString(`<button onclick="deleteLastColumn()" class="btn-secondary" title="` + translation.SprintfForRequest(configmanager.GetLanguage(), "delete last column") + `">- ` + translation.SprintfForRequest(configmanager.GetLanguage(), "column") + `</button>`)
	html.WriteString(`</div>`)

	// table container
	html.WriteString(`<div class="table-container">`)
	html.WriteString(`<table class="editable-table" id="data-table">`)

	// prepare data
	headers := tableData.Headers
	data := tableData.Data

	// ensure we have at least basic structure
	if len(headers) == 0 {
		headers = []string{translation.SprintfForRequest(configmanager.GetLanguage(), "column 1")}
	}
	if len(data) == 0 {
		data = [][]string{{""}}
	}

	// headers
	html.WriteString(`<thead><tr>`)
	for index, header := range headers {
		colText := translation.SprintfForRequest(configmanager.GetLanguage(), "column")
		html.WriteString(fmt.Sprintf(`<th contenteditable="true" data-col="%d" placeholder="%s %d">%s</th>`,
			index, colText, index+1, header))
	}
	html.WriteString(`</tr></thead>`)

	// data rows
	html.WriteString(`<tbody>`)
	for rowIndex, row := range data {
		html.WriteString(`<tr>`)
		for colIndex := range headers {
			cellValue := ""
			if colIndex < len(row) {
				cellValue = row[colIndex]
			}
			html.WriteString(fmt.Sprintf(`<td contenteditable="true" data-row="%d" data-col="%d" placeholder="...">%s</td>`,
				rowIndex, colIndex, cellValue))
		}
		html.WriteString(`</tr>`)
	}
	html.WriteString(`</tbody>`)

	html.WriteString(`</table>`)
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)

	return html.String()
}

// RenderTableEditorJS renders the JavaScript functions for table editing
func RenderTableEditorJS(filePath string) string {
	var js strings.Builder

	js.WriteString(fmt.Sprintf(`
let filePath = '%s';

// table editor is already loaded - no need to fetch
document.addEventListener('DOMContentLoaded', function() {
    console.log('table editor ready');
    setupKeyboardNavigation();
});

function setupKeyboardNavigation() {
    const table = document.getElementById('data-table');
    if (!table) return;

    table.addEventListener('keydown', function(e) {
        if (e.key === 'Tab') {
            e.preventDefault();
            const current = e.target;
            if (current.tagName === 'TD' || current.tagName === 'TH') {
                const cells = Array.from(table.querySelectorAll('td, th'));
                const currentIndex = cells.indexOf(current);
                const nextIndex = e.shiftKey ? currentIndex - 1 : currentIndex + 1;

                if (nextIndex >= 0 && nextIndex < cells.length) {
                    cells[nextIndex].focus();
                }
            }
        } else if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            const current = e.target;
            if (current.tagName === 'TD') {
                const currentRow = parseInt(current.dataset.row);
                const currentCol = parseInt(current.dataset.col);
                const nextCell = table.querySelector('td[data-row="' + (currentRow + 1) + '"][data-col="' + currentCol + '"]');
                if (nextCell) {
                    nextCell.focus();
                }
            }
        }
    });
}

function saveTable() {
    let tableData = { headers: [], data: [] };

    try {
        const table = document.querySelector('#data-table');
        if (table) {
            const headerCells = table.querySelectorAll('thead th');
            const bodyRows = table.querySelectorAll('tbody tr');

            tableData.headers = Array.from(headerCells).map(cell => cell.textContent || '');
            tableData.data = Array.from(bodyRows).map(row => {
                const cells = row.querySelectorAll('td');
                return Array.from(cells).map(cell => cell.textContent || '');
            });
        }

        // ensure we have valid data
        if (!tableData.headers || tableData.headers.length === 0) {
            tableData.headers = ['%s'];
        }
        if (!tableData.data || tableData.data.length === 0) {
            tableData.data = [['']];
        }

        // clean data
        tableData.headers = tableData.headers.map(h => String(h || '').trim());
        tableData.data = tableData.data.map(row => {
            if (Array.isArray(row)) {
                return row.map(cell => String(cell || '').trim());
            }
            return [''];
        });

        console.log('saving table data:', tableData);

        const formData = new FormData();
        formData.append('filepath', filePath);
        formData.append('tableData', JSON.stringify(tableData));

        // show loading
        document.getElementById('table-editor-status').innerHTML =
            '<div class="status-info">%s...</div>';

        fetch('/api/editor/tableeditor', {
            method: 'POST',
            body: formData
        })
        .then(response => response.text())
        .then(result => {
            document.getElementById('table-editor-status').innerHTML = result;
            // redirect to file view after successful save
            if (result.includes('status-ok') || result.includes('status-success')) {
                setTimeout(() => {
                    window.location.href = '/files/' + filePath;
                }, 1500);
            }
        })
        .catch(error => {
            console.error('error saving table:', error);
            document.getElementById('table-editor-status').innerHTML =
                '<div class="status-error">%s: ' + error.message + '</div>';
        });
    } catch (error) {
        console.error('error getting table data:', error);
        document.getElementById('table-editor-status').innerHTML =
            '<div class="status-error">%s: ' + error.message + '</div>';
    }
}

// table manipulation functions
function addRow() {
    const table = document.getElementById('data-table');
    const tbody = table?.querySelector('tbody');
    const headerCount = table?.querySelectorAll('thead th').length || 1;

    if (tbody) {
        const newRow = document.createElement('tr');
        const rowIndex = tbody.children.length;

        for (let i = 0; i < headerCount; i++) {
            const cell = document.createElement('td');
            cell.contentEditable = true;
            cell.setAttribute('data-row', rowIndex);
            cell.setAttribute('data-col', i);
            cell.setAttribute('placeholder', '...');
            newRow.appendChild(cell);
        }
        tbody.appendChild(newRow);

        // focus first cell of new row
        newRow.firstElementChild?.focus();
    }
}

function addColumn() {
    const table = document.getElementById('data-table');
    if (!table) return;

    // add header
    const headerRow = table.querySelector('thead tr');
    if (headerRow) {
        const colIndex = headerRow.children.length;
        const newHeader = document.createElement('th');
        newHeader.contentEditable = true;
        newHeader.setAttribute('data-col', colIndex);
        newHeader.setAttribute('placeholder', '%s ' + (colIndex + 1));
        newHeader.textContent = '%s ' + (colIndex + 1);
        headerRow.appendChild(newHeader);
    }

    // add cells to existing data rows
    const bodyRows = table.querySelectorAll('tbody tr');
    bodyRows.forEach((row, rowIndex) => {
        const colIndex = headerRow?.children.length - 1 || 0;
        const newCell = document.createElement('td');
        newCell.contentEditable = true;
        newCell.setAttribute('data-row', rowIndex);
        newCell.setAttribute('data-col', colIndex);
        newCell.setAttribute('placeholder', '...');
        row.appendChild(newCell);
    });
}

function deleteLastRow() {
    const table = document.getElementById('data-table');
    const tbody = table?.querySelector('tbody');

    if (tbody && tbody.children.length > 1) {
        tbody.removeChild(tbody.lastElementChild);
    }
}

function deleteLastColumn() {
    const table = document.getElementById('data-table');
    if (!table) return;

    const headerRow = table.querySelector('thead tr');
    const bodyRows = table.querySelectorAll('tbody tr');

    if (headerRow && headerRow.children.length > 1) {
        // remove header
        headerRow.removeChild(headerRow.lastElementChild);

        // remove last cell from each data row
        bodyRows.forEach(row => {
            if (row.lastElementChild) {
                row.removeChild(row.lastElementChild);
            }
        });
    }
}

function cancelTableEdit() {
    window.location.href = '/files/' + filePath;
}
`,
		filePath,
		translation.SprintfForRequest(configmanager.GetLanguage(), "column 1"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "saving table"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save table"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get table data"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "column"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "column")))

	return js.String()
}
