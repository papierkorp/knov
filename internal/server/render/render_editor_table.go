// Package render - table editor rendering
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/translation"
	"knov/internal/types"
)

// tableEditorSettingsMenuHTML renders the gear-menu button in the table editor toolbar,
// exposing the GroupTableEditor settings the same way codeMirrorSettingsMenuHTML does for
// CodeMirror: reuses renderSettingItem so persistence and markup stay identical to /settings.
func tableEditorSettingsMenuHTML(lang string) string {
	if !configmanager.TableEditorShowSettingsMenu.Get() {
		return ""
	}
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
	var sb strings.Builder
	sb.WriteString(`<div id="component-table-editor-settings" class="menu-wrap" x-data="dropdownMenu()" @click.outside="close()">`)
	fmt.Fprintf(&sb, `<button type="button" class="btn-secondary" x-ref="btn" @click="toggle()" title="%s"><i class="fa fa-gear"></i></button>`,
		t("editor settings"))
	sb.WriteString(`<div class="menu" x-ref="menu" :hidden="!open">`)
	for _, s := range configmanager.SettingsBySection(configmanager.SectionEditor) {
		if s.GetMeta().Group == configmanager.GroupTableEditor && s.Key() != configmanager.TableEditorShowSettingsMenu.Key() {
			sb.WriteString(renderSettingItem(s, t))
		}
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// jsTableEditorSettingsMenu wires the table editor settings menu: each boolean toggle persists
// via its own htmx form (see renderSettingItem) and is additionally applied live by rebuilding
// the running Tabulator instance via reinitTable.
func jsTableEditorSettingsMenu() string {
	if !configmanager.TableEditorShowSettingsMenu.Get() {
		return ""
	}
	return `
	document.getElementById('component-table-editor-settings').addEventListener('change', function(e) {
		var cb = e.target.closest('input[type=checkbox][name^="tableEditor"]');
		if (!cb) return;
		var opt = cb.name.slice('tableEditor'.length);
		opt = opt.charAt(0).toLowerCase() + opt.slice(1);
		var patch = {};
		patch[opt] = cb.checked;
		reinitTable(patch);
	});`
}

// tableOptionsJS renders the tableOptions JS object literal from the GroupTableEditor
// settings. Each field is written next to its own value instead of through a shared
// positional Sprintf argument list, so two adjacent booleans can't be silently swapped.
func tableOptionsJS() string {
	opts := []struct {
		key   string
		value bool
	}{
		{"selectableRows", configmanager.TableEditorSelectableRows.Get()},
		{"selectableCellRange", configmanager.TableEditorSelectableCellRange.Get()},
		{"pagination", configmanager.TableEditorPagination.Get()},
		{"rowNumbers", configmanager.TableEditorRowNumbers.Get()},
		{"sorting", configmanager.TableEditorSorting.Get()},
		{"editableColumns", configmanager.TableEditorEditableColumns.Get()},
		{"contextMenus", configmanager.TableEditorContextMenus.Get()},
	}
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, o := range opts {
		fmt.Fprintf(&sb, "\t%s: %t", o.key, o.value)
		if i < len(opts)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

// headerContextMenuScript renders the column header-context-menu/insert-column JS, with
// each translated label declared right next to the %s it fills rather than buried in a
// shared positional argument list far away in the template.
func headerContextMenuScript(lang string) string {
	insertLeft := jsEscapeString(translation.SprintfForRequest(lang, "insert column left"))
	insertRight := jsEscapeString(translation.SprintfForRequest(lang, "insert column right"))
	alignLeft := jsEscapeString(translation.SprintfForRequest(lang, "align left"))
	alignCenter := jsEscapeString(translation.SprintfForRequest(lang, "align center"))
	alignRight := jsEscapeString(translation.SprintfForRequest(lang, "align right"))
	removeColumn := jsEscapeString(translation.SprintfForRequest(lang, "remove column"))
	newColumn := jsEscapeString(translation.SprintfForRequest(lang, "new column"))
	return fmt.Sprintf(`
function headerContextMenuItems() {
	return [
		{ label: %s, action: function(e, column) { insertColumn(column, true); } },
		{ label: %s, action: function(e, column) { insertColumn(column, false); } },
		{ separator: true },
		{ label: %s, action: function(e, column) { setColumnAlign(column, 'left'); } },
		{ label: %s, action: function(e, column) { setColumnAlign(column, 'center'); } },
		{ label: %s, action: function(e, column) { setColumnAlign(column, 'right'); } },
		{ separator: true },
		{ label: %s, action: function(e, column) { deleteColumns(column); } },
	];
}

// deletes every column of the selected ranges if the clicked column is part of them,
// otherwise just the clicked column (see deleteRows)
function deleteColumns(column) {
	var columns = tableOptions.selectableCellRange ? table.getRanges().flatMap(function(r) { return r.getColumns(); }).filter(function(c) { return c.getField(); }) : [];
	columns = columns.includes(column) ? Array.from(new Set(columns)) : [column];
	columns.forEach(function(c) { c.delete(); });
}

function columnDefinition(field, title) {
	return {
		title: title,
		field: field,
		editor: tableOptions.editableColumns ? 'input' : false,
		headerSort: tableOptions.sorting,
		titleFormatter: makeTitleFormatter(title),
		headerContextMenu: tableOptions.contextMenus ? headerContextMenuItems() : undefined,
	};
}

// creates a column from a definition - shared by insertColumn and by pasteIntoTable's
// column-growing loop. redraw defaults to true; pasteIntoTable passes false to skip the
// expensive full-table redraw on each column of a multi-column growth and do it once after
// the whole batch instead.
function createColumn(definition, before, anchorField, redraw) {
	const anchorColumn = anchorField ? table.getColumn(anchorField) : null;
	const wasLast = !before && anchorColumn && anchorColumn === dataColumns()[dataColumns().length - 1];
	table.addColumn(definition, before, anchorField);
	if (wasLast) {
		// fitDataStretch permanently pins the old last column's width once it has been
		// stretched, so appending after it would otherwise leave the new column squeezed
		// off past the edge of the table - free it back up and let the layout recompute
		anchorColumn.setWidth(true);
		if (redraw !== false) table.redraw(true);
	}
}

function insertColumn(column, before, redraw) {
	const field = 'col' + (nextColIndex++);
	const title = %s;
	createColumn(columnDefinition(field, title), before, column.getField(), redraw);
}`, insertLeft, insertRight, alignLeft, alignCenter, alignRight, removeColumn, newColumn)
}

// rowContextMenuScript renders the row-context-menu JS, with each translated label
// declared right next to the %s it fills (see headerContextMenuScript).
func rowContextMenuScript(lang string) string {
	insertAbove := jsEscapeString(translation.SprintfForRequest(lang, "insert row above"))
	insertBelow := jsEscapeString(translation.SprintfForRequest(lang, "insert row below"))
	removeRow := jsEscapeString(translation.SprintfForRequest(lang, "remove row"))
	return fmt.Sprintf(`
function rowContextMenuItems() {
	return [
		{ label: %s, action: function(e, row) { table.addRow(emptyRowData(), true, row); } },
		{ label: %s, action: function(e, row) { table.addRow(emptyRowData(), false, row); } },
		{ separator: true },
		{ label: %s, action: function(e, row) { deleteRows(row); } },
	];
}

// deletes every row of the selected ranges if the clicked row is part of them,
// otherwise just the clicked row - as a single undo step
function deleteRows(row) {
	var rows = tableOptions.selectableCellRange ? table.getRanges().flatMap(function(r) { return r.getRows(); }) : [];
	rows = rows.includes(row) ? Array.from(new Set(rows)) : [row];
	withGroupedHistory(function() {
		rows.forEach(function(r) { r.delete(); });
	});
}`, insertAbove, insertBelow, removeRow)
}

// RenderTableEditorForm renders the complete table editor form
func RenderTableEditorForm(filePath string, tableIndex int) string {
	// extract table from markdown using contenthandler
	handler := contentHandler.GetHandler("markdown")
	headers, rows, aligns, err := handler.ExtractTable(filePath, tableIndex)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to extract table from file %s: %v", filePath, err)
		return fmt.Sprintf(`<div class="status-error">%s</div>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no table found in file"))
	}

	tableData := &types.SimpleTableData{
		Headers:    headers,
		Rows:       rows,
		Aligns:     aligns,
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
	returnURL := pathutils.ToFileURL(pathutils.ToWithPrefix(filePath))
	if anchor := contentHandler.FindMarkdownTableAnchor(filePath, tableIndex); anchor != "" {
		returnURL += "#" + anchor
	}

	downloadName := pathutils.BaseWithoutExt(filePath) + ".csv"
	settingsMenu := tableEditorSettingsMenuHTML(configmanager.GetLanguage())
	settingsJS := jsTableEditorSettingsMenu()

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
	%s
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

// Tabulator only builds its own clickable sort arrow (needed by headerSortClickElement:
// 'icon') as part of its *default* header renderer, which this titleFormatter replaces
// entirely - so the icon has to be added here by hand to keep sorting clickable at all.
// 'icon' mode (rather than 'header') avoids a documented conflict where a whole-header
// click handler fights with selectableRangeColumns' own header click handler.
function sortIconHTML() {
	return '<span class="tabulator-col-sorter tabulator-col-sorter-element"><span class="tabulator-arrow"></span></span>';
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
		return title + (tableOptions.editableColumns ? headerEditIcon() : '') + (tableOptions.sorting ? sortIconHTML() : '');
	};
}

function buildColumns(headers, aligns) {
	return headers.map(function(header, index) {
		return {
			title: header,
			field: 'col' + index,
			editor: tableOptions.editableColumns ? 'input' : false,
			headerSort: tableOptions.sorting,
			titleFormatter: makeTitleFormatter(header),
			hozAlign: (aligns && aligns[index]) || 'left',
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

%s

// live-toggleable via the settings menu (jsTableEditorSettingsMenu) - patched in place by
// reinitTable, which rebuilds the Tabulator instance so the new options actually take effect
let tableOptions = %s;

%s

let table;

// splits clipboard text into a raw grid instead of Tabulator's built-in 'table' parser,
// which drops any pasted columns beyond the ones the table already has. Parses CSV/TSV
// quoting rules (a field starting with '"' runs until the closing '"', "" is a literal
// quote, and a tab/newline inside quotes is part of the value, not a delimiter) so cells
// copied from a spreadsheet that contain embedded tabs or newlines survive intact.
// An unterminated quote (malformed/truncated clipboard data) is not specially handled -
// inQuotes just stays true for the rest of the text, silently folding every remaining
// tab/newline into one field instead of erroring. Accepted as-is: well-formed clipboard
// data is the overwhelmingly common case and there's no good place to surface a parse
// error from a paste event.
function pasteGrid(text) {
	text = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
	const grid = [];
	let row = [];
	let field = '';
	let inQuotes = false;
	for (let i = 0; i < text.length; i++) {
		const c = text[i];
		if (inQuotes) {
			if (c === '"') {
				if (text[i + 1] === '"') { field += '"'; i++; } else { inQuotes = false; }
			} else {
				field += c;
			}
		} else if (c === '"' && field === '') {
			inQuotes = true;
		} else if (c === '\t') {
			row.push(field);
			field = '';
		} else if (c === '\n') {
			row.push(field);
			grid.push(row);
			row = [];
			field = '';
		} else {
			field += c;
		}
	}
	row.push(field);
	grid.push(row);
	if (grid.length > 1 && grid[grid.length - 1].length === 1 && grid[grid.length - 1][0] === '') grid.pop();
	return grid;
}

// pastes starting at the active range's top-left cell, growing columns (via insertColumn)
// and rows (via addRow) so the paste always fits instead of being clipped to the current size.
// Row growth and cell writes run inside withGroupedHistory so undo restores them in one step;
// column growth is not tracked in history (see registerCustomHistoryTypes), so undoing a paste
// that added columns leaves the new, now-empty columns in place. Cells are written via
// cell.setValue() rather than row.update() because row.update() bypasses the event Tabulator's
// history module listens on and would otherwise leave pasted values completely untracked by
// undo/redo; that same event is what the cellEdited handler below listens on, so it also keeps
// a trailing spare row after a paste that fills the last row, same as for a manual edit.
function pasteIntoTable(grid) {
	if (!grid.length) return false;
	const ranges = table.getRanges();
	const range = ranges[ranges.length - 1];
	// clamp rather than abort on a stale/unmatched range reference - paste falls back to
	// the top-left cell instead of silently no-op-ing
	const startCol = range ? Math.max(0, dataColumns().indexOf(range.getColumns()[0])) : 0;
	const startRow = range ? Math.max(0, table.getRows().indexOf(range.getRows()[0])) : 0;
	const width = Math.max.apply(null, grid.map(function(r) { return r.length; }));

	withGroupedHistory(function() {
		let cols = dataColumns();
		const colsToAdd = startCol + width - cols.length;
		// skip createColumn's per-column redraw while growing (would otherwise force a full
		// table reflow once per new column) and do a single redraw after the whole batch
		for (let i = 0; i < colsToAdd; i++) {
			cols = dataColumns();
			insertColumn(cols[cols.length - 1], false, false);
		}
		if (colsToAdd > 0) table.redraw(true);
		cols = dataColumns();

		for (let i = startRow + grid.length - table.getRows().length; i > 0; i--) {
			table.addRow({});
		}
		const rows = table.getRows();

		grid.forEach(function(gridRow, ri) {
			gridRow.forEach(function(val, ci) {
				const col = cols[startCol + ci];
				if (col) rows[startRow + ri].getCell(col.getField()).setValue(val);
			});
		});
	});
	return true;
}

function createTable(data, columns) {
	columns.forEach(function(c) {
		c.headerContextMenu = tableOptions.contextMenus ? headerContextMenuItems() : undefined;
		c.headerSort = tableOptions.sorting;
		c.editor = tableOptions.editableColumns ? 'input' : false;
	});
	// with neither numbers nor drag-to-reorder active, the row-header column would show
	// nothing at all, so drop it entirely instead of leaving an empty strip
	const showRowHeader = tableOptions.rowNumbers || tableOptions.selectableRows;
	table = new Tabulator(container, {
		data: data,
		columns: columns,
		layout: 'fitDataStretch',
		height: computeTableHeight(),
		movableRows: tableOptions.selectableRows,
		movableColumns: true,
		editTriggerEvent: 'dblclick',
		headerSortClickElement: 'icon',
		rowHeader: showRowHeader ? { headerSort: false, resizable: false, frozen: true, minWidth: 30, width: 30, hozAlign: 'center', formatter: tableOptions.rowNumbers ? 'rownum' : 'handle', editor: false, rowHandle: tableOptions.selectableRows } : undefined,
		history: true,
		clipboard: tableOptions.selectableCellRange,
		clipboardPasteParser: pasteGrid,
		clipboardPasteAction: pasteIntoTable,
		selectableRange: tableOptions.selectableCellRange,
		selectableRangeColumns: tableOptions.selectableCellRange,
		selectableRangeRows: tableOptions.selectableCellRange,
		// cleared via our own keydown handler below, which groups the whole
		// selection into a single history entry
		selectableRangeClearCells: false,
		pagination: tableOptions.pagination,
		paginationSize: %d,
		rowContextMenu: tableOptions.contextMenus ? rowContextMenuItems() : undefined,
	});
	registerCustomHistoryTypes(table.modules.history);
	table.on('cellEdited', function(cell) {
		// keep a trailing spare row, same as the previous editor's minSpareRows
		if (cell.getRow().getPosition() === table.getDataCount()) {
			table.addRow({});
		}
	});
}

// Tabulator's history module records one undo step per cell edit or row add, so a
// multi-cell range-clear or paste would otherwise take many Ctrl+Z presses to undo.
// Register a combined 'grouped' history type once (shared across table rebuilds) that
// replays a batch of cellEdit/rowAdd entries through Tabulator's own per-type
// undoers/redoers, so withGroupedHistory below can collapse a whole operation into a
// single undo step. Column add/delete (insertColumn / column.delete()) is intentionally
// not recorded to history, so undoing a paste that grew the table restores its rows and
// cells but leaves the added columns in place.
//
// This reaches into undocumented Tabulator internals (history.constructor.undoers/redoers,
// history.history, history.index) that aren't covered by any automated test - there's no JS
// test harness in this repo. Re-verify range-clear/paste undo/redo by hand after bumping
// the vendored tabulator-*.min.js.
function registerCustomHistoryTypes(history) {
	var HistoryClass = history.constructor;
	if (HistoryClass.undoers.grouped) return;

	HistoryClass.undoers.grouped = function(e) {
		for (var i = e.data.length - 1; i >= 0; i--) {
			HistoryClass.undoers[e.data[i].type].call(this, e.data[i]);
		}
	};
	HistoryClass.redoers.grouped = function(e) {
		for (var i = 0; i < e.data.length; i++) {
			HistoryClass.redoers[e.data[i].type].call(this, e.data[i]);
		}
	};
}

// runs fn and collapses any history entries it produces into a single 'grouped' entry
// (registered above), so multi-cell/multi-row operations like a range clear or a paste
// undo in one step instead of one step per cell or row
function withGroupedHistory(fn) {
	var history = table.modules.history;
	var beforeIndex = history.index;
	fn();
	if (history.index > beforeIndex) {
		var entries = history.history.splice(beforeIndex + 1, history.index - beforeIndex);
		history.index = beforeIndex;
		history.action('grouped', entries[0].component, entries);
	}
}

// Ctrl/Cmd+S saves instead of triggering the browser's "Save Page" dialog - blurring
// first commits a cell that is still being edited
container.addEventListener('keydown', function(e) {
	if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.key.toLowerCase() !== 's') return;
	e.preventDefault();
	document.activeElement.blur();
	saveTable();
});

container.addEventListener('keydown', function(e) {
	if ((e.key !== 'Delete' && e.key !== 'Backspace') || !tableOptions.selectableCellRange) return;
	if (table.modules.edit && table.modules.edit.currentCell) return;
	var ranges = table.getRanges().filter(function(r) { return r.getCells().length; });
	if (!ranges.length) return;
	e.preventDefault();
	withGroupedHistory(function() {
		ranges.forEach(function(r) { r.clearValues(); });
	});
});

createTable(rowsToObjects(tableData.headers, tableData.rows).concat([{}]), buildColumns(tableData.headers, tableData.aligns));

// rebuilds the table in place with a patched option, preserving current data/columns -
// mirrors reinitCodeMirror (render_editor_codemirror.go)
function reinitTable(patch) {
	Object.assign(tableOptions, patch);
	const data = table.getData();
	const columns = table.getColumnDefinitions().filter(function(c) { return c.field; }).map(function(c) { return Object.assign({}, c); });
	table.destroy();
	createTable(data, columns);
}

window.addEventListener('resize', function() {
	table.setHeight(computeTableHeight());
});

%s

function setColumnAlign(column, align) {
	column.updateDefinition({ hozAlign: align });
}

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
	const aligns = columns.map(function(c) { return c.getDefinition().hozAlign || 'left'; });
	const fields = columns.map(function(c) { return c.getField(); });
	const data = table.getData().map(function(row) {
		return fields.map(function(f) { return row[f] !== undefined ? row[f] : ''; });
	});
	const tableIndex = tableData.tableIndex;

	const formData = new FormData();
	formData.append('filepath', filePath);
	formData.append('headers', JSON.stringify(headers));
	formData.append('rows', JSON.stringify(data));
	formData.append('aligns', JSON.stringify(aligns));
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
		settingsMenu,
		string(tableJSON),
		jsEscapeString(pathutils.ToRelative(filePath)),
		jsEscapeString(returnURL),
		jsEscapeString(downloadName),
		headerContextMenuScript(configmanager.GetLanguage()),
		tableOptionsJS(),
		rowContextMenuScript(configmanager.GetLanguage()),
		configmanager.GetTablePageSize(),
		settingsJS,
		translation.SprintfForRequest(configmanager.GetLanguage(), "error saving table"),
	)

	return html
}
