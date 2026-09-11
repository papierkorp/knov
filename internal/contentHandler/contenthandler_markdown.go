package contentHandler

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/markdown"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/types"
)

// splitPreHeader separates any leading non-header content from the section content.
// Returns (preHeaderContent, sectionContent). If newContent starts with a header,
// preHeaderContent is empty.
func splitPreHeader(content string) (string, string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return strings.Join(lines[:i], "\n"), strings.Join(lines[i:], "\n")
		}
	}
	// no header found, everything is pre-header
	return content, ""
}

// MarkdownContentHandler implements ContentHandler for markdown files
type MarkdownContentHandler struct{}

// NewMarkdownContentHandler creates a new markdown content handler
func NewMarkdownContentHandler() *MarkdownContentHandler {
	return &MarkdownContentHandler{}
}

// Name returns the handler identifier
func (h *MarkdownContentHandler) Name() string {
	return "markdown"
}

// SupportsSection returns true as markdown supports section operations
func (h *MarkdownContentHandler) SupportsSection() bool {
	return true
}

// SupportsTable returns true as markdown supports table operations
func (h *MarkdownContentHandler) SupportsTable() bool {
	return true
}

// ExtractSection extracts content of a specific section by ID
func (h *MarkdownContentHandler) ExtractSection(filePath, sectionID string, includeSubheaders bool) (string, error) {
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return h.extractSectionFromMarkdown(string(content), sectionID, includeSubheaders)
}

// ExtractSectionFromString extracts a section by ID from content already in memory.
func (h *MarkdownContentHandler) ExtractSectionFromString(content, sectionID string, includeSubheaders bool) (string, error) {
	return h.extractSectionFromMarkdown(content, sectionID, includeSubheaders)
}

