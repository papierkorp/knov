// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"

	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/utils"
)

// RenderWidget renders a widget based on its type and configuration
func RenderWidget(widgetType dashboard.WidgetType, config dashboard.WidgetConfig) (string, error) {
	switch widgetType {
	case dashboard.WidgetTypeFilter:
		return renderFilterWidget(config.Filter)
	case dashboard.WidgetTypeFilterForm:
		return renderFilterFormWidget()
	case dashboard.WidgetTypeFileContent:
		return renderFileContentWidget(config.FileContent)
	case dashboard.WidgetTypeStatic:
		return renderStaticWidget(config.Static)
	case dashboard.WidgetTypeTags:
		return renderTagsWidget()
	case dashboard.WidgetTypeCollections:
		return renderCollectionsWidget()
	case dashboard.WidgetTypeFolders:
		return renderFoldersWidget()
	default:
		return "", fmt.Errorf("unknown widget type: %s", widgetType)
	}
}

func renderFilterFormWidget() (string, error) {
	return `<div class="widget-filter-form">
		<form id="metadata-filter-form" hx-post="/api/files/filter" hx-target="#filter-results">
			<div>
				<button type="submit">apply filter</button>
				<select name="logic" id="logic-operator">
					<option value="and">and</option>
					<option value="or">or</option>
				</select>
				<button type="button" onclick="addFilterRow()">add filter</button>
			</div>
			<div id="filter-container">
				<div class="filter-row" id="filter-row-0">
					<select name="metadata[]" id="metadata-0" onchange="updateValueField(0, this.value)">
						<option value="">select field</option>
						<option value="collection">collection</option>
						<option value="tags">tags</option>
						<option value="type">type</option>
						<option value="status">status</option>
						<option value="priority">priority</option>
						<option value="createdAt">created date</option>
						<option value="lastEdited">last edited</option>
						<option value="folders">folders</option>
						<option value="boards">boards</option>
                        <option value="para_projects">para: projects</option>
                        <option value="para_areas">para: areas</option>
                        <option value="para_resources">para: resources</option>
                        <option value="para_archive">para: archive</option>
					</select>
					<select name="operator[]" id="operator-0">
						<option value="equals">equals</option>
						<option value="contains">contains</option>
						<option value="greater">greater than</option>
						<option value="less">less than</option>
						<option value="in">in array</option>
					</select>
					<div id="value-container-0">
						<input type="text" name="value[]" id="value-0" placeholder="value"/>
					</div>
					<select name="action[]" id="action-0">
						<option value="include">include</option>
						<option value="exclude">exclude</option>
					</select>
					<button type="button" onclick="removeFilterRow(0)">remove</button>
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
					'<option value="">select field</option>' +
					'<option value="collection">collection</option>' +
					'<option value="tags">tags</option>' +
					'<option value="type">type</option>' +
					'<option value="status">status</option>' +
					'<option value="priority">priority</option>' +
					'<option value="createdAt">created date</option>' +
					'<option value="lastEdited">last edited</option>' +
					'<option value="folders">folders</option>' +
					'<option value="boards">boards</option>' +
                    '<option value="para_projects">para: projects</option>' +
                    '<option value="para_areas">para: areas</option>' +
                    '<option value="para_resources">para: resources</option>' +
                    '<option value="para_archive">para: archive</option>' +
					'</select>';

				const operatorHTML = '<select name="operator[]" id="operator-' + filterRowCount + '">' +
					'<option value="equals">equals</option>' +
					'<option value="contains">contains</option>' +
					'<option value="greater">greater than</option>' +
					'<option value="less">less than</option>' +
					'<option value="in">in array</option>' +
					'</select>';

				const valueHTML = '<div id="value-container-' + filterRowCount + '">' +
					'<input type="text" name="value[]" id="value-' + filterRowCount + '" placeholder="value"/>' +
					'</div>';

				const actionHTML = '<select name="action[]" id="action-' + filterRowCount + '">' +
					'<option value="include">include</option>' +
					'<option value="exclude">exclude</option>' +
					'</select>';

				const removeHTML = '<button type="button" onclick="removeFilterRow(' + filterRowCount + ')">remove</button>';

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
				container.innerHTML = '<input type="text" name="value[]" autocomplete="off" id="value-' + rowIndex + '" list="collections-' + rowIndex + '" placeholder="type or select collection (supports wildcards: project*)">' +
					'<datalist id="collections-' + rowIndex + '" hx-get="/api/metadata/options/collections" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading collections...</option>' +
					'</datalist>';
			} else if (fieldType === 'tags') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="tags-' + rowIndex + '" placeholder="type or select tag (supports wildcards: para/p/*, zk/*)">' +
					'<datalist id="tags-' + rowIndex + '" hx-get="/api/metadata/options/tags" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading tags...</option>' +
					'</datalist>';
			} else if (fieldType === 'folders') {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" autocomplete="off" list="folders-' + rowIndex + '" placeholder="type or select folder (supports wildcards: guides/*, *temp*)">' +
					'<datalist id="folders-' + rowIndex + '" hx-get="/api/metadata/options/folders" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading folders...</option>' +
					'</datalist>';
			} else if (fieldType === 'type') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/filetypes" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading types...</option>' +
					'</select>';
			} else if (fieldType === 'status') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/status" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading status...</option>' +
					'</select>';
			} else if (fieldType === 'priority') {
				container.innerHTML = '<select name="value[]" id="value-' + rowIndex + '" hx-get="/api/metadata/options/priorities" hx-trigger="load" hx-target="this" hx-swap="innerHTML">' +
					'<option value="">loading priorities...</option>' +
					'</select>';
			} else if (fieldType === 'createdAt' || fieldType === 'lastEdited') {
				container.innerHTML = '<input type="date" name="value[]" id="value-' + rowIndex + '" placeholder="yyyy-mm-dd"/>';
			} else {
				container.innerHTML = '<input type="text" name="value[]" id="value-' + rowIndex + '" placeholder="value"/>';
			}

			// trigger htmx processing for new elements
			if (window.htmx) {
				htmx.process(container);
			}
		}

			// initialize first row
			document.addEventListener('DOMContentLoaded', function() {
				updateValueField(0, document.getElementById('metadata-0').value);
			});
		</script>
	</div>`, nil
}

func renderFileContentWidget(config *dashboard.FileContentConfig) (string, error) {
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

func renderStaticWidget(config *dashboard.StaticConfig) (string, error) {
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

func renderFilterWidget(config *dashboard.FilterConfig) (string, error) {
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
		return RenderFileCards(filteredFiles), nil
	case "dropdown":
		return RenderFileDropdown(filteredFiles, 10), nil
	default:
		return RenderFileList(filteredFiles), nil
	}
}

func renderTagsWidget() (string, error) {
	tagCount, err := files.GetAllTags()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(tagCount, "/browse/tags"), nil
}

func renderCollectionsWidget() (string, error) {
	collectionCount, err := files.GetAllCollections()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(collectionCount, "/browse/collection"), nil
}

func renderFoldersWidget() (string, error) {
	folderCount, err := files.GetAllFolders()
	if err != nil {
		return "", err
	}

	return RenderBrowseHTML(folderCount, "/browse/folders"), nil
}
