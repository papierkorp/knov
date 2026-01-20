// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/parser"
	"knov/internal/translation"
	"knov/internal/types"
)

// RenderTableComponent renders a paginated, sortable, searchable table HTML fragment
func RenderTableComponent(tableData *types.TableData, filepath string, page, size, sortCol int, sortOrder, searchQuery string) string {
	return parser.RenderTableHTML(tableData, filepath, page, size, sortCol, sortOrder, searchQuery)
}

// RenderIndexEntryRowHelper generates HTML for a single index entry row
func RenderIndexEntryRowHelper(index int, entry IndexEntry) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="entry-row" data-entry-index="%d">`, index))

	// controls on the left
	html.WriteString(`<div class="entry-controls">`)
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, -1)" class="btn-move"><i class="fa-solid fa-arrow-up"></i></button>`, index))
	html.WriteString(fmt.Sprintf(`<button type="button" onclick="moveEntry(%d, 1)" class="btn-move"><i class="fa-solid fa-arrow-down"></i></button>`, index))
	html.WriteString(`<button type="button" onclick="removeEntry(this)" class="btn-remove"><i class="fa-solid fa-xmark"></i></button>`)
	html.WriteString(`</div>`)

	// content on the right
	html.WriteString(`<div class="entry-content">`)
	html.WriteString(fmt.Sprintf(`<input type="hidden" name="entries[%d][type]" value="%s"/>`, index, entry.Type))

	switch entry.Type {
	case "separator":
		html.WriteString(`<div class="entry-separator">`)
		html.WriteString(fmt.Sprintf(`<span>%s</span>`, translation.SprintfForRequest(configmanager.GetLanguage(), "separator")))
		html.WriteString(`</div>`)

	case "file":
		html.WriteString(`<div class="entry-file">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "file")))
		inputID := fmt.Sprintf("entry-file-%d", index)
		html.WriteString(GenerateDatalistInput(inputID, fmt.Sprintf("entries[%d][value]", index), entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "search files"), "/api/files/list?format=datalist"))
		html.WriteString(`</div>`)

	case "title":
		html.WriteString(`<div class="entry-title">`)
		html.WriteString(fmt.Sprintf(`<label>%s:</label>`, translation.SprintfForRequest(configmanager.GetLanguage(), "title")))
		html.WriteString(fmt.Sprintf(`<input type="text" name="entries[%d][value]" value="%s" class="form-input" placeholder="%s"/>`, index, entry.Value, translation.SprintfForRequest(configmanager.GetLanguage(), "enter title")))
		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// Use HTMX event to trigger reindexing after content is swapped
	html.WriteString(`<script>
document.body.addEventListener('htmx:afterSwap', function(evt) {
	if (evt.detail.target.id === 'entries-container') {
		console.log('htmx:afterSwap triggered for entries-container');
		if (typeof window.reindexEntries === 'function') {
			window.reindexEntries();
		}
	}
});
</script>`)

	return html.String()
}