// SaveSection saves content to a specific section by ID
func (h *MarkdownContentHandler) SaveSection(filePath, sectionID, sectionContent string) error {
	fullPath := pathutils.ToDocsPath(filePath)
	originalContent, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// use the setting to determine replacement scope
	includeSubheaders := configmanager.GetSectionEditIncludeSubheaders()
	updatedContent, err := h.replaceSectionInMarkdown(string(originalContent), sectionID, sectionContent, includeSubheaders)
	if err != nil {
		return fmt.Errorf("failed to replace section: %w", err)
	}

	if err := contentStorage.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExtractTable extracts table data at specific index, returns headers, rows and per-column alignment
func (h *MarkdownContentHandler) ExtractTable(filePath string, tableIndex int) ([]string, [][]string, []string, error) {
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	tableData, err := h.extractTableFromMarkdown(string(content), tableIndex)
	if err != nil {
		return nil, nil, nil, err
	}

	return tableData.Headers, tableData.Rows, tableData.Aligns, nil
}

// SaveTable saves table data and per-column alignment at specific index
func (h *MarkdownContentHandler) SaveTable(filePath string, tableIndex int, headers []string, rows [][]string, aligns []string) error {
	fullPath := pathutils.ToDocsPath(filePath)
	originalContent, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	updatedContent := h.replaceTableInMarkdown(string(originalContent), headers, rows, aligns, tableIndex)

	if err := contentStorage.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractSectionFromMarkdown extracts content between headers with options for subheader inclusion
func (h *MarkdownContentHandler) extractSectionFromMarkdown(content, sectionID string, includeSubheaders bool) (string, error) {
	logging.LogDebug(logging.KeyApp, "extractSectionFromMarkdown: looking for section '%s', includeSubheaders=%t", sectionID, includeSubheaders)
	lines := strings.Split(content, "\n")
	headings := parser.Headings(lines)

	for i, hd := range headings {
		if hd.ID != sectionID {
			continue
		}
		// with includeSubheaders, keep going until a header of the same or higher level
		// (fewer #); without it, stop at the very next header.
		sectionEnd := len(lines)
		for _, next := range headings[i+1:] {
			if includeSubheaders && next.Level > hd.Level {
				continue
			}
			sectionEnd = next.Line
			break
		}
		logging.LogDebug(logging.KeyApp, "extracted section '%s': lines %d-%d", sectionID, hd.Line, sectionEnd-1)
		return strings.Join(lines[hd.Line:sectionEnd], "\n"), nil
	}

	logging.LogDebug(logging.KeyApp, "section '%s' not found", sectionID)
	return "", fmt.Errorf("section not found: %s", sectionID)
}

// replaceSectionInMarkdown replaces content of a specific section with options for subheader handling
func (h *MarkdownContentHandler) replaceSectionInMarkdown(content, sectionID, newContent string, includeSubheaders bool) (string, error) {
	logging.LogDebug(logging.KeyApp, "replaceSectionInMarkdown: replacing section '%s' with %d bytes of content, includeSubheaders=%t", sectionID, len(newContent), includeSubheaders)
	lines := strings.Split(content, "\n")

	// First pass: find the exact boundaries of the original section in the file.
	// This must be independent of newContent so that headers in newContent cannot
	// affect where the replacement ends (which would cause duplicate headers on
	// repeated saves when the user keeps a subheader in the editor content).
	sectionStart := -1
	sectionEnd := len(lines)

	headings := parser.Headings(lines)
	for i, hd := range headings {
		if hd.ID != sectionID {
			continue
		}
		sectionStart = hd.Line
		for _, next := range headings[i+1:] {
			if includeSubheaders && next.Level > hd.Level {
				continue
			}
			sectionEnd = next.Line
			break
		}
		logging.LogDebug(logging.KeyApp, "section '%s' spans lines %d-%d", sectionID, sectionStart, sectionEnd)
		break
	}

	if sectionStart == -1 {
		return "", fmt.Errorf("section not found: %s", sectionID)
	}

	// Adjust sectionEnd to skip over sub-headers from newContent that were written to the
	// file in a previous save. This happens when the user saves, then edits again and saves
	// from the same editor session (without reloading): the editor content still includes
	// headers added in the first save, which are also now present in the file at sectionEnd.
	// We detect this by collecting all headers from newContent that come after the section
	// header itself, then scanning forward from sectionEnd to skip any lines in the original
	// file that belong to those same sub-header blocks.
	if sectionEnd < len(lines) {
		// Collect the sub-headers from newContent (everything after the section
		// header itself), keyed by their raw trimmed line so the tail scan can
		// match the identical lines. parser.Headings gives the same fence-aware,
		// CommonMark heading definition the first pass uses, so a "#" inside a
		// fenced block is never mistaken for a header on either side.
		newLines := strings.Split(newContent, "\n")
		subHeaders := make(map[string]bool)
		pastSectionHeader := false
		for _, nh := range parser.Headings(newLines) {
			if !pastSectionHeader && nh.ID == sectionID {
				pastSectionHeader = true
			} else if pastSectionHeader {
				subHeaders[strings.TrimSpace(newLines[nh.Line])] = true
			}
		}
		if len(subHeaders) > 0 {
			headingLine := make(map[int]bool, len(headings))
			for _, hd := range headings {
				headingLine[hd.Line] = true
			}
			adjustedEnd := sectionEnd
			inSubSection := false
			for j := sectionEnd; j < len(lines); j++ {
				if headingLine[j] {
					if subHeaders[strings.TrimSpace(lines[j])] {
						adjustedEnd = j + 1
						inSubSection = true
					} else {
						break
					}
				} else if inSubSection {
					adjustedEnd = j + 1
				}
			}
			if adjustedEnd > sectionEnd {
				logging.LogDebug(logging.KeyApp, "adjusted sectionEnd from %d to %d (skipping previously-saved sub-headers from newContent)", sectionEnd, adjustedEnd)
				sectionEnd = adjustedEnd
			}
		}
	}

	// Second pass: splice new content in place of lines[sectionStart:sectionEnd].
	var result []string
	result = append(result, lines[:sectionStart]...)
	pre, sec := splitPreHeader(newContent)
	if pre != "" {
		result = append(result, strings.Split(pre, "\n")...)
	}
	if sec != "" {
		result = append(result, strings.Split(sec, "\n")...)
	}
	result = append(result, lines[sectionEnd:]...)

	logging.LogDebug(logging.KeyApp, "replaceSectionInMarkdown complete: original %d lines, result %d lines", len(lines), len(result))
	return strings.Join(result, "\n"), nil
}

// extractTableFromMarkdown extracts table data at specific index
func (h *MarkdownContentHandler) extractTableFromMarkdown(content string, tableIndex int) (*types.SimpleTableData, error) {
	lines := strings.Split(content, "\n")
	var currentTable int = -1
	var inTable bool
	var tableLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect table start (header separator line)
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "-") && !inTable {
			inTable = true
			currentTable++
			// add the header line (previous line if it exists in tableLines)
			if len(tableLines) > 0 && currentTable == tableIndex {
				tableLines = tableLines[len(tableLines)-1:]
			} else {
				tableLines = []string{}
			}
			if currentTable == tableIndex {
				tableLines = append(tableLines, line)
			}
			continue
		}

		if inTable {
			if strings.HasPrefix(trimmed, "|") {
				if currentTable == tableIndex {
					tableLines = append(tableLines, line)
				}
			} else {
				// table ended
				if currentTable == tableIndex {
					break
				}
				inTable = false
				tableLines = []string{}
			}
		} else if strings.HasPrefix(trimmed, "|") && currentTable+1 == tableIndex {
			// potential table header for target table
			tableLines = []string{line}
		}
	}

	if currentTable < tableIndex {
		return nil, fmt.Errorf("table not found at index %d", tableIndex)
	}

	if len(tableLines) < 2 {
		return nil, fmt.Errorf("invalid table structure at index %d", tableIndex)
	}

	tableData := h.parseMarkdownTable(tableLines)
	tableData.TableIndex = tableIndex
	return tableData, nil
}

