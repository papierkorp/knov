// Package render - Test data rendering functions
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/test"
	"knov/internal/translation"
)

// RenderSuiteResult renders a SuiteResult (from any test suite, or RunAllTests) with a
// per-case breakdown. Generic across suites since CaseResult.Expected/Actual are free-form.
func RenderSuiteResult(results *test.SuiteResult) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var html strings.Builder

	html.WriteString(`<div id="component-filter-test-results">`)

	// summary section
	html.WriteString(`<div class="test-summary">`)
	html.WriteString(fmt.Sprintf(`<h4>%s</h4>`, t("Test Results Summary")))

	html.WriteString(`<div class="summary-stats">`)
	html.WriteString(fmt.Sprintf(`<span class="stat total">%s</span>`, t("Total: %d", results.Total)))
	html.WriteString(fmt.Sprintf(`<span class="stat passed">✅ %s</span>`, t("Passed: %d", results.Passed)))
	html.WriteString(fmt.Sprintf(`<span class="stat failed">❌ %s</span>`, t("Failed: %d", results.Failed)))

	// overall status
	overallStatus := "✅ " + t("ALL TESTS PASSED")
	statusClass := "status-ok"
	if results.Failed > 0 {
		overallStatus = "⚠️ " + t("%d TESTS FAILED", results.Failed)
		statusClass = "status-failure"
	}
	html.WriteString(fmt.Sprintf(`<span class="overall-status %s">%s</span>`, statusClass, overallStatus))
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)

	// detailed results
	for _, c := range results.Cases {
		statusClass := "test-passed"
		if !c.Success {
			statusClass = "test-failed"
		}

		html.WriteString(`<div class="test-result ` + statusClass + `">`)

		// test header with status and basic info
		html.WriteString(`<div class="test-header">`)

		statusIcon := "✅"
		if !c.Success {
			statusIcon = "❌"
		}
		fmt.Fprintf(&html, `<div class="test-title"><span class="status-icon">%s</span><h5>%s</h5></div>`,
			statusIcon, c.Name)

		html.WriteString(`<div class="test-counts">`)
		html.WriteString(fmt.Sprintf(`<span class="count-expected"><strong>%s:</strong> %s</span>`, t("Expected"), c.Expected))
		html.WriteString(fmt.Sprintf(`<span class="count-actual"><strong>%s:</strong> %s</span>`, t("Actual"), c.Actual))
		if !c.Success && c.Error != "" {
			html.WriteString(fmt.Sprintf(`<p class="test-error">%s: %s</p>`, t("Error"), c.Error))
		}
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)

		// suite-specific detail - collapsible
		if cfg, ok := c.Detail.(filter.Config); ok {
			html.WriteString(`<details class="test-config">`)
			html.WriteString(fmt.Sprintf(`<summary>⚙️ %s</summary>`, t("Filter Configuration")))

			html.WriteString(`<div class="config-content">`)
			html.WriteString(fmt.Sprintf(`<p class="config-logic"><strong>%s:</strong> <code>%s</code></p>`, t("Logic"), strings.ToUpper(cfg.Logic)))
			html.WriteString(fmt.Sprintf(`<p class="config-criteria-title"><strong>%s:</strong></p>`, t("Criteria")))
			html.WriteString(`<ul class="config-criteria">`)
			for _, criteria := range cfg.Criteria {
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
		}

		html.WriteString(`</div>`)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// RenderFilterTestMetadataTable renders the filter test metadata in a table format
func RenderFilterTestMetadataTable(metadataList []*files.Metadata) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var html strings.Builder

	html.WriteString(`<div id="component-filter-test-metadata-table">`)
	html.WriteString(fmt.Sprintf(`<h3>📋 %s (%d)</h3>`, t("Filter Test Metadata"), len(metadataList)))

	// date legend
	html.WriteString(`<div class="table-date-legend">`)
	html.WriteString(fmt.Sprintf(`<span><strong>%s:</strong> %s (A) +1 day per file</span>`,
		t("created"),
		metadataList[0].CreatedAt.Format("2.1.2006")))
	html.WriteString(fmt.Sprintf(`<span><strong>%s:</strong> %s (A) +1 day per file</span>`,
		t("last edited"),
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
		html.WriteString(fmt.Sprintf(`<th>%s</th>`, t(col)))
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
	html.WriteString(fmt.Sprintf(`<strong>%s:</strong> %d`, t("Total Files"), len(metadataList)))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}
