package contentHandler

import (
	"fmt"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/types"
	"knov/internal/utils"
)

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
func (h *MarkdownContentHandler) ExtractSection(filePath, sectionID string) (string, error) {
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return h.extractSectionFromMarkdown(string(content), sectionID)
}

// SaveSection saves content to a specific section by ID
func (h *MarkdownContentHandler) SaveSection(filePath, sectionID, sectionContent string) error {
	fullPath := pathutils.ToDocsPath(filePath)
	originalContent, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	updatedContent, err := h.replaceSectionInMarkdown(string(originalContent), sectionID, sectionContent)
	if err != nil {
		return fmt.Errorf("failed to replace section: %w", err)
	}

	if err := contentStorage.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExtractTable extracts table data at specific index, returns headers and rows
func (h *MarkdownContentHandler) ExtractTable(filePath string, tableIndex int) ([]string, [][]string, error) {
	fullPath := pathutils.ToDocsPath(filePath)
	content, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	tableData, err := h.extractTableFromMarkdown(string(content), tableIndex)
	if err != nil {
		return nil, nil, err
	}

	return tableData.Headers, tableData.Rows, nil
}

// SaveTable saves table data at specific index
func (h *MarkdownContentHandler) SaveTable(filePath string, tableIndex int, headers []string, rows [][]string) error {
	fullPath := pathutils.ToDocsPath(filePath)
	originalContent, err := contentStorage.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	updatedContent := h.replaceTableInMarkdown(string(originalContent), headers, rows, tableIndex)

	if err := contentStorage.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractSectionFromMarkdown extracts content between headers including the header itself
func (h *MarkdownContentHandler) extractSectionFromMarkdown(content, sectionID string) (string, error) {
	logging.LogDebug("extractSectionFromMarkdown: looking for section '%s'", sectionID)
	lines := strings.Split(content, "\n")

	var sectionStart, sectionEnd int
	var inSection bool
	var inCodeBlock bool
	var sectionHeaderLevel int
	usedIDs := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			logging.LogDebug("found header at line %d: level=%d, id='%s', text='%s'", i, level, headerID, headerText)

			if headerID == sectionID && !inSection {
				sectionStart = i
				sectionHeaderLevel = level
				inSection = true
				logging.LogDebug("found target section '%s' at line %d (level %d)", sectionID, i, level)
				continue
			}

			// stop at header of same or higher level (fewer # = higher level)
			if inSection && level <= sectionHeaderLevel && headerID != sectionID {
				sectionEnd = i
				logging.LogDebug("section ended at line %d due to header level %d (section level %d)", i, level, sectionHeaderLevel)
				break
			}
		}
	}

	if !inSection {
		logging.LogDebug("section '%s' not found", sectionID)
		return "", fmt.Errorf("section not found: %s", sectionID)
	}

	if sectionEnd == 0 {
		sectionEnd = len(lines)
		logging.LogDebug("section extends to end of file (line %d)", sectionEnd)
	}

	sectionLines := lines[sectionStart:sectionEnd]
	logging.LogDebug("extracted section '%s': lines %d-%d (%d lines total)", sectionID, sectionStart, sectionEnd-1, len(sectionLines))
	return strings.Join(sectionLines, "\n"), nil
}

// replaceSectionInMarkdown replaces content of a specific section including the header
func (h *MarkdownContentHandler) replaceSectionInMarkdown(content, sectionID, newContent string) (string, error) {
	logging.LogDebug("replaceSectionInMarkdown: replacing section '%s' with %d bytes of content", sectionID, len(newContent))
	lines := strings.Split(content, "\n")

	var result []string
	var inTargetSection bool
	var inCodeBlock bool
	var headerLevel int
	usedIDs := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inTargetSection {
				// start of target section - replace with new content
				inTargetSection = true
				headerLevel = level
				logging.LogDebug("found target section '%s' at line %d (level %d), replacing content", sectionID, i, level)
				if strings.TrimSpace(newContent) != "" {
					result = append(result, strings.Split(newContent, "\n")...)
				}
				continue
			} else if inTargetSection && level <= headerLevel {
				// reached next section of same or higher level
				inTargetSection = false
				logging.LogDebug("section replacement ended at line %d due to header level %d (section level %d)", i, level, headerLevel)
				result = append(result, line)
				continue
			}
		}

		if !inTargetSection {
			result = append(result, line)
		}
	}

	logging.LogDebug("replaceSectionInMarkdown complete: original %d lines, result %d lines", len(lines), len(result))
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

	// skip separator line (index 1)
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
		Total:   len(rows),
	}
}