// parseMarkdownTable parses markdown table lines into types.SimpleTableData
func (h *MarkdownContentHandler) parseMarkdownTable(lines []string) *types.SimpleTableData {
	if len(lines) < 2 {
		return nil
	}

	// parse header
	headerLine := strings.Trim(lines[0], " ")
	headers := h.parseTableRow(headerLine)

	aligns := h.parseAlignRow(lines[1], len(headers))

	var rows [][]string
	for i := 2; i < len(lines); i++ {
		row := h.parseTableRow(lines[i])
		if len(row) > 0 {
			// pad or trim to match header length
			for len(row) < len(headers) {
				row = append(row, "")
			}
			if len(row) > len(headers) {
				row = row[:len(headers)]
			}
			rows = append(rows, row)
		}
	}

	return &types.SimpleTableData{
		Headers: headers,
		Rows:    rows,
		Aligns:  aligns,
		Total:   len(rows),
	}
}

// parseAlignRow parses the markdown header separator row (e.g. "|:---|:---:|---:|")
// into per-column alignment ("left", "center" or "right"), padded/trimmed to colCount.
func (h *MarkdownContentHandler) parseAlignRow(line string, colCount int) []string {
	cells := h.parseTableRow(line)
	aligns := make([]string, colCount)
	for i := range aligns {
		aligns[i] = "left"
		if i >= len(cells) {
			continue
		}
		cell := cells[i]
		left := strings.HasPrefix(cell, ":")
		right := strings.HasSuffix(cell, ":")
		switch {
		case left && right:
			aligns[i] = "center"
		case right:
			aligns[i] = "right"
		}
	}
	return aligns
}

// parseTableRow parses a single markdown table row. Trims exactly one
// leading/trailing "|" rather than a whole run of " |" chars - an empty
// edge cell (e.g. "|   | b | c |") would otherwise be swallowed into the
// same trim as its delimiter, silently dropping the column.
func (h *MarkdownContentHandler) parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	// split by pipe
	cells := strings.Split(line, "|")

	var result []string
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}

	return result
}

