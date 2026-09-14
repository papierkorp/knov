// Package render - shared sortable base for list and todo editors
package render

import (
	"fmt"
	"strings"

	"knov/internal/parser"
)

// todo state constants
const (
	TodoStateOpen      = "open"
	TodoStateDone      = "done"
	TodoStateCancelled = "cancelled"
	TodoStateWaiting   = "waiting"
)

// stateToGlyph maps a state string to its GFM display glyph
func stateToGlyph(state string) string {
	switch state {
	case TodoStateDone:
		return "[X]"
	case TodoStateCancelled:
		return "[-]"
	case TodoStateWaiting:
		return "[O]"
	default:
		return "[ ]"
	}
}

// stateToMarkdown maps a state string to its GFM markdown prefix
func stateToMarkdown(state string) string {
	switch state {
	case TodoStateDone:
		return "[X] "
	case TodoStateCancelled:
		return "[-] "
	case TodoStateWaiting:
		return "[O] "
	default:
		return "[ ] "
	}
}

// markdownToState parses a GFM checkbox prefix into a state string
func markdownToState(prefix string) string {
	switch strings.ToUpper(prefix) {
	case "[X]":
		return TodoStateDone
	case "[-]":
		return TodoStateCancelled
	case "[O]":
		return TodoStateWaiting
	default:
		return TodoStateOpen
	}
}

// ListItem represents a single item in the list/todo editor. State is set for
// checkbox items (open/done/cancelled/waiting) and empty for a plain bullet -
// both can be mixed freely in the same file. Type is "header" for a section
// heading (rendered/parsed as "# text".."###### text"), with Level (1-6)
// giving the heading depth; headers are always flat, never nested as a child.
type ListItem struct {
	ID       string     `json:"id"`
	Content  string     `json:"content"`
	State    string     `json:"state,omitempty"`
	Date     string     `json:"date,omitempty"`
	Type     string     `json:"type,omitempty"`
	Level    int        `json:"level,omitempty"`
	Children []ListItem `json:"children,omitempty"`
}

// headerLevel returns the ATX heading level (1-6) if trimmed is a "# "/"## "/... line,
// along with the heading text, or ok=false if it isn't a heading line.
func headerLevel(trimmed string) (level int, title string, ok bool) {
	for level < 6 && level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmed[level+1:]), true
}

// ParseMarkdownToListItems parses a nested markdown list, extracting GFM checkbox
// state per item when present (- [ ] open, - [x]/[X] done, - [-] cancelled, - [o]/[O]
// waiting) and leaving State empty for plain bullets. A "#".."######" line is parsed
// as a root-level header item carrying its heading level, resetting any open nesting.
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
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if level, title, ok := headerLevel(trimmed); ok {
			stack = stack[:1]
			indentLevels = indentLevels[:1]
			*stack[0] = append(*stack[0], ListItem{
				ID:      fmt.Sprintf("%d", idCounter),
				Content: title,
				Type:    "header",
				Level:   level,
			})
			idCounter++
			continue
		}

		if !strings.HasPrefix(trimmed, "- ") {
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

		rest := strings.TrimPrefix(trimmed, "- ")

		// extract state prefix if present (e.g. "[ ] ", "[X] ", "[-] ", "[O] "); only
		// checkbox items can carry a date stamp, so plain bullets never have coincidental
		// trailing "(...)" text mistaken for one
		state := ""
		date := ""
		itemContent := rest
		if len(rest) >= 4 && rest[0] == '[' && rest[2] == ']' && rest[3] == ' ' {
			state = markdownToState(strings.ToUpper(rest[0:3]))
			itemContent, date = parser.SplitTodoDate(rest[4:])
		}

		for len(indentLevels) > 1 && indent <= indentLevels[len(indentLevels)-1] {
			stack = stack[:len(stack)-1]
			indentLevels = indentLevels[:len(indentLevels)-1]
		}

		item := ListItem{
			ID:       fmt.Sprintf("%d", idCounter),
			Content:  itemContent,
			State:    state,
			Date:     date,
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

// ConvertListItemsToMarkdown converts list/todo items to markdown: a GFM checkbox
// prefix when item.State is set, a plain bullet otherwise, and "#".."######" for
// header items (per item.Level, root-level only, no children).
func ConvertListItemsToMarkdown(items []ListItem, indent int) string {
	var md strings.Builder
	indentStr := strings.Repeat("  ", indent)

	for _, item := range items {
		if item.Type == "header" {
			level := item.Level
			if level < 1 || level > 6 {
				level = 1
			}
			fmt.Fprintf(&md, "\n%s %s\n\n", strings.Repeat("#", level), item.Content)
			continue
		}

		md.WriteString(indentStr)
		md.WriteString("- ")
		if item.State != "" {
			md.WriteString(stateToMarkdown(item.State))
		}
		md.WriteString(item.Content)
		if item.State != "" && item.Date != "" {
			fmt.Fprintf(&md, " (%s)", item.Date)
		}
		md.WriteString("\n")

		if len(item.Children) > 0 {
			md.WriteString(ConvertListItemsToMarkdown(item.Children, indent+1))
		}
	}

	return md.String()
}

// sortableBaseJS returns the shared JS fragment embedded by the list/todo editor.
// Assumes createListItem(text, state, type, level, date) and changeHeaderLevel(li, delta)
// are defined in the enclosing editor scope.
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

						// headers are flat section markers: always root-level, so block any
						// native reorder that would drop one into a nested-list, not just
						// the drop-as-child gesture below
						if (evt.dragged.dataset.type === "header") return evt.to.id === "main-list";

						const related = evt.related;
						if (!related || !related.classList.contains("list-item")) return true;
						if (related === evt.dragged) return true;
						if (related.dataset.type === "header") return true;

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
				if (parentLi.dataset.type === "header") return; // headers can't hold children

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
				// headers are flat section markers: indent bumps heading level instead
				if (li.dataset.type === "header") {
					changeHeaderLevel(li, 1);
					return;
				}

				const previousLi = li.previousElementSibling;
				if (!previousLi) return;
				if (previousLi.dataset.type === "header") return; // headers can't hold children

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
				if (li.dataset.type === "header") {
					changeHeaderLevel(li, -1);
					return;
				}

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
					const item = {
						id: li.dataset.id,
						content: input ? input.value : "",
						state: li.dataset.state || "",
						date: li.dataset.date || "",
						type: li.dataset.type || "",
						children: nestedList ? serializeList(nestedList) : []
					};
					if (li.dataset.type === "header") {
						item.level = parseInt(li.dataset.level || "1", 10);
					}
					items.push(item);
				}
				return items;
			}

			function deserializeList(items, parentUl) {
				items.forEach(function(item) {
					const li = createListItem(item.content, item.state || "", item.type || "", item.level || 1, item.date || "");
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
