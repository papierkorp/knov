// Package render - list editor for generic nested lists
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/translation"
)

// RenderListEditor renders a nested list editor with drag-and-drop support.
// initialItem is optional: omit for no starting item, pass "" for one empty item,
// pass a string to pre-fill the first item.
func RenderListEditor(filepath string, initialItem ...string) string {
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
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	lang := configmanager.GetLanguage()

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

	startItemJS := "null"
	if len(initialItem) > 0 {
		if jsonBytes, err := json.Marshal(initialItem[0]); err == nil {
			startItemJS = string(jsonBytes)
		}
	}

	var filepathInputHTML string
	if isEdit {
		filepathInputHTML = fmt.Sprintf(`<input type="hidden" name="filepath" value="%s" />`, filepath)
	} else {
		datalistInput := GenerateDatalistInput("filepath-input", "filepath", "",
			translation.SprintfForRequest(lang, "path/to/file.list"), "/api/files/folder-suggestions")
		filepathInputHTML = `<div class="form-group"><label>` +
			translation.SprintfForRequest(lang, "file path") + `:</label>` +
			strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1) +
			`</div>`
	}

	return fmt.Sprintf(`
<div class="component-list-editor">

	<form hx-post="%s" hx-target="#list-editor-status" hx-swap="innerHTML" id="list-editor-form">
		%s

		<div class="controls">
			<button type="button" onclick="listEditor.addItem()">+ %s</button>
			<button type="button" onclick="listEditor.addNestedItem()">+ %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalIndent()" title="%s">→ %s</button>
			<button type="button" onclick="listEditor.globalOutdent()" title="%s">← %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalDelete()" class="danger">🗑 %s</button>
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
		</div>
		<div id="list-editor-status"></div>
	</form>

	<script>
		window.listEditor = (function() {
			%s

			function createListItem(text = "", state = "") {
				const li = document.createElement("li");
				li.className = "list-item";
				li.dataset.id = itemCounter++;

				const input = document.createElement("input");
				input.type = "text";
				input.className = "item-input";
				input.value = text;
				input.placeholder = "%s";

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

				const row = document.createElement("div");
				row.className = "item-row";
				const handle = document.createElement("span");
				handle.className = "drag-handle";
				handle.textContent = "⋮⋮";
				row.appendChild(handle);
				row.appendChild(input);
				li.appendChild(row);

				return li;
			}

			function init() {
				const mainList = document.getElementById("main-list");
				initSortable(mainList);

				const initialContent = %s;
				const startItem = %s;
				if (initialContent && initialContent.length > 0) {
					try {
						deserializeList(initialContent, mainList);
					} catch (e) {
						console.error("failed to parse initial content:", e);
					}
				} else if (startItem !== null) {
					const item = createListItem(startItem);
					mainList.appendChild(item);
					item.querySelector(".item-input").focus();
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

			return { addItem, addNestedItem, globalIndent, globalOutdent, globalDelete, undoDelete };
		})();
	</script>
</div>
	`,
		action,
		filepathInputHTML,
		translation.SprintfForRequest(lang, "add item"),
		translation.SprintfForRequest(lang, "add nested item"),
		translation.SprintfForRequest(lang, "tab"),
		translation.SprintfForRequest(lang, "indent"),
		translation.SprintfForRequest(lang, "shift+tab"),
		translation.SprintfForRequest(lang, "outdent"),
		translation.SprintfForRequest(lang, "delete"),
		translation.SprintfForRequest(lang, "item deleted"),
		translation.SprintfForRequest(lang, "undo"),
		translation.SprintfForRequest(lang, "save file"),
		cancelURL,
		translation.SprintfForRequest(lang, "cancel"),
		sortableBaseJS(),
		translation.SprintfForRequest(lang, "type here..."),
		listItemsJSON,
		startItemJS)
}
