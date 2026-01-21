// Package render - table editor rendering
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/logging"
	"knov/internal/translation"
	"knov/internal/types"
)

// RenderTableEditorForm renders the complete table editor form
func RenderTableEditorForm(filePath string, tableIndex int) string {
	// extract table from markdown using contenthandler
	handler := contentHandler.GetHandler("markdown")
	headers, rows, err := handler.ExtractTable(filePath, tableIndex)
	if err != nil {
		logging.LogError("failed to extract table from file %s: %v", filePath, err)
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no table found in file"))
	}

	tableData := &types.SimpleTableData{
		Headers:    headers,
		Rows:       rows,
		Total:      len(rows),
		TableIndex: tableIndex,
	}

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

// jsEscape escapes a string for use in JavaScript
func jsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}
