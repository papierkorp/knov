// Package render - list editor for todo and journaling files
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

// Usage
// no initial item (editing existing file)
//render.RenderListEditor("path/to/file.list")
// one empty item ready to type
// render.RenderListEditor("", "")
// pre-filled first item
// render.RenderListEditor("", "Buy milk")

// ListItem represents a single item in the list editor
type ListItem struct {
	ID       string     `json:"id"`
	Content  string     `json:"content"`
	Children []ListItem `json:"children,omitempty"`
}

// ParseMarkdownToListItems parses markdown list format to list items
// Format: markdown nested lists with - for items, indentation for nesting
func ParseMarkdownToListItems(content string) []ListItem {
	if content == "" {
		return []ListItem{}
	}

	lines := strings.Split(content, "\n")
	var items []ListItem
	var stack []*[]ListItem // stack of parent lists
	var indentLevels []int  // track indent level at each stack position

	stack = append(stack, &items)
	indentLevels = append(indentLevels, -1) // root level

	idCounter := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// count leading spaces/tabs for indent level
		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else if ch == '\t' {
				indent += 4
			} else {
				break
			}
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}

		// extract content (remove "- " prefix)
		itemContent := strings.TrimPrefix(trimmed, "- ")

		// find correct parent level
		for len(indentLevels) > 1 && indent <= indentLevels[len(indentLevels)-1] {
			stack = stack[:len(stack)-1]
			indentLevels = indentLevels[:len(indentLevels)-1]
		}

		// create new item
		item := ListItem{
			ID:       fmt.Sprintf("%d", idCounter),
			Content:  itemContent,
			Children: []ListItem{},
		}
		idCounter++

		// add to current parent
		*stack[len(stack)-1] = append(*stack[len(stack)-1], item)

		// if this might have children, push it to stack
		lastIdx := len(*stack[len(stack)-1]) - 1
		stack = append(stack, &(*stack[len(stack)-1])[lastIdx].Children)
		indentLevels = append(indentLevels, indent)
	}

	return items
}

