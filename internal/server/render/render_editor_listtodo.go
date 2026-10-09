// Package render - unified list/todo editor: nested drag-and-drop list items that can
// optionally carry checkbox state (open/done/cancelled/waiting) and section headers.
// A settings menu lets the user toggle "todo mode" (whether new items default to
// checkbox items) live, mirroring the CodeMirror editor's settings menu.
package render

import (
	"encoding/json"
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// RenderListEditor renders the list/todo editor with state badge cycling per item and
// an "add title" button for section headers. todoMode picks the initial default for
// new items and whether state badges start visible; the user can flip it live via the
// settings menu without losing any per-item state already on the page.
func RenderListEditor(filepath string, todoMode bool) string {
	content := ""
	isEdit := filepath != ""

	if isEdit {
		fullPath := pathutils.ToDocsPath(filepath)
		rawContent, err := contentStorage.ReadFile(fullPath)
		if err == nil {
			content = string(rawContent)
		}
	}

	action := "/api/editor/listeditor"

	cancelURL := "/"
	if isEdit {
		cancelURL = pathutils.ToFileURL(pathutils.ToWithPrefix(filepath))
	}

	lang := configmanager.GetLanguage()

	todoDateToday := ""
	if configmanager.TodoStampDate.Get() {
		todoDateToday = parser.TodayStamp()
	}

	var listItems []ListItem
	if content != "" {
		listItems = ParseMarkdownToListItems(content)
	}

	listItemsJSON := "[]"
	if len(listItems) > 0 {
		if jsonBytes, err := json.Marshal(listItems); err == nil {
			listItemsJSON = string(jsonBytes)
		}
	}

	modeValue := "list"
	placeholderExt := "path/to/file.list"
	if todoMode {
		modeValue = "todo"
		placeholderExt = "path/to/file.todo"
	}

	var filepathInputHTML string
	if isEdit {
		filepathInputHTML = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, pathutils.ToRelative(filepath))
	} else {
		datalistInput := GenerateDatalistInput("filepath-input", "filepath", "",
			translation.SprintfForRequest(lang, placeholderExt), "/api/files/folder-suggestions", true)
		filepathInputHTML = `<div class="form-group"><label>` +
			translation.SprintfForRequest(lang, "file path") + `:</label>` +
			datalistInput +
			`</div>`
	}

	settingsMenuHTML := fmt.Sprintf(`
		<div id="list-editor-settings" class="menu-wrap" x-data="dropdownMenu()" @click.outside="close()">
			<button type="button" class="menu-btn" x-ref="btn" @click="toggle()" title="%s"><i class="fa fa-gear"></i></button>
			<div class="menu" x-ref="menu" :hidden="!open">
				<label class="menu-item"><input type="checkbox" id="todo-mode-toggle"%s /> %s</label>
			</div>
		</div>`,
		translation.SprintfForRequest(lang, "editor settings"),
		func() string {
			if todoMode {
				return " checked"
			}
			return ""
		}(),
		translation.SprintfForRequest(lang, "todo mode"))

	return fmt.Sprintf(`
<div class="component-list-editor%s" id="component-list-editor">

	<form hx-post="%s" hx-target="#list-editor-status" hx-swap="innerHTML" id="list-editor-form">
		%s
		<input type="hidden" name="mode" id="list-editor-mode" value="%s" />

		<div class="controls">
			<button type="button" onclick="listEditor.addItem()">+ %s</button>
			<button type="button" onclick="listEditor.addNestedItem()">+ %s</button>
			<button type="button" onclick="listEditor.addHeader()">+ %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalIndent()" title="%s">→ %s</button>
			<button type="button" onclick="listEditor.globalOutdent()" title="%s">← %s</button>
			<button type="button" id="cascade-status-toggle" class="toggle-btn active" onclick="listEditor.toggleCascadeStatus()" title="%s">⤓ %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalDelete()" class="danger">🗑 %s</button>
			%s
		</div>

		<div id="undo-bar">
			%s <button type="button" onclick="listEditor.undoDelete()">%s</button>
		</div>

		<div class="editor-container">
			<ul id="main-list" class="sortable-list"></ul>
		</div>

		<input type="hidden" name="content" id="list-content" />

		<div class="form-actions">
			<button type="submit" class="btn-primary">%s</button>
			<button type="button" onclick="window.location.href='%s'" class="btn-secondary">%s</button>
			<div id="list-editor-status"></div>
		</div>
	</form>

	<script>
		window.listEditor = (function() {
			%s

			const STATE_CYCLE = ["open", "done", "cancelled", "waiting"];
			let cascadeStatus = true;
			let todoMode = %t;
			// server-computed "today" (app's configured timezone/format), empty when date
			// stamping is off - never computed client-side so it can't disagree with the
			// raw/CodeMirror editor's stamp (see parser.TodayStamp)
			const todoDateToday = %s;
			// tooltip for the per-item date-clear button, translated server-side like every
			// other user-facing string in this editor
			const removeDateTitle = %s;

			function stateToGlyph(state) {
				switch(state) {
					case "done":      return "[X]";
					case "cancelled": return "[-]";
					case "waiting":   return "[O]";
					default:          return "[ ]";
				}
			}

			// applies a state to a single item's badge/button/input styling
			function applyItemState(li, state) {
				const stateBtn = li.querySelector(".state-btn");
				const input = li.querySelector(".item-input");
				li.dataset.state = state;
				if (stateBtn) {
					stateBtn.className = "state-btn state-" + state;
					stateBtn.textContent = stateToGlyph(state);
				}
				if (state === "done" || state === "cancelled") {
					input.classList.add("item-struck");
				} else {
					input.classList.remove("item-struck");
				}
				if (state === "waiting") {
					input.classList.add("item-waiting");
				} else {
					input.classList.remove("item-waiting");
				}
			}

			// stamps li with a single date, replacing any previous one, so repeated
			// clicks never accumulate more than the latest date
			function setItemDate(li, date) {
				li.dataset.date = date;
				const dateSpan = li.querySelector(".item-date");
				if (dateSpan) dateSpan.textContent = date;
			}

			// hands the given state down to all nested descendants of li
			function cascadeStateToChildren(li, state) {
				li.querySelectorAll(".list-item").forEach(function(child) {
					applyItemState(child, state);
				});
			}

			function toggleCascadeStatus() {
				cascadeStatus = !cascadeStatus;
				const btn = document.getElementById("cascade-status-toggle");
				if (btn) btn.classList.toggle("active", cascadeStatus);
			}

			// text/state/type/level: state defaults to the current todo-mode default when
			// omitted (a brand-new item); pass "" explicitly to force a plain bullet.
			// level (1-6) only applies to header items.
			function createListItem(text = "", state = undefined, type = "", level = 1, date = "") {
				if (state === undefined) state = todoMode ? "open" : "";
				const isHeader = type === "header";

				const li = document.createElement("li");
				li.className = "list-item" + (isHeader ? " list-item-header" : "");
				li.dataset.id = itemCounter++;
				li.dataset.state = state;
				li.dataset.date = date;
				li.dataset.type = type;
				if (isHeader) li.dataset.level = level;

				const row = document.createElement("div");
				row.className = "item-row";
				const handle = document.createElement("span");
				handle.className = "drag-handle";
				handle.textContent = "⋮⋮";
				row.appendChild(handle);

				if (!isHeader) {
					const stateBtn = document.createElement("button");
					stateBtn.type = "button";
					stateBtn.className = "state-btn state-" + (state || "open");
					stateBtn.textContent = stateToGlyph(state || "open");
					stateBtn.addEventListener("click", function() {
						const current = li.dataset.state || "open";
						const next = STATE_CYCLE[(STATE_CYCLE.indexOf(current) + 1) %% STATE_CYCLE.length];
						applyItemState(li, next);
						if (cascadeStatus) {
							cascadeStateToChildren(li, next);
						}
						if (todoDateToday) {
							setItemDate(li, todoDateToday);
						}
					});
					row.appendChild(stateBtn);
				} else {
					const levelBadge = document.createElement("span");
					levelBadge.className = "header-level-badge";
					levelBadge.textContent = "H" + level;
					row.appendChild(levelBadge);
				}

				const input = document.createElement("input");
				input.type = "text";
				input.className = "item-input";
				input.value = text;
				input.placeholder = isHeader ? "%s" : "%s";
				if (state === "done" || state === "cancelled") {
					input.classList.add("item-struck");
				}
				if (state === "waiting") {
					input.classList.add("item-waiting");
				}

				input.addEventListener("focus", function() {
					document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
					li.classList.add("selected");
				});

				input.addEventListener("keydown", function(e) {
					if (e.key === "Enter" && !e.shiftKey) {
						e.preventDefault();
						const parentUl = li.parentElement;
						const newItem = createListItem();
						if (li.nextSibling) {
							parentUl.insertBefore(newItem, li.nextSibling);
						} else {
							parentUl.appendChild(newItem);
						}
						document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
						newItem.classList.add("selected");
						newItem.querySelector(".item-input").focus();
					}
					if (e.key === "Tab" && !e.shiftKey) { e.preventDefault(); indentItem(li); }
					if (e.key === "Tab" && e.shiftKey) { e.preventDefault(); outdentItem(li); }
					if (e.key === "Delete" && e.ctrlKey) { e.preventDefault(); deleteItem(li); }
					if (e.key === "ArrowUp" || e.key === "ArrowDown") {
						e.preventDefault();
						const inputs = Array.from(document.querySelectorAll(".item-input"));
						const idx = inputs.indexOf(e.target);
						const next = e.key === "ArrowUp" ? inputs[idx - 1] : inputs[idx + 1];
						if (next) next.focus();
					}
				});

				row.appendChild(input);

				if (!isHeader) {
					const dateSpan = document.createElement("span");
					dateSpan.className = "item-date";
					dateSpan.textContent = date;
					row.appendChild(dateSpan);

					const clearDateBtn = document.createElement("button");
					clearDateBtn.type = "button";
					clearDateBtn.className = "item-date-clear";
					clearDateBtn.title = removeDateTitle;
					clearDateBtn.textContent = "×";
					clearDateBtn.addEventListener("click", function() {
						setItemDate(li, "");
					});
					row.appendChild(clearDateBtn);
				}

				li.appendChild(row);

				return li;
			}

			// headers are section markers, always inserted at the root level
			function addHeader() {
				const mainList = document.getElementById("main-list");
				const newItem = createListItem("", "", "header", 1);
				mainList.appendChild(newItem);
				document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
				newItem.classList.add("selected");
				newItem.querySelector(".item-input").focus();
			}

			// bumps a header's heading level (1-6, clamped) instead of nesting it - called
			// by indentItem/outdentItem (sortableBaseJS) when the item is a header
			function changeHeaderLevel(li, delta) {
				const current = parseInt(li.dataset.level || "1", 10);
				const next = Math.min(6, Math.max(1, current + delta));
				li.dataset.level = next;
				const badge = li.querySelector(".header-level-badge");
				if (badge) badge.textContent = "H" + next;
			}

			function init() {
				const mainList = document.getElementById("main-list");
				initSortable(mainList);

				const initialContent = %s;
				if (initialContent && initialContent.length > 0) {
					try {
						deserializeList(initialContent, mainList);
					} catch (e) {
						console.error("failed to parse initial content:", e);
					}
				}

				const toggle = document.getElementById("todo-mode-toggle");
				if (toggle) {
					toggle.addEventListener("change", function(e) {
						todoMode = e.target.checked;
						document.getElementById("component-list-editor").classList.toggle("todo-mode", todoMode);
						document.getElementById("list-editor-mode").value = todoMode ? "todo" : "list";
						// apply the mode to existing items too, not just new ones, so saved
						// content always matches the mode (and thus the tagged editor type)
						document.querySelectorAll("#main-list .list-item").forEach(function(li) {
							if (li.dataset.type === "header") return;
							applyItemState(li, todoMode ? (li.dataset.state || "open") : "");
						});
					});
				}

				document.getElementById("list-editor-form").addEventListener("submit", function() {
					document.getElementById("list-content").value = JSON.stringify(serializeList(document.getElementById("main-list")));
				});
			}

			if (document.readyState === "loading") {
				document.addEventListener("DOMContentLoaded", init);
			} else {
				init();
			}

			return { addItem, addNestedItem, addHeader, globalIndent, globalOutdent, globalDelete, undoDelete, toggleCascadeStatus };
		})();
		initWikiAutocompleteForInputs(document.getElementById('list-editor-form'), {cursorEnd: %t, currentFile: %s});
	</script>
</div>
	`,
		func() string {
			if todoMode {
				return " todo-mode"
			}
			return ""
		}(),
		action,
		filepathInputHTML,
		modeValue,
		translation.SprintfForRequest(lang, "add item"),
		translation.SprintfForRequest(lang, "add nested item"),
		translation.SprintfForRequest(lang, "add title"),
		translation.SprintfForRequest(lang, "tab"),
		translation.SprintfForRequest(lang, "indent"),
		translation.SprintfForRequest(lang, "shift+tab"),
		translation.SprintfForRequest(lang, "outdent"),
		translation.SprintfForRequest(lang, "hand down status to sub-items"),
		translation.SprintfForRequest(lang, "cascade status"),
		translation.SprintfForRequest(lang, "delete"),
		settingsMenuHTML,
		translation.SprintfForRequest(lang, "item deleted"),
		translation.SprintfForRequest(lang, "undo"),
		translation.SprintfForRequest(lang, "save file"),
		cancelURL,
		translation.SprintfForRequest(lang, "cancel"),
		sortableBaseJS(),
		todoMode,
		jsEscapeString(todoDateToday),
		jsEscapeString(translation.SprintfForRequest(lang, "remove date")),
		translation.SprintfForRequest(lang, "heading..."),
		translation.SprintfForRequest(lang, "type here..."),
		listItemsJSON,
		configmanager.WikiLinkCursorEnd.Get(),
		jsEscapeString(pathutils.ToRelative(filepath)))
}
