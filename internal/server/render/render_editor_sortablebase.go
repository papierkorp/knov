// Package render - shared sortable base for list and todo editors
package render

import (
	"fmt"
	"strings"
)

// ListItem represents a single item in the list or todo editor.
// State is only used by the todo editor; list editor always leaves it empty.
type ListItem struct {
	ID       string     `json:"id"`
	Content  string     `json:"content"`
	State    string     `json:"state,omitempty"`
	Children []ListItem `json:"children,omitempty"`
}

// ParseMarkdownToListItems parses plain markdown list format (no state extraction).
// Format: nested lists with "- " prefix and indentation for nesting.
func ParseMarkdownToListItems(content string) []ListItem {
	if content == "" {
		return []ListItem{}
	}

	lines := strings.Split(content, "\n")
	var items []ListItem
	var stack []*[]ListItem
	var indentLevels []int

	stack = append(stack, &items)
	indentLevels = append(indentLevels, -1)

	idCounter := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

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

		itemContent := strings.TrimPrefix(trimmed, "- ")

		for len(indentLevels) > 1 && indent <= indentLevels[len(indentLevels)-1] {
			stack = stack[:len(stack)-1]
			indentLevels = indentLevels[:len(indentLevels)-1]
		}

		item := ListItem{
			ID:       fmt.Sprintf("%d", idCounter),
			Content:  itemContent,
			Children: []ListItem{},
		}
		idCounter++

		*stack[len(stack)-1] = append(*stack[len(stack)-1], item)

		lastIdx := len(*stack[len(stack)-1]) - 1
		stack = append(stack, &(*stack[len(stack)-1])[lastIdx].Children)
		indentLevels = append(indentLevels, indent)
	}

	return items
}

// ConvertListItemsToMarkdown converts plain list items to markdown (no state prefix).
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

// sortableBaseJS returns the shared JS fragment embedded by both list and todo editors.
// Assumes createListItem(text, state) is defined in the enclosing editor scope.
func sortableBaseJS() string {
	return `
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

					onStart: function(evt) {
						evt.item.querySelectorAll(".item-input").forEach(function(inp) {
							inp.setAttribute("value", inp.value);
						});
						// prevent interactive child elements from interfering with drag hit-testing
						document.querySelectorAll(".state-btn").forEach(function(btn) {
							btn.style.pointerEvents = "none";
						});
					},

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

					onEnd: function(evt) {
						// restore state button interaction
						document.querySelectorAll(".state-btn").forEach(function(btn) {
							btn.style.pointerEvents = "";
						});
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

				document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
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

				document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
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
					document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
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

				document.querySelectorAll(".list-item.selected").forEach(function(i) { i.classList.remove("selected"); });
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
						state: li.dataset.state || "",
						children: nestedList ? serializeList(nestedList) : []
					});
				}
				return items;
			}

			function deserializeList(items, parentUl) {
				items.forEach(function(item) {
					const li = createListItem(item.content, item.state || "");
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
			}`
}
