package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filetype"
	"knov/internal/logging"
	"knov/internal/utils"
)

// @Summary Get paginated table
// @Description Returns paginated, sortable, searchable table HTML fragment
// @Tags components
// @Param filepath query string true "File path"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Items per page" default(25)
// @Param sort query int false "Column index to sort by" default(-1)
// @Param order query string false "Sort order (asc/desc)" default(asc)
// @Param search query string false "Search query"
// @Produce text/html
// @Success 200 {string} string "table html fragment"
// @Failure 400 {string} string "invalid parameters"
// @Failure 500 {string} string "failed to process table"
// @Router /api/components/table [get]
func handleAPIGetTable(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "filepath parameter required", http.StatusBadRequest)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	size := 25
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			size = s
		}
	}

	sortCol := -1
	if sortStr := r.URL.Query().Get("sort"); sortStr != "" {
		if s, err := strconv.Atoi(sortStr); err == nil && s >= 0 {
			sortCol = s
		}
	}

	sortOrder := "asc"
	if order := r.URL.Query().Get("order"); order == "desc" {
		sortOrder = "desc"
	}

	searchQuery := r.URL.Query().Get("search")

	fullPath := utils.ToFullPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	handler := files.GetFileTypeRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	var tableData *filetype.TableData
	if dokuwikiHandler, ok := handler.(*filetype.DokuwikiHandler); ok {
		tableData, err = dokuwikiHandler.ParseDokuWikiTable(string(fileContent))
		if err != nil {
			logging.LogError("failed to parse dokuwiki table: %v", err)
			http.Error(w, "failed to parse table", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "table parsing not supported for this file type", http.StatusBadRequest)
		return
	}

	if searchQuery != "" {
		tableData = filetype.SearchTable(tableData, searchQuery)
	}

	if sortCol >= 0 {
		tableData = filetype.SortTable(tableData, sortCol, sortOrder)
	}

	paginatedData := filetype.PaginateTable(tableData, page, size)

	html := filetype.RenderTableHTML(paginatedData, filepath, page, size, sortCol, sortOrder, searchQuery)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get file editor
// @Tags components
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/components/editor [get]
func handleAPIGetEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		content = "" // empty for new files
	}

	html := fmt.Sprintf(`
		<div id="component-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">save</button>
					<button type="button" onclick="location.reload()" class="btn-secondary">cancel</button>
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// / @Summary Get markdown editor with form
// @Tags components
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/components/markdown-editor [get]
func handleAPIGetMarkdownEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		content = ""
	}

	theme := "light"
	if configmanager.GetDarkMode() {
		theme = "dark"
	}

	html := fmt.Sprintf(`
		<form id="editor-form" hx-post="/api/files/save/%s" hx-target="#editor-status">
			<textarea id="initial-content" style="display:none;">%s</textarea>
			<div id="markdown-editor"></div>
			<div style="margin-top: 12px;">
				<button type="submit" class="btn-primary">save</button>
				<a href="/files/%s" class="btn-secondary">cancel</a>
			</div>
		</form>
		<div id="editor-status"></div>
		<script>
			(function() {
				const initialContent = document.getElementById('initial-content').value;
				const editor = new toastui.Editor({
					el: document.querySelector('#markdown-editor'),
					height: '600px',
					initialEditType: 'markdown',
					previewStyle: 'tab',
					initialValue: initialContent,
					usageStatistics: false,
					theme: '%s'
				});

				const form = document.getElementById('editor-form');
				form.addEventListener('htmx:configRequest', function(evt) {
					evt.detail.parameters['content'] = editor.getMarkdown();
				});
			})();
		</script>
	`, filepath, content, filepath, theme)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get filter form widget
// @Tags components
// @Produce html
// @Router /api/components/filter-form [get]
func handleAPIGetFilterForm(w http.ResponseWriter, r *http.Request) {
	html := `<div class="widget-filter-form">
		<form id="metadata-filter-form" hx-post="/api/files/filter" hx-target="#filter-results">
			<div>
				<button type="submit">Apply Filter</button>
				<select name="logic" id="logic-operator">
					<option value="and">AND</option>
					<option value="or">OR</option>
				</select>
				<button type="button" onclick="addFilterRow()">Add Filter</button>
			</div>
			<div id="filter-container">
				<div class="filter-row" id="filter-row-0">
					<select name="metadata[]" id="metadata-0">
						<option value="collection">Collection</option>
						<option value="tags">Tags</option>
						<option value="type">Type</option>
						<option value="status">Status</option>
						<option value="priority">Priority</option>
						<option value="createdAt">Created Date</option>
						<option value="lastEdited">Last Edited</option>
						<option value="folders">Folders</option>
						<option value="boards">Boards</option>
					</select>
					<select name="operator[]" id="operator-0">
						<option value="equals">Equals</option>
						<option value="contains">Contains</option>
						<option value="greater">Greater Than</option>
						<option value="less">Less Than</option>
						<option value="in">In Array</option>
					</select>
					<input type="text" name="value[]" id="value-0" placeholder="Value"/>
					<select name="action[]" id="action-0">
						<option value="include">Include</option>
						<option value="exclude">Exclude</option>
					</select>
					<button type="button" onclick="removeFilterRow(0)">-</button>
				</div>
			</div>
		</form>
		<div id="filter-results">
			Filtered results will appear here
		</div>
		<script>
			let filterRowCount = 1;
			function addFilterRow() {
				const container = document.getElementById('filter-container');
				const newRow = document.createElement('div');
				newRow.className = 'filter-row';
				newRow.id = 'filter-row-' + filterRowCount;
				newRow.innerHTML =
					'<select name="metadata[]" id="metadata-' + filterRowCount + '">' +
						'<option value="collection">Collection</option>' +
						'<option value="tags">Tags</option>' +
						'<option value="type">Type</option>' +
						'<option value="status">Status</option>' +
						'<option value="priority">Priority</option>' +
						'<option value="createdAt">Created Date</option>' +
						'<option value="lastEdited">Last Edited</option>' +
						'<option value="folders">Folders</option>' +
						'<option value="boards">Boards</option>' +
					'</select>' +
					'<select name="operator[]" id="operator-' + filterRowCount + '">' +
						'<option value="equals">Equals</option>' +
						'<option value="contains">Contains</option>' +
						'<option value="greater">Greater Than</option>' +
						'<option value="less">Less Than</option>' +
						'<option value="in">In Array</option>' +
					'</select>' +
					'<input type="text" name="value[]" id="value-' + filterRowCount + '" placeholder="Value"/>' +
					'<select name="action[]" id="action-' + filterRowCount + '">' +
						'<option value="include">Include</option>' +
						'<option value="exclude">Exclude</option>' +
					'</select>' +
					'<button type="button" onclick="removeFilterRow(' + filterRowCount + ')">-</button>';
				container.appendChild(newRow);
				filterRowCount++;
			}
			function removeFilterRow(index) {
				const row = document.getElementById('filter-row-' + index);
				if (row && document.querySelectorAll('.filter-row').length > 1) {
					row.remove();
				}
			}
		</script>
	</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