// replaceTableInMarkdown replaces a table in markdown content
func (h *MarkdownContentHandler) replaceTableInMarkdown(content string, headers []string, rows [][]string, aligns []string, tableIndex int) string {
	logging.LogDebug(logging.KeyApp, "replaceTableInMarkdown: looking for table %d, headers=%v, rows count=%d", tableIndex, headers, len(rows))

	lines := strings.Split(content, "\n")
	var result []string
	var inAnyTable bool
	var inTable bool
	var currentTable int = -1
	var tableStartIdx int
	var tableEndIdx int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect table header (starts with | but is not separator line)
		if strings.HasPrefix(trimmed, "|") && !inAnyTable {
			// check if next line exists and is separator
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(nextLine, "|") && strings.Contains(nextLine, "-") {
					// this is a table header
					currentTable++
					inAnyTable = true
					logging.LogDebug(logging.KeyApp, "found table %d at line %d: %s", currentTable, i, trimmed)
					if currentTable == tableIndex {
						inTable = true
						tableStartIdx = i
						logging.LogDebug(logging.KeyApp, "starting replacement of table %d at line %d", tableIndex, i)
						continue
					}
				}
			}
		}

		if inAnyTable {
			if strings.HasPrefix(trimmed, "|") {
				if inTable {
					// still in target table, continue skipping
					logging.LogDebug(logging.KeyApp, "skipping table line %d: %s", i, trimmed)
					continue
				}
				// row of a different table, keep as-is
				result = append(result, line)
				continue
			}

			// table ended
			inAnyTable = false
			if inTable {
				tableEndIdx = i
				logging.LogDebug(logging.KeyApp, "table %d ended at line %d, generating replacement", tableIndex, i)
				newTable := h.generateMarkdownTable(headers, rows, aligns)

				logging.LogDebug(logging.KeyApp, "replacing table from line %d to %d with %d new lines", tableStartIdx, tableEndIdx, len(newTable))
				// replace the old table with new table
				result = append(result[:tableStartIdx], newTable...)
				inTable = false
			}
		}

		result = append(result, line)
	}

	// handle case where table is at end of file
	if inTable {
		logging.LogDebug(logging.KeyApp, "table %d at end of file, generating replacement", tableIndex)
		newTable := h.generateMarkdownTable(headers, rows, aligns)
		result = append(result[:tableStartIdx], newTable...)
	}

	logging.LogDebug(logging.KeyApp, "replaceTableInMarkdown complete: original %d lines, result %d lines", len(lines), len(result))
	return strings.Join(result, "\n")
}

// generateMarkdownTable creates markdown table from data
func (h *MarkdownContentHandler) generateMarkdownTable(headers []string, rows [][]string, aligns []string) []string {
	var lines []string

	logging.LogDebug(logging.KeyApp, "generateMarkdownTable: headers=%v, rows count=%d", headers, len(rows))

	// header row
	headerRow := "| " + strings.Join(headers, " | ") + " |"
	lines = append(lines, headerRow)

	// separator row, reflecting per-column alignment where given
	separators := make([]string, len(headers))
	for i := range separators {
		switch {
		case i < len(aligns) && aligns[i] == "center":
			separators[i] = ":---:"
		case i < len(aligns) && aligns[i] == "right":
			separators[i] = "---:"
		default:
			separators[i] = "---"
		}
	}
	sepRow := "| " + strings.Join(separators, " | ") + " |"
	lines = append(lines, sepRow)

	// data rows
	for i, row := range rows {
		logging.LogDebug(logging.KeyApp, "processing row %d: %v", i, row)

		// ensure row matches header length
		for len(row) < len(headers) {
			row = append(row, "")
		}
		if len(row) > len(headers) {
			row = row[:len(headers)]
		}

		// skip completely empty rows (all cells empty or nil)
		allEmpty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}

		// only skip if ALL cells are truly empty - be more lenient
		if allEmpty {
			logging.LogDebug(logging.KeyApp, "skipping empty row %d: %v", i, row)
			continue
		}

		dataRow := "| " + strings.Join(row, " | ") + " |"
		lines = append(lines, dataRow)
		logging.LogDebug(logging.KeyApp, "added row %d: %s", i, dataRow)
	}

	logging.LogDebug(logging.KeyApp, "generated table with %d lines (including header and separator)", len(lines))
	return lines
}

// FindMarkdownTableAnchor returns the id of the header that precedes the Nth
// table (0-based) in the markdown file - the same id the renderer assigns, so a
// wiki link can jump straight to it. Returns "" if none is found.
func FindMarkdownTableAnchor(filePath string, tableIndex int) string {
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		logging.LogDebug(logging.KeyApp, "findMarkdownTableAnchor: could not read %s: %v", filePath, err)
		return ""
	}

	lines := strings.Split(string(content), "\n")
	mask := markdown.FenceMask(lines)

	headerID := make(map[int]string)
	for _, hd := range parser.Headings(lines) {
		headerID[hd.Line] = hd.ID
	}

	lastAnchor := ""
	tableCount := -1
	for i, line := range lines {
		if id, ok := headerID[i]; ok {
			lastAnchor = id
		}
		if mask[i] {
			continue
		}

		// a table separator line (| --- |) following a row line marks a table
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "-") {
			if i > 0 && strings.HasPrefix(strings.TrimSpace(lines[i-1]), "|") {
				tableCount++
				if tableCount == tableIndex {
					return lastAnchor
				}
			}
		}
	}
	return lastAnchor
}
