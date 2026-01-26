// Package render - Test data rendering functions
package render

import (
	"fmt"
	"sort"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/testdata"
	"knov/internal/translation"
)

// RenderFilterTestResults renders comprehensive filter test results with detailed breakdown
func RenderFilterTestResults(results *testdata.FilterTestResults) string {
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
	statusClass := "status-success"
	if results.FailedTests > 0 {
		overallStatus = fmt.Sprintf("⚠️ %d TESTS FAILED", results.FailedTests)
		statusClass = "status-failure"
	}
	html.WriteString(fmt.Sprintf(`<span class="overall-status %s">%s</span>`, statusClass, overallStatus))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// detailed results
	for i, result := range results.Results {
		statusClass := "test-passed"
		if !result.Success {
			statusClass = "test-failed"
		}

		html.WriteString(`<div class="test-result ` + statusClass + `">`)

		// test header with status and basic info
		html.WriteString(`<div class="test-header">`)
		html.WriteString(fmt.Sprintf(`<div class="test-title">
			<h5>Test %d: %s</h5>`, i+1, result.ConfigName))

		statusIcon := "✅"
		if !result.Success {
			statusIcon = "❌"
		}
		html.WriteString(fmt.Sprintf(`<span class="status-icon">%s</span>`, statusIcon))
		html.WriteString(`</div>`)

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

	// responsive table wrapper
	html.WriteString(`<div class="table-wrapper">`)
	html.WriteString(`<table class="metadata-table">`)

	// table header
	html.WriteString(`<thead>`)
	html.WriteString(`<tr>`)

	// define table columns
	columns := []struct {
		key   string
		width string
	}{
		{"Name", "10%"},
		{"Path", "15%"},
		{"Collection", "12%"},
		{"Tags", "20%"},
		{"FileType", "8%"},
		{"Status", "7%"},
		{"Priority", "7%"},
		{"PARA Projects", "21%"},
	}

	for _, col := range columns {
		html.WriteString(fmt.Sprintf(`<th class="col-width-%s">%s</th>`, strings.ReplaceAll(col.width, "%", ""), col.key))
	}
	html.WriteString(`</tr>`)
	html.WriteString(`</thead>`)

	// table body
	html.WriteString(`<tbody>`)
	for i, metadata := range metadataList {
		// alternate row colors
		rowClass := ""
		if i%2 == 1 {
			rowClass = ` class="alt-row"`
		}

		html.WriteString(fmt.Sprintf(`<tr%s>`, rowClass))

		// name
		html.WriteString(fmt.Sprintf(`<td class="cell-name">%s</td>`, metadata.Name))

		// path
		html.WriteString(fmt.Sprintf(`<td class="cell-path">%s</td>`, metadata.Path))

		// collection
		html.WriteString(fmt.Sprintf(`<td class="cell-collection">%s</td>`, metadata.Collection))

		// tags
		tagsStr := strings.Join(metadata.Tags, ", ")
		html.WriteString(fmt.Sprintf(`<td class="cell-tags">%s</td>`, tagsStr))

		// file type with color coding
		fileTypeClass := "type-permanent"
		switch string(metadata.FileType) {
		case "fleeting":
			fileTypeClass = "type-fleeting"
		case "literature":
			fileTypeClass = "type-literature"
		case "permanent":
			fileTypeClass = "type-permanent"
		case "journaling":
			fileTypeClass = "type-journaling"
		case "moc":
			fileTypeClass = "type-moc"
		}
		html.WriteString(fmt.Sprintf(`<td class="cell-filetype"><span class="badge %s">%s</span></td>`, fileTypeClass, string(metadata.FileType)))

		// status with color coding
		statusClass := "status-published"
		switch string(metadata.Status) {
		case "draft":
			statusClass = "status-draft"
		case "published":
			statusClass = "status-published"
		case "archived":
			statusClass = "status-archived"
		}
		html.WriteString(fmt.Sprintf(`<td class="cell-status"><span class="badge %s">%s</span></td>`, statusClass, string(metadata.Status)))

		// priority with color coding
		priorityClass := "priority-medium"
		switch string(metadata.Priority) {
		case "high":
			priorityClass = "priority-high"
		case "medium":
			priorityClass = "priority-medium"
		case "low":
			priorityClass = "priority-low"
		}
		html.WriteString(fmt.Sprintf(`<td class="cell-priority"><span class="badge %s">%s</span></td>`, priorityClass, string(metadata.Priority)))

		// para projects
		projectsStr := strings.Join(metadata.PARA.Projects, ", ")
		html.WriteString(fmt.Sprintf(`<td class="cell-projects">%s</td>`, projectsStr))

		html.WriteString(`</tr>`)
	}
	html.WriteString(`</tbody>`)
	html.WriteString(`</table>`)
	html.WriteString(`</div>`)

	// summary stats
	html.WriteString(`<div class="table-summary">`)
	html.WriteString(fmt.Sprintf(`<strong>Total Files:</strong> %d | `, len(metadataList)))

	// count by status
	statusCounts := make(map[string]int)
	for _, meta := range metadataList {
		statusCounts[string(meta.Status)]++
	}
	html.WriteString(`<strong>Status:</strong> `)
	for status, count := range statusCounts {
		html.WriteString(fmt.Sprintf(`%s: %d | `, status, count))
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}
