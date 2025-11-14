// Package dashboard - Widget operations
package dashboard

import (
	"fmt"

	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/utils"
)

// WidgetPosition represents widget position on dashboard
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Widget represents a dashboard widget
type Widget struct {
	ID       string         `json:"id"`
	Type     WidgetType     `json:"type"`
	Title    string         `json:"title"`
	Position WidgetPosition `json:"position"`
	Config   WidgetConfig   `json:"config"`
}

// WidgetType represents widget types
type WidgetType string

const (
	WidgetTypeFilter      WidgetType = "filter"
	WidgetTypeFilterForm  WidgetType = "filterForm"
	WidgetTypeFileContent WidgetType = "fileContent"
	WidgetTypeStatic      WidgetType = "static"
	WidgetTypeTags        WidgetType = "tags"
	WidgetTypeCollections WidgetType = "collections"
	WidgetTypeFolders     WidgetType = "folders"
)

// FilterConfig represents filter configuration for widgets
type FilterConfig struct {
	Criteria []files.FilterCriteria `json:"criteria"`
	Logic    string                 `json:"logic"`
	Display  string                 `json:"display"` // list, cards, dropdown
	Limit    int                    `json:"limit"`
}

// StaticConfig represents static content configuration
type StaticConfig struct {
	Content string `json:"content"`
	Format  string `json:"format"` // html, markdown, text
}

// FileContentConfig represents file content configuration
type FileContentConfig struct {
	FilePath string `json:"filePath"`
}

// WidgetConfig represents widget-specific configuration
type WidgetConfig struct {
	Filter      *FilterConfig      `json:"filter,omitempty"`
	Static      *StaticConfig      `json:"static,omitempty"`
	FileContent *FileContentConfig `json:"fileContent,omitempty"`
}

// RenderWidget renders a widget based on its type and configuration
func RenderWidget(widgetType WidgetType, config WidgetConfig) (string, error) {
	switch widgetType {
	case WidgetTypeFilter:
		return renderFilterWidget(config.Filter)
	case WidgetTypeFilterForm:
		return renderFilterFormWidget()
	case WidgetTypeFileContent:
		return renderFileContentWidget(config.FileContent)
	case WidgetTypeStatic:
		return renderStaticWidget(config.Static)
	case WidgetTypeTags:
		return renderTagsWidget()
	case WidgetTypeCollections:
		return renderCollectionsWidget()
	case WidgetTypeFolders:
		return renderFoldersWidget()
	default:
		return "", fmt.Errorf("unknown widget type: %s", widgetType)
	}
}