// parseTableRow parses a single markdown table row
func (h *MarkdownContentHandler) parseTableRow(line string) []string {
	// remove leading/trailing pipes and whitespace
	line = strings.Trim(line, " |")

	// split by pipe
	cells := strings.Split(line, "|")

	var result []string
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}

	return result
}

// replaceTableInMarkdown replaces a table in markdown content
func (h *MarkdownContentHandler) replaceTableInMarkdown(content string, headers []string, rows [][]string, tableIndex int) string {
	logging.LogDebug("replaceTableInMarkdown: looking for table %d, headers=%v, rows count=%d", tableIndex, headers, len(rows))

	lines := strings.Split(content, "\n")
	var result []string
	var inTable bool
	var currentTable int = -1
	var tableStartIdx int
	var tableEndIdx int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect table header (starts with | but is not separator line)
		if strings.HasPrefix(trimmed, "|") && !inTable {
			// check if next line exists and is separator
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(nextLine, "|") && strings.Contains(nextLine, "-") {
					// this is a table header
					currentTable++
					logging.LogDebug("found table %d at line %d: %s", currentTable, i, trimmed)
					if currentTable == tableIndex {
						inTable = true
						tableStartIdx = i
						logging.LogDebug("starting replacement of table %d at line %d", tableIndex, i)
						continue
					}
				}
			}
		}

		if inTable {
			if strings.HasPrefix(trimmed, "|") {
				// still in table, continue skipping
				logging.LogDebug("skipping table line %d: %s", i, trimmed)
				continue
			} else {
				// table ended, insert new table
				tableEndIdx = i
				logging.LogDebug("table %d ended at line %d, generating replacement", tableIndex, i)
				newTable := h.generateMarkdownTable(headers, rows)

				logging.LogDebug("replacing table from line %d to %d with %d new lines", tableStartIdx, tableEndIdx, len(newTable))
				// replace the old table with new table
				result = append(result[:tableStartIdx], newTable...)
				result = append(result, line)
				inTable = false
				continue
			}
		}

		result = append(result, line)
	}

	// handle case where table is at end of file
	if inTable {
		logging.LogDebug("table %d at end of file, generating replacement", tableIndex)
		newTable := h.generateMarkdownTable(headers, rows)
		result = append(result[:tableStartIdx], newTable...)
	}

	logging.LogDebug("replaceTableInMarkdown complete: original %d lines, result %d lines", len(lines), len(result))
	return strings.Join(result, "\n")
}

// generateMarkdownTable creates markdown table from data
func (h *MarkdownContentHandler) generateMarkdownTable(headers []string, rows [][]string) []string {
	var lines []string

	logging.LogDebug("generateMarkdownTable: headers=%v, rows count=%d", headers, len(rows))

	// header row
	headerRow := "| " + strings.Join(headers, " | ") + " |"
	lines = append(lines, headerRow)

	// separator row
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	sepRow := "| " + strings.Join(separators, " | ") + " |"
	lines = append(lines, sepRow)

	// data rows
	for i, row := range rows {
		logging.LogDebug("processing row %d: %v", i, row)

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
			logging.LogDebug("skipping empty row %d: %v", i, row)
			continue
		}

		dataRow := "| " + strings.Join(row, " | ") + " |"
		lines = append(lines, dataRow)
		logging.LogDebug("added row %d: %s", i, dataRow)
	}

	logging.LogDebug("generated table with %d lines (including header and separator)", len(lines))
	return lines
}
