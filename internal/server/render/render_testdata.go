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

	html.WriteString(`<div class="filter-test-results">`)

	// summary section with improved styling and better contrast
	html.WriteString(`<div class="test-summary" style="background: #ffffff; border: 2px solid #e9ecef; padding: 20px; border-radius: 8px; margin-bottom: 25px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">`)
	html.WriteString(fmt.Sprintf(`<h4 style="margin: 0 0 15px 0; color: #212529; font-size: 1.5em;">%s</h4>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Test Results Summary")))

	html.WriteString(`<div class="summary-stats" style="display: flex; gap: 20px; align-items: center; flex-wrap: wrap;">`)
	html.WriteString(fmt.Sprintf(`<span class="stat total" style="font-weight: bold; color: #495057; font-size: 1.1em;">Total: %d</span>`, results.TotalTests))
	html.WriteString(fmt.Sprintf(`<span class="stat passed" style="font-weight: bold; color: #155724; font-size: 1.1em;">✅ Passed: %d</span>`, results.PassedTests))
	html.WriteString(fmt.Sprintf(`<span class="stat failed" style="font-weight: bold; color: #721c24; font-size: 1.1em;">❌ Failed: %d</span>`, results.FailedTests))

	// overall status
	overallStatus := "✅ ALL TESTS PASSED"
	statusColor := "#155724"
	statusBg := "#d1e7dd"
	if results.FailedTests > 0 {
		overallStatus = fmt.Sprintf("⚠️ %d TESTS FAILED", results.FailedTests)
		statusColor = "#721c24"
		statusBg = "#f8d7da"
	}
	html.WriteString(fmt.Sprintf(`<span style="margin-left: auto; font-weight: bold; color: %s; background: %s; padding: 8px 16px; border-radius: 4px; font-size: 1.2em;">%s</span>`, statusColor, statusBg, overallStatus))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// log file download link (if available)
	if results.LogFile != "" {
		html.WriteString(`<div class="test-log-download" style="background: #e7f3ff; border: 2px solid #0077cc; padding: 15px; border-radius: 8px; margin-bottom: 25px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">`)
		html.WriteString(fmt.Sprintf(`<div style="display: flex; align-items: center; gap: 15px;">
			<span style="font-size: 2em;">📋</span>
			<div style="flex: 1;">
				<h5 style="margin: 0 0 5px 0; color: #0056b3; font-size: 1.2em;">%s</h5>
				<p style="margin: 0; color: #495057; font-size: 0.95em;">%s</p>
			</div>
			<a href="/api/testdata/filtertest/log?key=%s" download="filter-test-log.txt" style="background: #0077cc; color: white; padding: 10px 20px; border-radius: 5px; text-decoration: none; font-weight: bold; white-space: nowrap; box-shadow: 0 2px 4px rgba(0,0,0,0.2);">
				⬇️ %s
			</a>
		</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Test Execution Log"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "download detailed log file with complete test execution output"),
			results.LogFile,
			translation.SprintfForRequest(configmanager.GetLanguage(), "download log")))
		html.WriteString(`</div>`)
	}

	// detailed results
	for i, result := range results.Results {
		statusClass := "test-passed"
		statusBgColor := "#d1e7dd"
		borderColor := "#badbcc"
		textColor := "#0f5132"

		if !result.Success {
			statusClass = "test-failed"
			statusBgColor = "#f8d7da"
			borderColor = "#f5c6cb"
			textColor = "#721c24"
		}

		html.WriteString(`<div class="test-result ` + statusClass + `" style="margin: 20px 0; background: white; border-radius: 5px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); overflow: hidden; border-left: 5px solid ` + borderColor + `;">`)

		// test header with status and basic info
		html.WriteString(`<div style="background: ` + statusBgColor + `; padding: 15px; border-bottom: 1px solid #dee2e6;">`)
		html.WriteString(fmt.Sprintf(`<div style="display: flex; align-items: center; gap: 15px; flex-wrap: wrap;">
			<h5 style="margin: 0; color: %s; font-size: 1.3em;">Test %d: %s</h5>`, textColor, i+1, result.ConfigName))

		statusIcon := "✅"
		if !result.Success {
			statusIcon = "❌"
		}
		html.WriteString(fmt.Sprintf(`<span style="font-weight: bold; color: %s; font-size: 1.2em;">%s</span>`, textColor, statusIcon))
		html.WriteString(`</div>`)

		// counts and description
		html.WriteString(fmt.Sprintf(`<p style="margin: 10px 0 0 0; color: %s; font-size: 1.05em;"><strong>%s</strong></p>`, textColor, result.Description))
		html.WriteString(fmt.Sprintf(`<div style="margin: 10px 0 0 0; color: %s; font-size: 1.05em;">`, textColor))
		html.WriteString(fmt.Sprintf(`<span style="margin-right: 20px;"><strong>Expected:</strong> %d files</span>`, result.ExpectedCount))
		html.WriteString(fmt.Sprintf(`<span><strong>Actual:</strong> %d files</span>`, result.ActualCount))
		if !result.Success && result.Error != "" {
			html.WriteString(fmt.Sprintf(`<p style="margin: 5px 0 0 0; color: #dc3545; font-weight: bold;">Error: %s</p>`, result.Error))
		}
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

		// filter configuration - collapsible with improved styling
		html.WriteString(`<details class="test-config" style="margin: 0; background: white; overflow: hidden;">`)
		html.WriteString(fmt.Sprintf(`<summary style="cursor: pointer; font-weight: bold; color: #495057; background: #f8f9fa; padding: 10px 15px; border-bottom: 1px solid #dee2e6;">⚙️ %s</summary>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Configuration")))

		html.WriteString(`<div style="background: white; color: #212529; padding: 15px; border-left: 4px solid #007bff;">`)
		html.WriteString(fmt.Sprintf(`<p style="margin: 0 0 10px 0; color: #212529;"><strong>Logic:</strong> <code style="background: #f8f9fa; padding: 2px 6px; border-radius: 3px; color: #495057;">%s</code></p>`, strings.ToUpper(result.Config.Logic)))
		html.WriteString(`<p style="margin: 0 0 10px 0; color: #212529;"><strong>Criteria:</strong></p>`)
		html.WriteString(`<ul style="margin-left: 20px; color: #212529;">`)
		for _, criteria := range result.Config.Criteria {
			actionColor := "#155724"
			actionBg := "#d1e7dd"
			if criteria.Action == "exclude" {
				actionColor = "#721c24"
				actionBg = "#f8d7da"
			}
			html.WriteString(fmt.Sprintf(`<li style="margin: 5px 0; color: #212529;"><code style="background: #f8f9fa; padding: 2px 6px; border-radius: 3px; color: #495057;">%s</code> %s <code style="background: #f8f9fa; padding: 2px 6px; border-radius: 3px; color: #495057;">"%s"</code> <span style="color: %s; background: %s; padding: 2px 8px; border-radius: 3px; font-weight: bold; font-size: 0.85em;">%s</span></li>`,
				criteria.Metadata, criteria.Operator, criteria.Value, actionColor, actionBg, strings.ToUpper(criteria.Action)))
		}
		html.WriteString(`</ul>`)
		html.WriteString(`</div>`)
		html.WriteString(`</details>`)

		// files comparison - collapsible with improved readability
		html.WriteString(`<details class="test-files" style="margin: 15px 0; background: white; border-radius: 5px; overflow: hidden;">`)
		html.WriteString(fmt.Sprintf(`<summary style="cursor: pointer; font-weight: bold; color: #495057; background: #f8f9fa; padding: 10px 15px; border-bottom: 1px solid #dee2e6;">📁 %s</summary>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Files Comparison")))

		html.WriteString(`<div style="background: white; color: #212529; padding: 15px; border-left: 4px solid #28a745;">`)
		html.WriteString(`<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 25px;">`)

		// expected files with better contrast
		html.WriteString(`<div class="expected-files">`)
		html.WriteString(fmt.Sprintf(`<h6 style="color: #155724; margin: 0 0 10px 0; background: #d1e7dd; padding: 8px 12px; border-radius: 4px; font-size: 1.05em;">📁‹ %s (%d)</h6>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Expected Files"),
			len(result.ExpectedFiles)))
		if len(result.ExpectedFiles) > 0 {
			// create sorted copy of expected files
			sortedExpected := make([]string, len(result.ExpectedFiles))
			copy(sortedExpected, result.ExpectedFiles)
			sort.Strings(sortedExpected)

			html.WriteString(`<ul style="margin: 0; padding-left: 20px; font-family: 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 0.9em; color: #212529;">`)
			for _, file := range sortedExpected {
				html.WriteString(fmt.Sprintf(`<li style="margin: 3px 0; padding: 2px 0; color: #212529;">%s</li>`, file))
			}
			html.WriteString(`</ul>`)
		} else {
			html.WriteString(`<em style="color: #6c757d; font-size: 0.95em;">No files expected</em>`)
		}
		html.WriteString(`</div>`)

		// actual files with better contrast
		html.WriteString(`<div class="actual-files">`)
		html.WriteString(fmt.Sprintf(`<h6 style="color: #721c24; margin: 0 0 10px 0; background: #f8d7da; padding: 8px 12px; border-radius: 4px; font-size: 1.05em;">📄„ %s (%d)</h6>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "Actual Files"),
			len(result.ActualFiles)))
		if len(result.ActualFiles) > 0 {
			// create sorted copy of actual files
			sortedActual := make([]string, len(result.ActualFiles))
			copy(sortedActual, result.ActualFiles)
			sort.Strings(sortedActual)

			html.WriteString(`<ul style="margin: 0; padding-left: 20px; font-family: 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 0.9em; color: #212529;">`)
			for _, file := range sortedActual {
				// highlight if file is unexpected
				style := "margin: 3px 0; padding: 2px 0; color: #212529;"
				isExpected := false
				for _, expected := range result.ExpectedFiles {
					if expected == file {
						isExpected = true
						break
					}
				}
				if !isExpected && !result.Success {
					style = "margin: 3px 0; padding: 2px 6px; color: #721c24; font-weight: bold; background: #f8d7da; border-radius: 3px;"
				}
				html.WriteString(fmt.Sprintf(`<li style="%s">%s</li>`, style, file))
			}
			html.WriteString(`</ul>`)
		} else {
			html.WriteString(`<em style="color: #6c757d; font-size: 0.95em;">No files found</em>`)
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

	html.WriteString(`<div class="filter-test-metadata-table">`)
	html.WriteString(fmt.Sprintf(`<h3 style="margin: 0 0 20px 0; color: #212529;">📁‹ %s (%d)</h3>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "Filter Test Metadata"),
		len(metadataList)))

	// responsive table wrapper with improved styling
	html.WriteString(`<div style="overflow-x: auto; border: 1px solid #dee2e6; border-radius: 8px; background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">`)
	html.WriteString(`<table style="width: 100%; border-collapse: collapse; font-size: 0.9em; min-width: 1200px;">`)

	// table header
	html.WriteString(`<thead style="background: #f8f9fa; border-bottom: 2px solid #dee2e6;">`)
	html.WriteString(`<tr>`)

	// define table columns with better widths for full content
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
		html.WriteString(fmt.Sprintf(`<th style="padding: 15px 10px; text-align: left; font-weight: bold; color: #495057; width: %s; border-right: 1px solid #dee2e6;">%s</th>`, col.width, col.key))
	}
	html.WriteString(`</tr>`)
	html.WriteString(`</thead>`)

	// table body
	html.WriteString(`<tbody>`)
	for i, metadata := range metadataList {
		// alternate row colors
		bgColor := "#ffffff"
		if i%2 == 1 {
			bgColor = "#f8f9fa"
		}

		html.WriteString(fmt.Sprintf(`<tr style="background: %s; border-bottom: 1px solid #dee2e6;">`, bgColor))

		// name
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; font-family: 'Consolas', 'Monaco', 'Courier New', monospace; color: #212529; font-weight: bold; border-right: 1px solid #dee2e6;">%s</td>`, metadata.Name))

		// path
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; font-family: 'Consolas', 'Monaco', 'Courier New', monospace; color: #6c757d; font-size: 0.85em; border-right: 1px solid #dee2e6;">%s</td>`, metadata.Path))

		// collection
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; color: #495057; border-right: 1px solid #dee2e6;">%s</td>`, metadata.Collection))

		// tags - full content with word wrapping
		tagsStr := strings.Join(metadata.Tags, ", ")
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; color: #007bff; font-size: 0.85em; word-wrap: break-word; line-height: 1.4; border-right: 1px solid #dee2e6;">%s</td>`, tagsStr))

		// file type with color coding
		fileTypeColor := "#28a745"
		switch string(metadata.FileType) {
		case "fleeting":
			fileTypeColor = "#ffc107"
		case "literature":
			fileTypeColor = "#17a2b8"
		case "permanent":
			fileTypeColor = "#28a745"
		case "journaling":
			fileTypeColor = "#6610f2"
		case "moc":
			fileTypeColor = "#fd7e14"
		}
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; border-right: 1px solid #dee2e6;"><span style="background: %s; color: white; padding: 4px 8px; border-radius: 3px; font-size: 0.8em; font-weight: bold;">%s</span></td>`, fileTypeColor, string(metadata.FileType)))

		// status with color coding
		statusColor := "#28a745"
		switch string(metadata.Status) {
		case "draft":
			statusColor = "#ffc107"
		case "published":
			statusColor = "#28a745"
		case "archived":
			statusColor = "#6c757d"
		}
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; border-right: 1px solid #dee2e6;"><span style="background: %s; color: white; padding: 4px 8px; border-radius: 3px; font-size: 0.8em; font-weight: bold;">%s</span></td>`, statusColor, string(metadata.Status)))

		// priority with color coding
		priorityColor := "#6c757d"
		switch string(metadata.Priority) {
		case "high":
			priorityColor = "#dc3545"
		case "medium":
			priorityColor = "#ffc107"
		case "low":
			priorityColor = "#28a745"
		}
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; border-right: 1px solid #dee2e6;"><span style="background: %s; color: white; padding: 4px 8px; border-radius: 3px; font-size: 0.8em; font-weight: bold;">%s</span></td>`, priorityColor, string(metadata.Priority)))

		// para projects - full content with word wrapping
		projectsStr := strings.Join(metadata.PARA.Projects, ", ")
		html.WriteString(fmt.Sprintf(`<td style="padding: 12px 10px; color: #495057; font-size: 0.85em; word-wrap: break-word; line-height: 1.4;">%s</td>`, projectsStr))

		html.WriteString(`</tr>`)
	}
	html.WriteString(`</tbody>`)
	html.WriteString(`</table>`)
	html.WriteString(`</div>`)

	// summary stats
	html.WriteString(`<div style="margin-top: 15px; padding: 10px; background: #f8f9fa; border-radius: 5px; font-size: 0.9em; color: #495057;">`)
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
