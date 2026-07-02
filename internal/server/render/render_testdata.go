// Package render - Test data rendering functions
package render

import (
	"fmt"
	"sort"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/test"
	"knov/internal/translation"
)

// RenderFilterTestResults renders comprehensive filter test results with detailed breakdown
func RenderFilterTestResults(results *test.FilterTestResults) string {
	var html strings.Builder

	html.WriteString(`<div id="component-filter-test-results">`)

	// summary section
	html.WriteString(`<div class="test-summary">`)
	html.WriteString(fmt.Sprintf(`<h4>%s</h4>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Test Results Summary")))

	html.WriteString(`<div class="summary-stats">`)
	html.WriteString(fmt.Sprintf(`<span class="stat total">Total: %d</span>`, results.TotalTests))
	html.WriteString(fmt.Sprintf(`<span class="stat passed">✅ Passed: %d</span>`, results.PassedTests))
	html.WriteString(fmt.Sprintf(`<span class="stat failed">❌ Failed: %d</span>`, results.FailedTests))

	// overall status
	overallStatus := "✅ ALL TESTS PASSED"
	statusClass := "status-ok"
	if results.FailedTests > 0 {
		overallStatus = fmt.Sprintf("⚠️ %d TESTS FAILED", results.FailedTests)
		statusClass = "status-failure"
	}
	html.WriteString(fmt.Sprintf(`<span class="overall-status %s">%s</span>`, statusClass, overallStatus))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// detailed results
	for _, result := range results.Results {
		statusClass := "test-passed"
		if !result.Success {
			statusClass = "test-failed"
		}

		html.WriteString(`<div class="test-result ` + statusClass + `">`)

		// test header with status and basic info
		html.WriteString(`<div class="test-header">`)

		statusIcon := "✅"
		if !result.Success {
			statusIcon = "❌"
		}
		fmt.Fprintf(&html, `<div class="test-title"><span class="status-icon">%s</span><h5>%s</h5></div>`,
			statusIcon, result.ConfigName)

		// counts and description
		html.WriteString(fmt.Sprintf(`<p class="test-description"><strong>%s</strong></p>`, result.Description))
		html.WriteString(`<div class="test-counts">`)
		html.WriteString(fmt.Sprintf(`<span class="count-expected"><strong>Expected:</strong> %d files</span>`, result.ExpectedCount))
		html.WriteString(fmt.Sprintf(`<span class="count-actual"><strong>Actual:</strong> %d files</span>`, result.ActualCount))
		if !result.Success && result.Error != "" {
			html.WriteString(fmt.Sprintf(`<p class="test-error">Error: %s</p>`, result.Error))
		}
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

		// filter configuration - collapsible
		html.WriteString(`<details class="test-config">`)
		html.WriteString(fmt.Sprintf(`<summary>⚙️ %s</summary>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Configuration")))

		html.WriteString(`<div class="config-content">`)
		html.WriteString(fmt.Sprintf(`<p class="config-logic"><strong>Logic:</strong> <code>%s</code></p>`, strings.ToUpper(result.Config.Logic)))
		html.WriteString(`<p class="config-criteria-title"><strong>Criteria:</strong></p>`)
		html.WriteString(`<ul class="config-criteria">`)
		for _, criteria := range result.Config.Criteria {
			actionClass := "action-include"
			if criteria.Action == "exclude" {
				actionClass = "action-exclude"
			}
			html.WriteString(fmt.Sprintf(`<li><code>%s</code> %s <code>"%s"</code> <span class="action-badge %s">%s</span></li>`,
				criteria.Metadata, criteria.Operator, criteria.Value, actionClass, strings.ToUpper(criteria.Action)))
		}
		html.WriteString(`</ul>`)
		html.WriteString(`</div>`)
		html.WriteString(`</details>`)

		// files comparison - collapsible
		html.WriteString(`<details class="test-files">`)
		html.WriteString(fmt.Sprintf(`<summary>📁 %s</summary>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Files Comparison")))

		html.WriteString(`<div class="files-content">`)
		html.WriteString(`<div class="files-grid">`)

		// expected files
		html.WriteString(`<div class="expected-files">`)
		html.WriteString(fmt.Sprintf(`<h6 class="files-header expected">📋 %s (%d)</h6>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Expected Files"),
			len(result.ExpectedFiles)))
		if len(result.ExpectedFiles) > 0 {
			// create sorted copy of expected files
			sortedExpected := make([]string, len(result.ExpectedFiles))
			copy(sortedExpected, result.ExpectedFiles)
			sort.Strings(sortedExpected)

			html.WriteString(`<ul class="files-list">`)
			for _, file := range sortedExpected {
				html.WriteString(fmt.Sprintf(`<li>%s</li>`, file))
			}
			html.WriteString(`</ul>`)
		} else {
			html.WriteString(`<em class="no-files">No files expected</em>`)
		}
		html.WriteString(`</div>`)

		// actual files
		html.WriteString(`<div class="actual-files">`)
		html.WriteString(fmt.Sprintf(`<h6 class="files-header actual">📄 %s (%d)</h6>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Actual Files"),
			len(result.ActualFiles)))
		if len(result.ActualFiles) > 0 {
			// create sorted copy of actual files
			sortedActual := make([]string, len(result.ActualFiles))
			copy(sortedActual, result.ActualFiles)
			sort.Strings(sortedActual)

			html.WriteString(`<ul class="files-list">`)
			for _, file := range sortedActual {
				// highlight if file is unexpected
				liClass := ""
				isExpected := false
				for _, expected := range result.ExpectedFiles {
					if expected == file {
						isExpected = true
						break
					}
				}
				if !isExpected && !result.Success {
					liClass = ` class="unexpected-file"`
				}
				html.WriteString(fmt.Sprintf(`<li%s>%s</li>`, liClass, file))
			}
			html.WriteString(`</ul>`)
		} else {
			html.WriteString(`<em class="no-files">No files found</em>`)
		}
		html.WriteString(`</div>`)

		html.WriteString(`</div>`)
		html.WriteString(`</div>`)
		html.WriteString(`</details>`)

		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderFilterTestMetadataTable renders the filter test metadata in a table format
func RenderFilterTestMetadataTable(metadataList []*files.Metadata) string {
	var html strings.Builder

	html.WriteString(`<div id="component-filter-test-metadata-table">`)
	html.WriteString(fmt.Sprintf(`<h3>📋 %s (%d)</h3>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Test Metadata"),
		len(metadataList)))

	// date legend
	html.WriteString(`<div class="table-date-legend">`)
	html.WriteString(fmt.Sprintf(`<span><strong>%s:</strong> %s (A) +1 day per file</span>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "created"),
		metadataList[0].CreatedAt.Format("2.1.2006")))
	html.WriteString(fmt.Sprintf(`<span><strong>%s:</strong> %s (A) +1 day per file</span>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "last edited"),
		metadataList[0].LastEdited.Format("2.1.2006")))
	html.WriteString(`</div>`)

	// responsive table wrapper
	html.WriteString(`<div class="table-wrapper">`)
	html.WriteString(`<table class="metadata-table">`)

	// table header
	html.WriteString(`<thead>`)
	html.WriteString(`<tr>`)

	// define table columns
	columns := []string{"Path", "Tags", "Parents", "Editor"}
	for _, col := range columns {
		html.WriteString(fmt.Sprintf(`<th>%s</th>`, col))
	}
	html.WriteString(`</tr>`)
	html.WriteString(`</thead>`)

	// table body
	html.WriteString(`<tbody>`)
	for i, metadata := range metadataList {
		rowClass := ""
		if i%2 == 1 {
			rowClass = ` class="alt-row"`
		}

		html.WriteString(fmt.Sprintf(`<tr%s>`, rowClass))

		// path
		html.WriteString(fmt.Sprintf(`<td class="cell-path">%s</td>`, metadata.Path))

		// tags
		tagsStr := strings.Join(metadata.Tags, ", ")
		html.WriteString(fmt.Sprintf(`<td class="cell-tags">%s</td>`, tagsStr))

		// parents
		parentsStr := strings.Join(metadata.Parents, ", ")
		html.WriteString(fmt.Sprintf(`<td class="cell-parents">%s</td>`, parentsStr))

		// editor type with color coding
		editorClass := "type-markdown"
		switch string(metadata.Editor) {
		case "toastui-editor":
			editorClass = "type-markdown"
		case "textarea-editor":
			editorClass = "type-text"
		case "list-editor":
			editorClass = "type-list"
		case "todo-editor":
			editorClass = "type-todo"
		case "filter-editor":
			editorClass = "type-filter"
		case "index-editor":
			editorClass = "type-index"
		}
		html.WriteString(fmt.Sprintf(`<td class="cell-filetype"><span class="badge %s">%s</span></td>`, editorClass, string(metadata.Editor)))

		html.WriteString(`</tr>`)
	}
	html.WriteString(`</tbody>`)
	html.WriteString(`</table>`)
	html.WriteString(`</div>`)

	// summary stats
	html.WriteString(`<div class="table-summary">`)
	html.WriteString(fmt.Sprintf(`<strong>Total Files:</strong> %d`, len(metadataList)))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}