func renderFilterFormWidget() (string, error) {
	return `<div class="widget-filter-form">
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
					<select name="metadata[]" id="metadata-0" onchange="updateValueField(0, this.value)">
						<option value="">Select Field</option>
						<option value="collection">Collection</option>
						<option value="tags">Tags</option>
						<option value="type">Type</option>
						<option value="status">Status</option>
						<option value="priority">Priority</option>
						<option value="createdAt">Created Date</option>
						<option value="lastEdited">Last Edited</option>
						<option value="folders">Folders</option>
						<option value="boards">Boards</option>
                        <option value="boards">Boards</option>
                        <option value="para_projects">PARA: Projects</option>
                        <option value="para_areas">PARA: Areas</option>
                        <option value="para_resources">PARA: Resources</option>
                        <option value="para_archive">PARA: Archive</option>
					</select>
					<select name="operator[]" id="operator-0">
						<option value="equals">Equals</option>
						<option value="contains">Contains</option>
						<option value="greater">Greater Than</option>
						<option value="less">Less Than</option>
						<option value="in">In Array</option>
					</select>
					<div id="value-container-0">
						<input type="text" name="value[]" id="value-0" placeholder="Value"/>
					</div>
					<select name="action[]" id="action-0">
						<option value="include">Include</option>
						<option value="exclude">Exclude</option>
					</select>
					<button type="button" onclick="removeFilterRow(0)">Remove</button>
				</div>
			</div>
			<div id="filter-results"></div>
		</form>

		<script>
			let filterRowCount = 1;

			function addFilterRow() {
				const container = document.getElementById('filter-container');
				const newRow = document.createElement('div');
				newRow.className = 'filter-row';
				newRow.id = 'filter-row-' + filterRowCount;

				const selectHTML = '<select name="metadata[]" id="metadata-' + filterRowCount + '" onchange="updateValueField(' + filterRowCount + ', this.value)">' +
					'<option value="">Select Field</option>' +
					'<option value="collection">Collection</option>' +
					'<option value="tags">Tags</option>' +
					'<option value="type">Type</option>' +
					'<option value="status">Status</option>' +
					'<option value="priority">Priority</option>' +
					'<option value="createdAt">Created Date</option>' +
					'<option value="lastEdited">Last Edited</option>' +
					'<option value="folders">Folders</option>' +
					'<option value="boards">Boards</option>' +
                    '<option value="boards">Boards</option>' +
                    '<option value="para_projects">PARA: Projects</option>' +
                    '<option value="para_areas">PARA: Areas</option>
                    '<option value="para_resources">PARA: Resources</option>' +
                    '<option value="para_archive">PARA: Archive</option>' +
					'</select>';

				const operatorHTML = '<select name="operator[]" id="operator-' + filterRowCount + '">' +
					'<option value="equals">Equals</option>' +
					'<option value="contains">Contains</option>' +
					'<option value="greater">Greater Than</option>' +
					'<option value="less">Less Than</option>' +
					'<option value="in">In Array</option>' +
					'</select>';

				const valueHTML = '<div id="value-container-' + filterRowCount + '">' +
					'<input type="text" name="value[]" id="value-' + filterRowCount + '" placeholder="Value"/>' +
					'</div>';

				const actionHTML = '<select name="action[]" id="action-' + filterRowCount + '">' +
					'<option value="include">Include</option>' +
					'<option value="exclude">Exclude</option>' +
					'</select>';

				const removeHTML = '<button type="button" onclick="removeFilterRow(' + filterRowCount + ')">Remove</button>';

				newRow.innerHTML = selectHTML + operatorHTML + valueHTML + actionHTML + removeHTML;
				container.appendChild(newRow);
				filterRowCount++;
			}

			function removeFilterRow(index) {
				const row = document.getElementById('filter-row-' + index);
				if (row) row.remove();
			}

		function updateValueField(rowIndex, fieldType) {
			const container = document.getElementById('value-container-' + rowIndex);

			if (fieldType === 'collection') {
				container.innerHTML = '<input type="text" name="value[]" autocomplete="off" id="value-' + rowIndex + '" list="collections-' + rowIndex + '" placeholder="Type or select collection (supports wildcards: project*)">' +
					'<datalist id="collections-' + rowIndex + '" hx-get="/api/metadata/options/collections" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading collections...</option>' +
					'</datalist>';
			} else if (fieldType === 'tags') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="tags-' + rowIndex + '" placeholder="Type or select tag (supports wildcards: para/p/*, zk/*)">' +
					'<datalist id="tags-' + rowIndex + '" hx-get="/api/metadata/options/tags" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading tags...</option>' +
					'</datalist>';
			} else if (fieldType === 'folders') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="folders-' + rowIndex + '" placeholder="Type or select folder (supports wildcards: guides/*, *temp*)">' +
					'<datalist id="folders-' + rowIndex + '" hx-get="/api/metadata/options/folders" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading folders...</option>' +
					'</datalist>';
			} else if (fieldType === 'type') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/filetypes" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading types...</option>' +
					'</select>';
			} else if (fieldType === 'status') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/status" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading status...</option>' +
					'</select>';
			} else if (fieldType === 'priority') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/priorities" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">Loading priorities...</option>' +
					'</select>';
			} else if (fieldType === 'createdAt' || fieldType === 'lastEdited') {
				container.innerHTML = '<input type="date" name="value[]" id="value-' + rowIndex + '" placeholder="YYYY-MM-DD"/>';
			} else {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" placeholder="Value"/>';
			}

			// Trigger HTMX processing for new elements
			if (window.htmx) {
				htmx.process(container);
			}
		}

			// Initialize first row
			document.addEventListener('DOMContentLoaded', function() {
				updateValueField(0, document.getElementById('metadata-0').value);
			});
		</script>
	</div>`, nil
}

func renderFileContentWidget(config *FileContentConfig) (string, error) {
	if config == nil || config.FilePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	fullPath := utils.ToFullPath(config.FilePath)
	content, err := files.GetFileContent(fullPath)
	if err != nil {
		logging.LogError("failed to get file content: %v", err)
		return "", err
	}

	return string(content.HTML), nil
}

func renderStaticWidget(config *StaticConfig) (string, error) {
	if config == nil || config.Content == "" {
		return "", fmt.Errorf("static content is required")
	}

	switch config.Format {
	case "html":
		return config.Content, nil
	case "markdown":
		// could add markdown processing here if needed
		return fmt.Sprintf("<div class=\"markdown-content\">%s</div>", config.Content), nil
	default:
		return fmt.Sprintf("<pre>%s</pre>", config.Content), nil
	}
}

func renderFilterWidget(config *FilterConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("filter config is required")
	}

	filteredFiles, err := files.FilterFilesByMetadata(config.Criteria, config.Logic)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		return "", err
	}

	limit := config.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(filteredFiles) > limit {
		filteredFiles = filteredFiles[:limit]
	}

	switch config.Display {
	case "cards":
		return render.RenderFileCards(filteredFiles), nil
	case "dropdown":
		return render.RenderFileDropdown(filteredFiles, 10), nil
	default:
		return render.RenderFileList(filteredFiles), nil
	}
}

func renderTagsWidget() (string, error) {
	tagCount, err := files.GetAllTags()
	if err != nil {
		return "", err
	}

	return render.RenderBrowseHTML(tagCount, "/browse/tags"), nil
}

func renderCollectionsWidget() (string, error) {
	collectionCount, err := files.GetAllCollections()
	if err != nil {
		return "", err
	}

	return render.RenderBrowseHTML(collectionCount, "/browse/collection"), nil
}

func renderFoldersWidget() (string, error) {
	folderCount, err := files.GetAllFolders()
	if err != nil {
		return "", err
	}

	return render.RenderBrowseHTML(folderCount, "/browse/folders"), nil
}
