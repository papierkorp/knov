// Package render - HTMX HTML rendering functions for server responses
package render

import (
	"fmt"
	"net/url"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/translation"
)

// SelectOption represents an option in a select dropdown
type SelectOption struct {
	Value string
	Label string
}

// RenderInputField renders an input field with specified type, name, id, value and placeholder
func RenderInputField(inputType, name, id, value, placeholder string, required bool) string {
	requiredAttr := ""
	if required {
		requiredAttr = "required"
	}
	return fmt.Sprintf(`<input type="%s" name="%s" id="%s" value="%s" placeholder="%s" %s />`,
		inputType, name, id, value, placeholder, requiredAttr)
}

// StatusClass represents valid status message classes
type StatusClass string

const (
	StatusOK      StatusClass = "status-ok"
	StatusError   StatusClass = "status-error"
	StatusWarning StatusClass = "status-warning"
	StatusInfo    StatusClass = "status-info"
)

// RenderStatusMessage renders a status message span with predefined status class
func RenderStatusMessage(class StatusClass, message string) string {
	return fmt.Sprintf(`<span class="%s">%s</span>`, string(class), message)
}

// RenderStatusMessageWithLink renders a status message span with a link
func RenderStatusMessageWithLink(class StatusClass, message, linkURL, linkText string) string {
	return fmt.Sprintf(`<span class="%s">%s: <a href="%s">%s</a></span>`,
		string(class), message, linkURL, linkText)
}

// RenderSelectOptions renders option elements for select dropdown
func RenderSelectOptions(options []SelectOption, selectedValue string) string {
	var html string
	for _, option := range options {
		selected := ""
		if option.Value == selectedValue {
			selected = "selected"
		}
		html += fmt.Sprintf(`<option value="%s" %s>%s</option>`, option.Value, selected, option.Label)
	}
	return html
}

// RenderCheckbox renders a checkbox input with htmx attributes
func RenderCheckbox(name, endpoint string, checked bool, extraAttrs string) string {
	checkedAttr := ""
	if checked {
		checkedAttr = "checked"
	}

	baseAttrs := fmt.Sprintf(`type="checkbox" name="%s" %s hx-post="%s"`, name, checkedAttr, endpoint)
	if extraAttrs != "" {
		baseAttrs += " " + extraAttrs
	}

	return fmt.Sprintf(`<input %s />`, baseAttrs)
}

// RenderTextarea renders a textarea with specified attributes
func RenderTextarea(name, content string, rows int, extraAttrs string) string {
	baseAttrs := fmt.Sprintf(`name="%s" rows="%d"`, name, rows)
	if extraAttrs != "" {
		baseAttrs += " " + extraAttrs
	}
	return fmt.Sprintf(`<textarea %s>%s</textarea>`, baseAttrs, content)
}

