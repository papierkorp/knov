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

// RenderListEditor renders a nested list editor with drag-and-drop support
func RenderListEditor(filepath string) string {
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
			<button type="button" onclick="listEditor.globalBold()" title="%s">
				<strong>B</strong>
			</button>
			<button type="button" onclick="listEditor.globalStrike()"><s>S</s></button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalIndent()" title="%s">→ %s</button>
			<button type="button" onclick="listEditor.globalOutdent()" title="%s">← %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="listEditor.globalDelete()" class="danger" title="%s">
				🗑 %s
			</button>
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
			let dropZoneType = null;
			let dropTarget = null;

			function initSortable(element) {
				return new Sortable(element, {
					animation: 150,
					handle: ".drag-handle",
					ghostClass: "sortable-ghost",
					dragClass: "sortable-drag",
					group: "nested",
					fallbackOnBody: true,
					swapThreshold: 0.65,

					onMove: function(evt) {
						const draggedItem = evt.dragged;
						const relatedItem = evt.related;

						document.querySelectorAll(".drop-indicator").forEach((el) => {
							el.classList.remove("drop-indicator", "drop-as-child", "drop-before", "drop-after");
						});

						dropZoneType = null;
						dropTarget = null;

						if (draggedItem === relatedItem || draggedItem.contains(relatedItem)) {
							return false;
						}

						if (relatedItem.classList.contains("list-item")) {
							const rect = relatedItem.getBoundingClientRect();
							const mouseY = evt.originalEvent.clientY;
							const itemTop = rect.top;
							const itemBottom = rect.bottom;
							const itemHeight = rect.bottom - rect.top;

							const topZone = itemTop + itemHeight * 0.25;
							const bottomZone = itemBottom - itemHeight * 0.25;

							if (mouseY < topZone) {
								relatedItem.classList.add("drop-indicator", "drop-before");
								dropZoneType = "before";
								dropTarget = relatedItem;
							} else if (mouseY > bottomZone) {
								relatedItem.classList.add("drop-indicator", "drop-after");
								dropZoneType = "after";
								dropTarget = relatedItem;
							} else {
								relatedItem.classList.add("drop-indicator", "drop-as-child");
								dropZoneType = "child";
								dropTarget = relatedItem;
							}
						}

						return true;
					},

					onEnd: function(evt) {
						const item = evt.item;

						if (dropZoneType === "child" && dropTarget) {
							let nestedList = dropTarget.querySelector(".nested-list");
							if (!nestedList) {
								nestedList = document.createElement("ul");
								nestedList.className = "sortable-list nested-list";
								dropTarget.appendChild(nestedList);
								initSortable(nestedList);
							}
							nestedList.appendChild(item);
						}

						document.querySelectorAll(".drop-indicator").forEach((el) => {
							el.classList.remove("drop-indicator", "drop-as-child", "drop-before", "drop-after");
						});

						dropZoneType = null;
						dropTarget = null;
					}
				});
			}

			function createListItem(text = "") {
				const li = document.createElement("li");
				li.className = "list-item";
				li.dataset.id = itemCounter++;

				li.innerHTML = '<div class="item-row">' +
					'<span class="drag-handle">⋮⋮</span>' +
					'<div class="item-content" contenteditable="true" data-placeholder="%s">' + text + '</div>' +
					'</div>';

				return li;
			}

			function addItem() {
				const mainList = document.getElementById("main-list");
				const newItem = createListItem();
				mainList.appendChild(newItem);

				document.querySelectorAll(".list-item.selected").forEach((item) => {
					item.classList.remove("selected");
				});

				newItem.classList.add("selected");
				const content = newItem.querySelector(".item-content");
				content.focus();
			}

			function addNestedItem() {
				let parentLi = document.querySelector(".list-item.selected");

				if (!parentLi) {
					const selected = document.querySelector(".item-content:focus");
					if (selected) {
						parentLi = selected.closest(".list-item");
					}
				}

				if (!parentLi) {
					const allItems = document.querySelectorAll(".list-item");
					if (allItems.length > 0) {
						parentLi = allItems[allItems.length - 1];
						parentLi.classList.add("selected");
					} else {
						alert("%s");
						return;
					}
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

				const content = newItem.querySelector(".item-content");
				content.focus();

				document.querySelectorAll(".list-item.selected").forEach((item) => {
					item.classList.remove("selected");
				});
				newItem.classList.add("selected");
			}

			function globalBold() {
				const selectedItem = document.querySelector(".list-item.selected");
				if (!selectedItem) {
					alert("%s");
					return;
				}
				const content = selectedItem.querySelector(".item-content");
				content.focus();
				document.execCommand("bold", false, null);
			}

			function globalStrike() {
				const selectedItem = document.querySelector(".list-item.selected");
				if (!selectedItem) {
					alert("%s");
					return;
				}
				const content = selectedItem.querySelector(".item-content");
				content.focus();
				document.execCommand("strikeThrough", false, null);
			}

			function globalIndent() {
				const selectedItem = document.querySelector(".list-item.selected");
				if (!selectedItem) {
					alert("%s");
					return;
				}
				indentItem(selectedItem);
			}

			function globalOutdent() {
				const selectedItem = document.querySelector(".list-item.selected");
				if (!selectedItem) {
					alert("%s");
					return;
				}
				outdentItem(selectedItem);
			}

			function globalDelete() {
				const selectedItem = document.querySelector(".list-item.selected");
				if (!selectedItem) {
					alert("%s");
					return;
				}
				deleteItem(selectedItem);
			}

			function indentItem(listItem) {
				const currentLi = listItem.classList.contains("list-item") ? listItem : listItem.closest(".list-item");
				const previousLi = currentLi.previousElementSibling;

				if (!previousLi) {
					return;
				}

				let nestedList = previousLi.querySelector(".nested-list");

				if (!nestedList) {
					nestedList = document.createElement("ul");
					nestedList.className = "sortable-list nested-list";
					previousLi.appendChild(nestedList);
					initSortable(nestedList);
				}

				nestedList.appendChild(currentLi);
				currentLi.querySelector(".item-content").focus();
			}

			function outdentItem(listItem) {
				const currentLi = listItem.classList.contains("list-item") ? listItem : listItem.closest(".list-item");
				const parentUl = currentLi.parentElement;
				const grandparentLi = parentUl.closest(".list-item");

				if (!grandparentLi) {
					return;
				}

				const grandparentUl = grandparentLi.parentElement;

				if (grandparentLi.nextSibling) {
					grandparentUl.insertBefore(currentLi, grandparentLi.nextSibling);
				} else {
					grandparentUl.appendChild(currentLi);
				}

				if (parentUl.children.length === 0) {
					parentUl.remove();
				}

				currentLi.querySelector(".item-content").focus();
			}

			function deleteItem(listItem) {
				const currentLi = listItem.classList.contains("list-item") ? listItem : listItem.closest(".list-item");
				const parentUl = currentLi.parentElement;

				if (confirm("%s")) {
					let nextSelection = currentLi.nextElementSibling || currentLi.previousElementSibling;

					if (!nextSelection && parentUl.closest(".list-item")) {
						nextSelection = parentUl.closest(".list-item");
					}

					currentLi.remove();

					if (parentUl.classList.contains("nested-list") && parentUl.children.length === 0) {
						parentUl.remove();
					}

					if (nextSelection) {
						document.querySelectorAll(".list-item.selected").forEach((item) => {
							item.classList.remove("selected");
						});
						nextSelection.classList.add("selected");
						nextSelection.querySelector(".item-content").focus();
					}
				}
			}

			function serializeList(ul) {
				const items = [];
				const children = ul.children;

				for (let i = 0; i < children.length; i++) {
					const li = children[i];
					const content = li.querySelector(".item-content");
					const nestedList = li.querySelector(".nested-list");

					const item = {
						id: li.dataset.id,
						content: content.innerHTML,
						children: nestedList ? serializeList(nestedList) : []
					};

					items.push(item);
				}

				return items;
			}

			function deserializeList(items, parentUl) {
				items.forEach(item => {
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

				// load existing content
				const initialContent = %s;
				if (initialContent && initialContent.length > 0) {
					try {
						deserializeList(initialContent, mainList);
					} catch (e) {
						console.error("failed to parse initial content:", e);
					}
				}

				// keyboard shortcuts
				document.addEventListener("keydown", function(e) {
					const target = e.target;

					if (target.classList.contains("item-content")) {
						if (e.ctrlKey && e.key === "b") {
							e.preventDefault();
							globalBold();
						}

						if (e.key === "Enter" && !e.shiftKey) {
							e.preventDefault();
							const currentLi = target.closest(".list-item");
							const parentUl = currentLi.parentElement;
							const newItem = createListItem();

							if (currentLi.nextSibling) {
								parentUl.insertBefore(newItem, currentLi.nextSibling);
							} else {
								parentUl.appendChild(newItem);
							}

							document.querySelectorAll(".list-item.selected").forEach((item) => {
								item.classList.remove("selected");
							});
							newItem.classList.add("selected");
							newItem.querySelector(".item-content").focus();
						}

						if (e.key === "Tab" && !e.shiftKey) {
							e.preventDefault();
							globalIndent();
						}

						if (e.key === "Tab" && e.shiftKey) {
							e.preventDefault();
							globalOutdent();
						}

						if (e.key === "Delete" && e.ctrlKey) {
							e.preventDefault();
							globalDelete();
						}
					}
				});

				// track selection
				document.addEventListener("click", function(e) {
					const content = e.target.closest(".item-content");
					if (content) {
						document.querySelectorAll(".list-item.selected").forEach((item) => {
							item.classList.remove("selected");
						});

						const listItem = content.closest(".list-item");
						listItem.classList.add("selected");
					}
				});

				// serialize on submit
				document.getElementById("list-editor-form").addEventListener("submit", function(e) {
					const mainList = document.getElementById("main-list");
					const serialized = serializeList(mainList);
					document.getElementById("list-content").value = JSON.stringify(serialized);
				});
			}

			// initialize when DOM is ready
			if (document.readyState === "loading") {
				document.addEventListener("DOMContentLoaded", init);
			} else {
				init();
			}

			return {
				addItem,
				addNestedItem,
				globalBold,
				globalStrike,
				globalIndent,
				globalOutdent,
				globalDelete
			};
		})();
</script>
</div>
	`,
		action,
		translation.SprintfForRequest(lang, "file path"),
		filepathInputHTML,
		translation.SprintfForRequest(lang, "add item"),
		translation.SprintfForRequest(lang, "add nested item"),
		translation.SprintfForRequest(lang, "bold"),
		translation.SprintfForRequest(lang, "tab"),
		translation.SprintfForRequest(lang, "indent"),
		translation.SprintfForRequest(lang, "shift+tab"),
		translation.SprintfForRequest(lang, "outdent"),
		translation.SprintfForRequest(lang, "delete"),
		translation.SprintfForRequest(lang, "delete"),
		translation.SprintfForRequest(lang, "save file"),
		cancelURL,
		translation.SprintfForRequest(lang, "cancel"),
		translation.SprintfForRequest(lang, "type here..."),
		translation.SprintfForRequest(lang, "please add a regular item first, then click on it to add a nested item under it"),
		translation.SprintfForRequest(lang, "please select an item first"),
		translation.SprintfForRequest(lang, "please select an item first"),
		translation.SprintfForRequest(lang, "please select an item first"),
		translation.SprintfForRequest(lang, "please select an item first"),
		translation.SprintfForRequest(lang, "please select an item first"),
		translation.SprintfForRequest(lang, "delete this item and all its nested items?"),
		listItemsJSON)
}
