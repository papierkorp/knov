// Package render - todo editor with state badges (open, done, cancelled, waiting)
// Stores state using GFM checkbox syntax: - [ ] / - [X] / - [-] / - [O]
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

// ParseMarkdownToTodoItems parses GFM checkbox list format, extracting state per item.
// Supports: - [ ] open, - [x]/[X] done, - [-] cancelled, - [o]/[O] waiting
func ParseMarkdownToTodoItems(content string) []ListItem {
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

		rest := strings.TrimPrefix(trimmed, "- ")

		// extract state prefix if present (e.g. "[ ] ", "[X] ", "[-] ", "[O] ")
		state := TodoStateOpen
		itemContent := rest
		if len(rest) >= 4 && rest[0] == '[' && rest[2] == ']' && rest[3] == ' ' {
			state = markdownToState(strings.ToUpper(rest[0:3]))
			itemContent = rest[4:]
		}

		for len(indentLevels) > 1 && indent <= indentLevels[len(indentLevels)-1] {
			stack = stack[:len(stack)-1]
			indentLevels = indentLevels[:len(indentLevels)-1]
		}

		item := ListItem{
			ID:       fmt.Sprintf("%d", idCounter),
			Content:  itemContent,
			State:    state,
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

// ConvertTodoItemsToMarkdown converts todo items to GFM checkbox markdown.
func ConvertTodoItemsToMarkdown(items []ListItem, indent int) string {
	var md strings.Builder
	indentStr := strings.Repeat("  ", indent)

	for _, item := range items {
		md.WriteString(indentStr)
		md.WriteString("- ")
		md.WriteString(stateToMarkdown(item.State))
		md.WriteString(item.Content)
		md.WriteString("\n")

		if len(item.Children) > 0 {
			md.WriteString(ConvertTodoItemsToMarkdown(item.Children, indent+1))
		}
	}

	return md.String()
}

// RenderTodoEditor renders a todo editor with state badge cycling per item.
// initialItem is optional: omit for no starting item, pass "" for one empty open item,
// pass a string to pre-fill the first item.
func RenderTodoEditor(filepath string, initialItem ...string) string {
	content := ""
	isEdit := filepath != ""

	if isEdit {
		fullPath := pathutils.ToDocsPath(filepath)
		rawContent, err := contentStorage.ReadFile(fullPath)
		if err == nil {
			content = string(rawContent)
		}
	}

	action := "/api/editor/todoeditor"

	cancelURL := "/"
	if isEdit {
		cancelURL = fmt.Sprintf("/files/%s", filepath)
	}

	lang := configmanager.GetLanguage()

	var listItems []ListItem
	if content != "" {
		listItems = ParseMarkdownToTodoItems(content)
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
			translation.SprintfForRequest(lang, "path/to/file.todo"), "/api/files/folder-suggestions")
		filepathInputHTML = `<div class="form-group"><label>` +
			translation.SprintfForRequest(lang, "file path") + `:</label>` +
			strings.Replace(datalistInput, `class="form-input"`, `class="form-input" required`, 1) +
			`</div>`
	}

	return fmt.Sprintf(`
<div class="component-todo-editor">

	<form hx-post="%s" hx-target="#todo-editor-status" hx-swap="innerHTML" id="todo-editor-form">
		%s

		<div class="controls">
			<button type="button" onclick="todoEditor.addItem()">+ %s</button>
			<button type="button" onclick="todoEditor.addNestedItem()">+ %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="todoEditor.globalIndent()" title="%s">→ %s</button>
			<button type="button" onclick="todoEditor.globalOutdent()" title="%s">← %s</button>
			<button type="button" id="cascade-status-toggle" class="toggle-btn active" onclick="todoEditor.toggleCascadeStatus()" title="%s">⤓ %s</button>
			<span class="separator">|</span>
			<button type="button" onclick="todoEditor.globalDelete()" class="danger">🗑 %s</button>
		</div>

		<div id="undo-bar">
			%s <button type="button" onclick="todoEditor.undoDelete()">%s</button>
		</div>

		<div class="editor-container">
			<ul id="main-list" class="sortable-list"></ul>
		</div>

		<input type="hidden" name="content" id="list-content" />

		<div class="form-actions">
			<button type="submit" class="btn-primary">%s</button>
			<button type="button" onclick="window.location.href='%s'" class="btn-secondary">%s</button>
		</div>
		<div id="todo-editor-status"></div>
	</form>

	<script>
		window.todoEditor = (function() {
			%s

			const STATE_CYCLE = ["open", "done", "cancelled", "waiting"];
			let cascadeStatus = true;

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
				stateBtn.className = "state-btn state-" + state;
				stateBtn.textContent = stateToGlyph(state);
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

			function createListItem(text = "", state = "") {
				if (!state) state = "open";

				const li = document.createElement("li");
				li.className = "list-item";
				li.dataset.id = itemCounter++;
				li.dataset.state = state;

				const stateBtn = document.createElement("button");
				stateBtn.type = "button";
				stateBtn.className = "state-btn state-" + state;
				stateBtn.textContent = stateToGlyph(state);
				stateBtn.addEventListener("click", function() {
					const current = li.dataset.state || "open";
					const next = STATE_CYCLE[(STATE_CYCLE.indexOf(current) + 1) %% STATE_CYCLE.length];
					applyItemState(li, next);
					if (cascadeStatus) {
						cascadeStateToChildren(li, next);
					}
				});

				const input = document.createElement("input");
				input.type = "text";
				input.className = "item-input";
				input.value = text;
				input.placeholder = "%s";
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

				const row = document.createElement("div");
				row.className = "item-row";
				const handle = document.createElement("span");
				handle.className = "drag-handle";
				handle.textContent = "⋮⋮";
				row.appendChild(handle);
				row.appendChild(stateBtn);
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

				document.getElementById("todo-editor-form").addEventListener("submit", function() {
					document.getElementById("list-content").value = JSON.stringify(serializeList(document.getElementById("main-list")));
				});
			}

			if (document.readyState === "loading") {
				document.addEventListener("DOMContentLoaded", init);
			} else {
				init();
			}

			return { addItem, addNestedItem, globalIndent, globalOutdent, globalDelete, undoDelete, toggleCascadeStatus };
		})();
		initWikiAutocompleteForInputs(document.getElementById('todo-editor-form'), {cursorEnd: %t});
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
		translation.SprintfForRequest(lang, "hand down status to sub-items"),
		translation.SprintfForRequest(lang, "cascade status"),
		translation.SprintfForRequest(lang, "delete"),
		translation.SprintfForRequest(lang, "item deleted"),
		translation.SprintfForRequest(lang, "undo"),
		translation.SprintfForRequest(lang, "save file"),
		cancelURL,
		translation.SprintfForRequest(lang, "cancel"),
		sortableBaseJS(),
		translation.SprintfForRequest(lang, "type here..."),
		listItemsJSON,
		startItemJS,
		configmanager.GetEditorSettings().WikiLinkCursorEnd)
}
