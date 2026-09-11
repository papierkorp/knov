// Package render - table editor rendering
package render

import (
	"encoding/json"
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/translation"
	"knov/internal/types"
)

// RenderTableEditorForm renders the complete table editor form
func RenderTableEditorForm(filePath string, tableIndex int) string {
	// extract table from markdown using contenthandler
	handler := contentHandler.GetHandler("markdown")
	headers, rows, err := handler.ExtractTable(filePath, tableIndex)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to extract table from file %s: %v", filePath, err)
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
		logging.LogError(logging.KeyApp, "failed to marshal table data: %v", err)
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to process table"))
	}

	// build return URL including the header anchor so cancel/save land in the right spot
	returnURL := fmt.Sprintf("/files/%s", filePath)
	if anchor := contentHandler.FindMarkdownTableAnchor(filePath, tableIndex); anchor != "" {
		returnURL += "#" + anchor
	}

	downloadName := pathutils.BaseWithoutExt(filePath) + ".csv"

	html := fmt.Sprintf(`
<div class="table-editor-toolbar">
	<button type="button" onclick="saveTable()" class="btn-primary">
		<i class="fa fa-save"></i> %s
	</button>
	<button type="button" onclick="cancelTableEdit()" class="btn-secondary">
		%s
	</button>
	<button type="button" onclick="downloadTable()" class="btn-secondary">
		<i class="fa fa-download"></i> %s
	</button>
</div>
<div id="table-editor-container">
	<div id="tabulator-container"></div>
</div>
<div id="table-editor-status"></div>

<script>
const tableData = %s;
const filePath = %s;
const returnURL = %s;
const downloadFilename = %s;

const container = document.getElementById('tabulator-container');

function computeTableHeight() {
	const scrollParent = container.closest('main') || document.documentElement;
	let paddingBottomSum = 0;
	for (let el = container.parentElement; el; el = el.parentElement) {
		paddingBottomSum += parseFloat(getComputedStyle(el).paddingBottom) || 0;
		if (el === scrollParent) {
			break;
		}
	}
	const availableBottom = scrollParent.getBoundingClientRect().bottom - paddingBottomSum;
	// subtract a couple px to absorb sub-pixel layout rounding, which would
	// otherwise leave a hairline scrollbar on the scroll parent
	return availableBottom - container.getBoundingClientRect().top - 2;
}

function headerEditIcon() {
	return '<i class="fa fa-pen table-editor-rename-header"></i>';
}

// binds the rename icon's click directly, stopping propagation so it
// doesn't also trigger Tabulator's built-in header-click sort
function makeTitleFormatter(title) {
	return function(cell, formatterParams, onRendered) {
		onRendered(function() {
			const icon = cell.getElement().querySelector('.table-editor-rename-header');
			if (icon) {
				icon.addEventListener('click', function(e) {
					e.stopPropagation();
					openHeaderRenameInput(cell.getColumn());
				});
			}
		});
		return title + headerEditIcon();
	};
}

function buildColumns(headers) {
	return headers.map(function(header, index) {
		return {
			title: header,
			field: 'col' + index,
			editor: 'input',
			headerSort: true,
			titleFormatter: makeTitleFormatter(header),
		};
	});
}

function rowsToObjects(headers, rows) {
	return rows.map(function(row) {
		const obj = {};
		headers.forEach(function(header, index) {
			obj['col' + index] = row[index] !== undefined ? row[index] : '';
		});
		return obj;
	});
}

// table.getColumns() includes the rowHeader (blank field) once it has
// rowHandle set, so data columns must be filtered by a real field name
function dataColumns() {
	return table.getColumns().filter(function(c) { return c.getField(); });
}

function emptyRowData() {
	const obj = {};
	dataColumns().forEach(function(c) { obj[c.getField()] = ''; });
	return obj;
}

let nextColIndex = tableData.headers.length;

function insertColumn(column, before) {
	const field = 'col' + (nextColIndex++);
	const title = %s;
	table.addColumn({
		title: title,
		field: field,
		editor: 'input',
		headerSort: true,
		titleFormatter: makeTitleFormatter(title),
	}, before, column.getField());
}

const table = new Tabulator(container, {
	data: rowsToObjects(tableData.headers, tableData.rows).concat([{}]),
	columns: buildColumns(tableData.headers),
	layout: 'fitDataStretch',
	height: computeTableHeight(),
	movableRows: true,
	movableColumns: true,
	editTriggerEvent: 'dblclick',
	headerSortClickElement: 'icon',
	rowHeader: { headerSort: false, resizable: false, frozen: true, minWidth: 30, width: 30, hozAlign: 'center', formatter: 'rownum', editor: false, rowHandle: true },
	history: true,
	clipboard: true,
	selectableRange: true,
	selectableRangeColumns: true,
	selectableRangeRows: true,
	selectableRangeClearCells: true,
	rowContextMenu: [
		{ label: %s, action: function(e, row) { table.addRow(emptyRowData(), true, row); } },
		{ label: %s, action: function(e, row) { table.addRow(emptyRowData(), false, row); } },
		{ separator: true },
		{ label: %s, action: function(e, row) { row.delete(); } },
	],
	columnDefaults: {
		headerContextMenu: [
			{ label: %s, action: function(e, column) { insertColumn(column, true); } },
			{ label: %s, action: function(e, column) { insertColumn(column, false); } },
			{ separator: true },
			{ label: %s, action: function(e, column) { column.delete(); } },
		],
	},
});

window.addEventListener('resize', function() {
	table.setHeight(computeTableHeight());
});

table.on('cellEdited', function(cell) {
	// keep a trailing spare row, same as the previous editor's minSpareRows
	if (cell.getRow().getPosition() === table.getDataCount()) {
		table.addRow({});
	}
});

function openHeaderRenameInput(column) {
	const currentHeader = column.getDefinition().title;
	const rect = column.getElement().getBoundingClientRect();

	// overlay a text input on the header cell instead of a native prompt()
	// dialog, so the label stays free of hardcoded JS text
	const input = document.createElement('input');
	input.type = 'text';
	input.value = currentHeader;
	input.style.cssText = 'position:fixed;left:' + rect.left + 'px;top:' + rect.top + 'px;' +
		'width:' + rect.width + 'px;height:' + rect.height + 'px;z-index:1000;box-sizing:border-box;' +
		'border:2px solid var(--primary);background:var(--surface);color:var(--text);font:inherit;padding:0 4px;';
	document.body.appendChild(input);
	input.focus();
	input.select();

	let committed = false;
	function commit() {
		if (committed) return;
		committed = true;
		const newHeader = input.value.trim();
		input.remove();
		if (newHeader !== '' && newHeader !== currentHeader) {
			column.updateDefinition({ title: newHeader, titleFormatter: makeTitleFormatter(newHeader) });
		}
	}

	input.addEventListener('blur', commit);
	input.addEventListener('keydown', function(e) {
		if (e.key === 'Enter') {
			e.preventDefault();
			input.blur();
		} else if (e.key === 'Escape') {
			input.value = currentHeader;
			input.blur();
		}
	});
}

function saveTable() {
	const columns = dataColumns();
	const headers = columns.map(function(c) { return c.getDefinition().title; });
	const fields = columns.map(function(c) { return c.getField(); });
	const data = table.getData().map(function(row) {
		return fields.map(function(f) { return row[f] !== undefined ? row[f] : ''; });
	});
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
	.then(response => {
		if (response.ok) {
			window.location.href = returnURL;
		} else {
			return response.text().then(html => {
				document.getElementById('table-editor-status').innerHTML = html;
			});
		}
	})
	.catch(error => {
		document.getElementById('table-editor-status').innerHTML =
			'<div class="status-error">%s: ' + error + '</div>';
	});
}

function cancelTableEdit() {
	window.location.href = returnURL;
}

function downloadTable() {
	table.download('csv', downloadFilename);
}
</script>
`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "save"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "cancel"),
		translation.SprintfForRequest(configmanager.GetLanguage(), "download csv"),
		string(tableJSON),
		jsEscapeString(filePath),
		jsEscapeString(returnURL),
		jsEscapeString(downloadName),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "new column")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "insert row above")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "insert row below")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "remove row")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "insert column left")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "insert column right")),
		jsEscapeString(translation.SprintfForRequest(configmanager.GetLanguage(), "remove column")),
		translation.SprintfForRequest(configmanager.GetLanguage(), "error saving table"),
	)

	return html
}
