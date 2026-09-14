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
	sb.WriteString(`<div id="component-table-editor-settings" class="fp-menu-wrap" x-data="dropdownMenu()" @click.outside="close()">`)
	fmt.Fprintf(&sb, `<button type="button" class="btn-secondary" x-ref="btn" @click="toggle()" title="%s"><i class="fa fa-gear"></i></button>`,
		t("editor settings"))
	sb.WriteString(`<div class="fp-menu" x-ref="menu" :hidden="!open">`)
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
		{ label: %s, action: function(e, column) { column.delete(); } },
	];
}

function insertColumn(column, before) {
	const field = 'col' + (nextColIndex++);
	const title = %s;
	table.addColumn({
		title: title,
		field: field,
		editor: tableOptions.editableColumns ? 'input' : false,
		headerSort: tableOptions.sorting,
		titleFormatter: makeTitleFormatter(title),
		headerContextMenu: tableOptions.contextMenus ? headerContextMenuItems() : undefined,
	}, before, column.getField());
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
		{ label: %s, action: function(e, row) { row.delete(); } },
	];
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
	returnURL := pathutils.ToFileURL(filePath)
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
	registerRangeClearHistory(table.modules.history);
	table.on('cellEdited', function(cell) {
		// keep a trailing spare row, same as the previous editor's minSpareRows
		if (cell.getRow().getPosition() === table.getDataCount()) {
			table.addRow({});
		}
	});
}

// Tabulator's built-in range-clear writes each cell's value one at a time, so its
// history module records one undo step per cell. Register a combined history type
// once (shared across table rebuilds) so our keydown handler below can collapse a
// whole selection's clear into a single entry - one undo restores every cell.
//
// This reaches into undocumented Tabulator internals (history.constructor.undoers/redoers,
// history.history, history.index) that aren't covered by any automated test - there's no JS
// test harness in this repo. Re-verify range-clear undo/redo by hand after bumping the
// vendored tabulator-*.min.js.
function registerRangeClearHistory(history) {
	var HistoryClass = history.constructor;
	if (HistoryClass.undoers.rangeClear) return;
	HistoryClass.undoers.rangeClear = function(e) {
		e.data.forEach(function(entry) {
			entry.component.setValueProcessData(entry.data.oldValue);
			entry.component.cellRendered();
		});
	};
	HistoryClass.redoers.rangeClear = function(e) {
		e.data.forEach(function(entry) {
			entry.component.setValueProcessData(entry.data.newValue);
			entry.component.cellRendered();
		});
	};
}

container.addEventListener('keydown', function(e) {
	if ((e.key !== 'Delete' && e.key !== 'Backspace') || !tableOptions.selectableCellRange) return;
	if (table.modules.edit && table.modules.edit.currentCell) return;
	var ranges = table.getRanges().filter(function(r) { return r.getCells().length; });
	if (!ranges.length) return;
	e.preventDefault();
	var history = table.modules.history;
	var beforeIndex = history.index;
	ranges.forEach(function(r) { r.clearValues(); });
	if (history.index > beforeIndex) {
		var entries = history.history.splice(beforeIndex + 1, history.index - beforeIndex);
		history.index = beforeIndex;
		history.action('rangeClear', entries[0].component, entries);
	}
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
		jsEscapeString(filePath),
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