// ConvertListItemsToMarkdown converts list items to markdown format
func ConvertListItemsToMarkdown(items []ListItem, indent int) string {
	var md strings.Builder

	indentStr := strings.Repeat("  ", indent)

	for _, item := range items {
		md.WriteString(indentStr)
		md.WriteString("- ")
		md.WriteString(item.Content)
		md.WriteString("\n")

		if len(item.Children) > 0 {
			md.WriteString(ConvertListItemsToMarkdown(item.Children, indent+1))
		}
	}

	return md.String()
}

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

	// parse markdown to list items for editing
	var listItems []ListItem
	if content != "" {
		listItems = ParseMarkdownToListItems(content)
	}

	// serialize to JSON for frontend
	listItemsJSON := "[]"
	if len(listItems) > 0 {
		if jsonBytes, err := json.Marshal(listItems); err == nil {
			listItemsJSON = string(jsonBytes)
		}
	}

	// encode optional initial item as JS value: null = no item, "..." = item text
	startItemJS := "null"
	if len(initialItem) > 0 {
		if jsonBytes, err := json.Marshal(initialItem[0]); err == nil {
			startItemJS = string(jsonBytes)
		}
	}

	// generate filepath input - use datalist for new files, simple input for editing
	var filepathInputHTML string
	if isEdit {
		filepathInputHTML = fmt.Sprintf(`<input type="text" name="filepath" value="%s" readonly required class="form-input" />`, filepath)
	} else {
		datalistInput := GenerateDatalistInput("filepath-input", "filepath", "", translation.SprintfForRequest(lang, "path/to/file.list"), "/api/files/folder-suggestions")
		// add required attribute
		filepathInputHTML = strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1)
	}

	return fmt.Sprintf(`
<div id="component-list-editor">

	<form hx-post="%s" hx-target="#editor-status" id="list-editor-form">
		<div class="form-group">
			<label>%s:</label>
			%s
		</div>

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
		<div id="editor-status"></div>
	</form>

	<script>
		window.listEditor = (function() {
			let itemCounter = 0;
			let lastDeleted = null;
			let undoTimer = null;
			let dropAsChild = null;

			function initSortable(element) {
				return new Sortable(element, {
					animation: 150,
					handle: ".drag-handle",
					ghostClass: "sortable-ghost",
					group: "nested",
					swapThreshold: 0.65,

					// sync input value -> attribute so the drag ghost shows correct text
					// (cloneNode copies attributes, not DOM properties like .value)
					onStart: function(evt) {
						evt.item.querySelectorAll(".item-input").forEach(function(inp) {
							inp.setAttribute("value", inp.value);
						});
					},

					// hovering over another item's drag-handle = drop as child
					onMove: function(evt) {
						document.querySelectorAll(".drop-as-child").forEach(function(el) {
							el.classList.remove("drop-as-child");
						});
						dropAsChild = null;

						const related = evt.related;
						if (!related || !related.classList.contains("list-item")) return true;
						if (related === evt.dragged) return true;

						const target = evt.originalEvent.target;
						if (target && target.closest(".drag-handle")) {
							related.classList.add("drop-as-child");
							dropAsChild = related;
						}

						return true;
					},

					// after drop: nest if drop-as-child, restore values, clean empty lists
					onEnd: function(evt) {
						document.querySelectorAll(".drop-as-child").forEach(function(el) {
							el.classList.remove("drop-as-child");
						});

						if (dropAsChild && dropAsChild !== evt.item) {
							let nestedList = dropAsChild.querySelector(".nested-list");
							if (!nestedList) {
								nestedList = document.createElement("ul");
								nestedList.className = "sortable-list nested-list";
								dropAsChild.appendChild(nestedList);
								initSortable(nestedList);
							}
							nestedList.appendChild(evt.item);
						}
						dropAsChild = null;

						evt.item.querySelectorAll(".item-input").forEach(function(inp) {
							inp.value = inp.getAttribute("value") || "";
						});
						document.querySelectorAll(".nested-list").forEach(function(ul) {
							if (ul.children.length === 0) ul.remove();
						});
					}
				});
			}

			function createListItem(text = "") {
				const li = document.createElement("li");
				li.className = "list-item";
				li.dataset.id = itemCounter++;

				const input = document.createElement("input");
				input.type = "text";
				input.className = "item-input";
				input.value = text;
				input.placeholder = "%s";

				input.addEventListener("focus", function() {
					document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
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
						document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
						newItem.classList.add("selected");
						newItem.querySelector(".item-input").focus();
					}
					if (e.key === "Tab" && !e.shiftKey) {
						e.preventDefault();
						indentItem(li);
					}
					if (e.key === "Tab" && e.shiftKey) {
						e.preventDefault();
						outdentItem(li);
					}
					if (e.key === "Delete" && e.ctrlKey) {
						e.preventDefault();
						deleteItem(li);
					}
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

			function addItem() {
				const selected = document.querySelector(".list-item.selected");
				const mainList = document.getElementById("main-list");
				const newItem = createListItem();

				if (selected) {
					const parentUl = selected.parentElement;
					if (selected.nextSibling) {
						parentUl.insertBefore(newItem, selected.nextSibling);
					} else {
						parentUl.appendChild(newItem);
					}
				} else {
					mainList.appendChild(newItem);
				}

				document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
				newItem.classList.add("selected");
				newItem.querySelector(".item-input").focus();
			}

			function addNestedItem() {
				let parentLi = document.querySelector(".list-item.selected");
				if (!parentLi) {
					const allItems = document.querySelectorAll(".list-item");
					if (allItems.length === 0) return;
					parentLi = allItems[allItems.length - 1];
				}

				let nestedList = parentLi.querySelector(".nested-list");
				if (!nestedList) {
					nestedList = document.createElement("ul");
					nestedList.className = "sortable-list nested-list";
					parentLi.appendChild(nestedList);
					initSortable(nestedList);
				}

				const newItem = createListItem();
				nestedList.appendChild(newItem);

				document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
				newItem.classList.add("selected");
				newItem.querySelector(".item-input").focus();
			}

			function indentItem(li) {
				const previousLi = li.previousElementSibling;
				if (!previousLi) return;

				let nestedList = previousLi.querySelector(".nested-list");
				if (!nestedList) {
					nestedList = document.createElement("ul");
					nestedList.className = "sortable-list nested-list";
					previousLi.appendChild(nestedList);
					initSortable(nestedList);
				}

				nestedList.appendChild(li);
				li.querySelector(".item-input").focus();
			}

			function outdentItem(li) {
				const parentUl = li.parentElement;
				const grandparentLi = parentUl.closest(".list-item");
				if (!grandparentLi) return;

				const grandparentUl = grandparentLi.parentElement;
				if (grandparentLi.nextSibling) {
					grandparentUl.insertBefore(li, grandparentLi.nextSibling);
				} else {
					grandparentUl.appendChild(li);
				}

				if (parentUl.children.length === 0) parentUl.remove();
				li.querySelector(".item-input").focus();
			}

			function deleteItem(li) {
				const parentUl = li.parentElement;
				const nextFocus = li.nextElementSibling || li.previousElementSibling;

				lastDeleted = { item: li, parent: parentUl, nextSibling: li.nextSibling };
				li.remove();

				if (parentUl.classList.contains("nested-list") && parentUl.children.length === 0) {
					parentUl.remove();
				}

				if (nextFocus) {
					document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
					nextFocus.classList.add("selected");
					nextFocus.querySelector(".item-input").focus();
				}

				const bar = document.getElementById("undo-bar");
				bar.classList.add("visible");
				if (undoTimer) clearTimeout(undoTimer);
				undoTimer = setTimeout(function() {
					bar.classList.remove("visible");
					lastDeleted = null;
				}, 5000);
			}

			function undoDelete() {
				if (!lastDeleted) return;
				const { item, parent, nextSibling } = lastDeleted;

				if (!parent.isConnected) {
					document.getElementById("main-list").appendChild(item);
				} else if (nextSibling) {
					parent.insertBefore(item, nextSibling);
				} else {
					parent.appendChild(item);
				}

				document.querySelectorAll(".list-item.selected").forEach(i => i.classList.remove("selected"));
				item.classList.add("selected");
				item.querySelector(".item-input").focus();

				document.getElementById("undo-bar").classList.remove("visible");
				if (undoTimer) clearTimeout(undoTimer);
				lastDeleted = null;
			}

			function globalIndent() {
				const selected = document.querySelector(".list-item.selected");
				if (selected) indentItem(selected);
			}

			function globalOutdent() {
				const selected = document.querySelector(".list-item.selected");
				if (selected) outdentItem(selected);
			}

			function globalDelete() {
				const selected = document.querySelector(".list-item.selected");
				if (selected) deleteItem(selected);
			}

			function serializeList(ul) {
				const items = [];
				for (const li of ul.children) {
					const input = li.querySelector(".item-input");
					const nestedList = li.querySelector(".nested-list");
					items.push({
						id: li.dataset.id,
						content: input ? input.value : "",
						children: nestedList ? serializeList(nestedList) : []
					});
				}
				return items;
			}

			function deserializeList(items, parentUl) {
				items.forEach(function(item) {
					const li = createListItem(item.content);
					li.dataset.id = item.id;
					itemCounter = Math.max(itemCounter, parseInt(item.id) + 1);
					parentUl.appendChild(li);

					if (item.children && item.children.length > 0) {
						const nestedList = document.createElement("ul");
						nestedList.className = "sortable-list nested-list";
						li.appendChild(nestedList);
						initSortable(nestedList);
						deserializeList(item.children, nestedList);
					}
				});
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
		translation.SprintfForRequest(lang, "file path"),
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
		translation.SprintfForRequest(lang, "type here..."),
		listItemsJSON,
		startItemJS)
}