// RenderFileCards renders files as cards without search context
func RenderFileCards(files []files.File) string {
	var html strings.Builder
	html.WriteString(`<div class="search-results-cards">`)

	for _, file := range files {
		displayText := GetLinkDisplayText(file.Path)
		html.WriteString(fmt.Sprintf(`
			<div class="search-result-card">
				<h4><a href="/files/%s">%s</a></h4>
			</div>`,
			file.Path, displayText))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderFileList renders files as simple list without search context
func RenderFileList(files []files.File) string {
	var html strings.Builder
	html.WriteString(`<ul class="search-results-simple-list">`)

	for _, file := range files {
		displayText := GetLinkDisplayText(file.Path)
		html.WriteString(fmt.Sprintf(`
			<li><a href="/files/%s">%s</a></li>`,
			file.Path, displayText))
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// RenderFileDropdown renders files as dropdown list with limit
func RenderFileDropdown(files []files.File, limit int) string {
	var html strings.Builder
	html.WriteString(`<div id="filter-results">`)
	html.WriteString(`<select class="form-select" onchange="if(this.value) window.location.href='/files/'+this.value">`)
	html.WriteString(`<option value="">` + translation.SprintfForRequest(configmanager.GetLanguage(), "select file...") + `</option>`)

	displayLimit := limit
	if limit <= 0 {
		displayLimit = len(files)
	}

	for i, file := range files {
		if i >= displayLimit {
			break
		}
		displayText := GetLinkDisplayText(file.Path)
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, file.Path, displayText))
	}

	if len(files) == 0 {
		html.WriteString(`<option disabled>` + translation.SprintfForRequest(configmanager.GetLanguage(), "no results found") + `</option>`)
	}

	html.WriteString(`</select>`)
	html.WriteString(`</div>`)
	return html.String()
}

// DONT RENAME filez to files since files.GetFileContent is not working than!!
// RenderFileContent renders files with their actual content displayed
func RenderFileContent(filez []files.File) string {
	var html strings.Builder
	html.WriteString(`<div id="filter-results" class="filter-content-results">`)

	for _, file := range filez {
		displayText := GetLinkDisplayText(file.Path)
		html.WriteString(fmt.Sprintf(`<div class="filter-content-item">
			<h4><a href="/files/%s">%s</a></h4>
			<div class="filter-content-body">`, file.Path, displayText))

		// get file content
		fullPath := contentStorage.ToDocsPath(file.Path)
		content, err := files.GetFileContent(fullPath)
		if err != nil {
			html.WriteString(`<p class="filter-content-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "error loading content: %s", err.Error()) + `</p>`)
		} else {
			html.WriteString(content.HTML)
		}

		html.WriteString(`</div></div>`)
	}

	if len(filez) == 0 {
		html.WriteString(`<p class="filter-no-results">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no files found matching filter criteria") + `</p>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// GetFormValue returns form value at index, or empty string if index out of bounds
func GetFormValue(slice []string, index int) string {
	if index < len(slice) {
		return slice[index]
	}
	return ""
}

// GenerateDatalistInput creates an input field with autocomplete (without save)
func GenerateDatalistInput(id, name, value, placeholder, apiEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" autocomplete="off" list="%s" placeholder="%s"/>
<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
	<option value="">%s</option>
</datalist>`, id, name, value, datalistId, placeholder, datalistId, apiEndpoint, translation.SprintfForRequest(configmanager.GetLanguage(), "loading options..."))
}

// GenerateDatalistInputWithSave creates an input field with autocomplete and auto-save
func GenerateDatalistInputWithSave(id, name, value, placeholder, apiEndpoint, filePath, saveEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" autocomplete="off" list="%s" placeholder="%s"
	hx-post="%s" hx-vals='{"filepath": "%s"}' hx-trigger="input delay:500ms" hx-target="#metadata-save-status" hx-swap="innerHTML"/>
<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
	<option value="">%s</option>
</datalist>`, id, name, value, datalistId, placeholder, saveEndpoint, filePath, datalistId, apiEndpoint, translation.SprintfForRequest(configmanager.GetLanguage(), "loading options..."))
}

// GenerateDateInputWithSave creates a date input field with auto-save
func GenerateDateInputWithSave(id, name, value, filePath, saveEndpoint string) string {
	return fmt.Sprintf(`<input type="date" id="%s" name="%s" value="%s" class="form-input"
	hx-post="%s" hx-vals='{"filepath": "%s"}' hx-trigger="change" hx-target="#metadata-save-status" hx-swap="innerHTML"/>`, id, name, value, saveEndpoint, filePath)
}

// GenerateTagChipsInputWithSave creates a tag chips input with autocomplete and auto-save
func GenerateTagChipsInputWithSave(id, name, value, placeholder, apiEndpoint, filePath, saveEndpoint string) string {
	datalistId := fmt.Sprintf("%s-list", id)
	chipsId := fmt.Sprintf("%s-chips", id)
	inputId := fmt.Sprintf("%s-input", id)
	hiddenId := fmt.Sprintf("%s-hidden", id)

	var datalistHTML string
	if apiEndpoint != "" {
		datalistHTML = fmt.Sprintf(`<datalist id="%s" hx-get="%s" hx-trigger="load" hx-target="this" hx-swap="innerHTML">
		<option value="">%s</option>
	</datalist>`, datalistId, apiEndpoint, translation.SprintfForRequest(configmanager.GetLanguage(), "loading options..."))
	} else {
		datalistHTML = fmt.Sprintf(`<datalist id="%s"></datalist>`, datalistId)
	}

	return fmt.Sprintf(`<div class="tag-chips-container" id="%s">
	<div class="tag-chips" id="%s-display"></div>
	<input type="text" id="%s" class="tag-chips-input" autocomplete="off" list="%s" placeholder="%s"/>
	<input type="hidden" id="%s" name="%s" value="%s"
		hx-post="%s" hx-vals='{"filepath": "%s"}' hx-trigger="change delay:500ms" hx-target="#metadata-save-status" hx-swap="innerHTML"/>
	%s
</div>
<script>
(function() {
	const container = document.getElementById('%s');
	const display = document.getElementById('%s-display');
	const input = document.getElementById('%s');
	const hidden = document.getElementById('%s');

	let tags = [];
	let initialized = false;

	// initialize with existing values (no auto-save)
	if (hidden.value) {
		tags = hidden.value.split(',').map(t => t.trim()).filter(t => t);
		renderTags();
	}

	function renderTags() {
		display.innerHTML = '';
		tags.forEach((tag, index) => {
			const chip = document.createElement('span');
			chip.className = 'tag-chip';
			chip.innerHTML = tag + '<button type="button" class="tag-chip-remove">&times;</button>';

			const removeBtn = chip.querySelector('.tag-chip-remove');
			removeBtn.addEventListener('click', function() {
				removeTag(index);
			});

			display.appendChild(chip);
		});
		hidden.value = tags.join(', ');
		// only trigger save after initialization
		if (initialized) {
			htmx.trigger(hidden, 'change');
		}
	}

	function addTag(value) {
		const trimmed = value.trim();
		if (trimmed && !tags.includes(trimmed)) {
			tags.push(trimmed);
			renderTags();
			input.value = '';
		}
	}

	function removeTag(index) {
		tags.splice(index, 1);
		renderTags();
	}

	// handle input events
	input.addEventListener('keydown', function(e) {
		if (e.key === 'Enter' || e.key === ',' || e.key === 'Tab') {
			e.preventDefault();
			addTag(input.value);
		} else if (e.key === 'Backspace' && input.value === '' && tags.length > 0) {
			tags.pop();
			renderTags();
		}
	});

	// handle datalist selection
	input.addEventListener('change', function() {
		if (input.value) {
			addTag(input.value);
		}
	});

	// handle blur to catch paste events
	input.addEventListener('blur', function() {
		if (input.value) {
			addTag(input.value);
		}
	});

	// make container clickable to focus input
	container.addEventListener('click', function() {
		input.focus();
	});

	// mark as initialized after setup
	setTimeout(function() {
		initialized = true;
	}, 100);
})();
</script>`, chipsId, chipsId, inputId, datalistId, placeholder, hiddenId, name, value, saveEndpoint, filePath, datalistHTML, chipsId, chipsId, inputId, hiddenId)
}

// GenerateInputWithSaveOnBlur creates an input field that only saves when user leaves the field
func GenerateInputWithSaveOnBlur(id, name, value, placeholder, filePath, saveEndpoint string) string {
	return fmt.Sprintf(`<input type="text" id="%s" name="%s" value="%s" class="form-input" placeholder="%s"
	hx-post="%s" hx-vals='{"filepath": "%s"}' hx-trigger="blur" hx-target="#metadata-save-status" hx-swap="innerHTML"/>`,
		id, name, value, placeholder, saveEndpoint, filePath)
}

// RenderBrowseHTML renders a map of items with counts as browse links
func RenderBrowseHTML(items map[string]int, urlPrefix string) string {
	var html strings.Builder
	html.WriteString(`<ul class="search-results-simple-list">`)

	for item, count := range items {
		html.WriteString(fmt.Sprintf(`
			<li><a href="%s/%s">%s (%d)</a></li>`,
			urlPrefix, url.QueryEscape(item), item, count))
	}

	html.WriteString(`</ul>`)
	return html.String()
}

// RenderMetadataLinksHTML creates HTML links for metadata items (tags, folders, collections)
func RenderMetadataLinksHTML(items []string, browseType string) string {
	if len(items) == 0 {
		return `<span class="meta-empty">-</span>`
	}

	var html strings.Builder
	for i, item := range items {
		if i > 0 {
			html.WriteString(", ")
		}
		html.WriteString(fmt.Sprintf(`<a href="/browse/%s/%s" class="meta-link">%s</a>`, browseType, item, item))
	}

	return html.String()
}

// RenderMetadataLinkHTML creates a single HTML link for metadata (e.g., collection)
func RenderMetadataLinkHTML(item string, browseType string) string {
	if item == "" {
		return `<span class="meta-empty">-</span>`
	}

	return fmt.Sprintf(`<a href="/browse/%s/%s" class="meta-link">%s</a>`, browseType, item, item)
}
