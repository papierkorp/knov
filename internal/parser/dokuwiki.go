package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"knov/internal/logging"
)

type DokuwikiHandler struct{}

func NewDokuwikiHandler() *DokuwikiHandler {
	return &DokuwikiHandler{}
}

func (h *DokuwikiHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == ".dokuwiki" {
		return true
	}

	if ext == ".txt" {
		content, err := os.ReadFile(filename)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			if len(lines) > 0 {
				firstLine := strings.TrimSpace(lines[0])
				if strings.HasPrefix(firstLine, "======") || strings.HasPrefix(firstLine, "=====") {
					return true
				}
			}
		}
	}

	return false
}

func (h *DokuwikiHandler) GetContent(filepath string) ([]byte, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filepath, err)
		return nil, err
	}
	return content, nil
}

func (h *DokuwikiHandler) Parse(content []byte) ([]byte, error) {
	parsed := h.parseDokuWiki(string(content))
	return []byte(parsed), nil
}

func (h *DokuwikiHandler) Render(content []byte, filePath string) ([]byte, error) {
	return content, nil
}

func (h *DokuwikiHandler) ExtractLinks(content []byte) []string {
	return h.extractDokuWikiLinks(string(content))
}

func (h *DokuwikiHandler) Name() string {
	return "dokuwiki"
}

// parseDokuWiki converts DokuWiki syntax to HTML
func (h *DokuwikiHandler) parseDokuWiki(content string) string {
	// process headers first, before code blocks to avoid conflicts
	content = regexp.MustCompile(`======\s*(.+?)\s*======`).ReplaceAllString(content, "<h1>$1</h1>")
	content = regexp.MustCompile(`=====\s*(.+?)\s*=====`).ReplaceAllString(content, "<h2>$1</h2>")
	content = regexp.MustCompile(`====\s*(.+?)\s*====`).ReplaceAllString(content, "<h3>$1</h3>")
	content = regexp.MustCompile(`===\s*(.+?)\s*===`).ReplaceAllString(content, "<h4>$1</h4>")
	content = regexp.MustCompile(`==\s*(.+?)\s*==`).ReplaceAllString(content, "<h5>$1</h5>")

	content = h.processDokuWikiCodeBlocks(content)

	content = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(content, "<strong>$1</strong>")
	content = regexp.MustCompile(`//(.+?)//`).ReplaceAllString(content, "<em>$1</em>")
	content = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(content, "<u>$1</u>")
	content = regexp.MustCompile(`''(.+?)''`).ReplaceAllString(content, "<code>$1</code>")

	content = strings.ReplaceAll(content, "\\\\", "<br>")

	content = h.processDokuWikiFolded(content)
	content = h.replaceTablesWithHTMX(content)
	content = h.processDokuWikiLinks(content)
	content = h.processDokuWikiLists(content)

	lines := strings.Split(content, "\n\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "<") {
			lines[i] = "<p>" + line + "</p>"
		}
	}
	content = strings.Join(lines, "\n")

	return content
}

// processDokuWikiCodeBlocks handles all code block syntaxes
func (h *DokuwikiHandler) processDokuWikiCodeBlocks(content string) string {
	// <code language>...</code>
	content = regexp.MustCompile(`(?s)<code\s+([a-zA-Z0-9_-]+)>(.*?)</code>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<code\s+([a-zA-Z0-9_-]+)>(.*?)</code>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return HighlightCodeBlock(code, language)
	})

	// <code>...</code> (no language)
	content = regexp.MustCompile(`(?s)<code>(.*?)</code>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<code>(.*?)</code>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		code := strings.TrimSpace(matches[1])
		return HighlightCodeBlock(code, "text")
	})

	// <sxh language>...</sxh>
	content = regexp.MustCompile(`(?s)<sxh\s+([a-zA-Z0-9_-]+)>(.*?)</sxh>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<sxh\s+([a-zA-Z0-9_-]+)>(.*?)</sxh>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return HighlightCodeBlock(code, language)
	})

	// <codify language>...</codify>
	content = regexp.MustCompile(`(?s)<codify\s+([a-zA-Z0-9_-]+)>(.*?)</codify>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<codify\s+([a-zA-Z0-9_-]+)>(.*?)</codify>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return HighlightCodeBlock(code, language)
	})

	return content
}

// processDokuWikiFolded converts folded plugin syntax to HTML details/summary
func (h *DokuwikiHandler) processDokuWikiFolded(content string) string {
	re := regexp.MustCompile(`(?s)\+{4,}\s*([^|]+?)\s*\|\s*(.*?)\s*\+{4,}`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		title := strings.TrimSpace(matches[1])
		foldedContent := strings.TrimSpace(matches[2])

		return fmt.Sprintf(`<details class="dokuwiki-folded">
<summary>%s</summary>
<div class="folded-content">%s</div>
</details>`, title, foldedContent)
	})

	return content
}

// replaceTablesWithHTMX replaces table syntax with HTMX loading div
func (h *DokuwikiHandler) replaceTablesWithHTMX(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inTable bool

	for _, line := range lines {
		if strings.HasPrefix(line, "^") || strings.HasPrefix(line, "|") {
			if !inTable {
				result = append(result, `<div id="table-wrapper" hx-get="/api/components/table?filepath={{FILEPATH}}&page=1&size=25" hx-trigger="load" hx-swap="innerHTML">Loading table...</div>`)
				inTable = true
			}
		} else {
			if inTable {
				inTable = false
			}
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// processDokuWikiLinks converts DokuWiki-style links to HTML
func (h *DokuwikiHandler) processDokuWikiLinks(content string) string {
	re := regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		link := strings.TrimSpace(matches[1])
		title := link
		if len(matches) > 2 && matches[2] != "" {
			title = strings.TrimSpace(matches[2])
		}

		// check if it's a URL (external link)
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			return `<a href="` + link + `" target="_blank" rel="noopener noreferrer">` + title + `</a>`
		}

		// check for special protocols (apt://, etc.)
		if strings.Contains(link, "://") {
			return `<a href="` + link + `" target="_blank" rel="noopener noreferrer">` + title + `</a>`
		}

		// handle internal dokuwiki links with namespaces (colons) and anchors
		var anchor string
		if strings.Contains(link, "#") {
			parts := strings.Split(link, "#")
			link = parts[0]
			anchor = "#" + parts[1]
		}

		// convert dokuwiki namespace (colons) to filesystem path (slashes)
		link = strings.ReplaceAll(link, ":", "/")

		// add .txt extension if no extension present
		if !strings.HasSuffix(link, ".md") && !strings.HasSuffix(link, ".txt") {
			link += ".txt"
		}

		return `<a href="/files/` + link + anchor + `">` + title + `</a>`
	})

	return content
}

// extractDokuWikiLinks extracts all links from dokuwiki content
func (h *DokuwikiHandler) extractDokuWikiLinks(content string) []string {
	var links []string
	linkSet := make(map[string]bool)

	// remove code blocks to avoid extracting links from code
	content = h.removeDokuWikiCodeBlocks(content)

	re := regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			if idx := strings.Index(link, "|"); idx != -1 {
				link = link[:idx]
			}
			link = strings.TrimSpace(link)

			// skip URLs and special protocols - only extract internal links for metadata
			if strings.Contains(link, "://") {
				continue
			}

			// remove anchor for metadata (but keep the base link)
			if strings.Contains(link, "#") {
				link = strings.Split(link, "#")[0]
			}

			// convert dokuwiki namespace (colons) to filesystem path (slashes)
			link = strings.ReplaceAll(link, ":", "/")

			// add .txt extension for dokuwiki files
			if link != "" && !strings.HasSuffix(link, ".md") && !strings.HasSuffix(link, ".txt") {
				link += ".txt"
			}

			if link != "" && !linkSet[link] {
				linkSet[link] = true
				links = append(links, link)
			}
		}
	}

	return links
}

// removeDokuWikiCodeBlocks removes code blocks from dokuwiki content
func (h *DokuwikiHandler) removeDokuWikiCodeBlocks(content string) string {
	// remove <code>...</code> blocks
	content = regexp.MustCompile(`(?s)<code[^>]*>.*?</code>`).ReplaceAllString(content, "")

	// remove <file>...</file> blocks
	content = regexp.MustCompile(`(?s)<file[^>]*>.*?</file>`).ReplaceAllString(content, "")

	// remove <sxh>...</sxh> blocks
	content = regexp.MustCompile(`(?s)<sxh[^>]*>.*?</sxh>`).ReplaceAllString(content, "")

	// remove <codify>...</codify> blocks
	content = regexp.MustCompile(`(?s)<codify[^>]*>.*?</codify>`).ReplaceAllString(content, "")

	// remove <html>...</html> blocks
	content = regexp.MustCompile(`(?s)<html[^>]*>.*?</html>`).ReplaceAllString(content, "")

	// remove <nowiki>...</nowiki> blocks
	content = regexp.MustCompile(`(?s)<nowiki>.*?</nowiki>`).ReplaceAllString(content, "")

	return content
}

// ParseDokuWikiTable extracts table data from dokuwiki content
func (h *DokuwikiHandler) ParseDokuWikiTable(content string) (*TableData, error) {
	lines := strings.Split(content, "\n")
	var headers []TableHeader
	var rows [][]TableCell
	var inTable bool
	headerParsed := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "^") || strings.HasPrefix(line, "|") {
			inTable = true

			line = strings.TrimPrefix(line, "^")
			line = strings.TrimPrefix(line, "|")
			line = strings.TrimSuffix(line, "^")
			line = strings.TrimSuffix(line, "|")

			cells := h.splitMixedDelimiters(line)
			isHeaderRow := !headerParsed && h.isMajorityHeaderCells(line)

			if isHeaderRow {
				for idx, cell := range cells {
					cellContent := strings.TrimSpace(cell)
					align := h.detectCellAlignment(cell)
					dataType := h.detectCellType(cellContent)

					headers = append(headers, TableHeader{
						Content:   cellContent,
						DataType:  dataType,
						Align:     align,
						Sortable:  true,
						ColumnIdx: idx,
					})
				}
				headerParsed = true
			} else {
				var row []TableCell
				for i, cell := range cells {
					cellContent := strings.TrimSpace(cell)
					align := h.detectCellAlignment(cell)
					dataType := h.detectCellType(cellContent)

					if i < len(headers) {
						if headers[i].Align != "" {
							align = headers[i].Align
						}
						if headers[i].DataType != "" {
							dataType = headers[i].DataType
						}
					}

					row = append(row, TableCell{
						Content:  cellContent,
						DataType: dataType,
						Align:    align,
						RawValue: cellContent,
					})
				}
				rows = append(rows, row)
			}
		} else if inTable {
			break
		}
	}

	return &TableData{
		Headers: headers,
		Rows:    rows,
		Total:   len(rows),
	}, nil
}

func (h *DokuwikiHandler) splitMixedDelimiters(line string) []string {
	var cells []string
	var currentCell strings.Builder

	for _, char := range line {
		if char == '^' || char == '|' {
			if currentCell.Len() > 0 || len(cells) > 0 {
				cells = append(cells, currentCell.String())
				currentCell.Reset()
			}
		} else {
			currentCell.WriteRune(char)
		}
	}

	if currentCell.Len() > 0 {
		cells = append(cells, currentCell.String())
	}

	return cells
}

func (h *DokuwikiHandler) isMajorityHeaderCells(line string) bool {
	headerCount := strings.Count(line, "^")
	normalCount := strings.Count(line, "|")
	return headerCount > normalCount
}

func (h *DokuwikiHandler) detectCellAlignment(cell string) string {
	trimmed := strings.TrimSpace(cell)
	if len(trimmed) == 0 {
		return "left"
	}

	leftSpaces := len(cell) - len(strings.TrimLeft(cell, " "))
	rightSpaces := len(cell) - len(strings.TrimRight(cell, " "))

	if leftSpaces > 0 && rightSpaces > 0 && leftSpaces == rightSpaces {
		return "center"
	}
	if rightSpaces > leftSpaces {
		return "right"
	}
	return "left"
}

func (h *DokuwikiHandler) detectCellType(content string) string {
	content = strings.TrimSpace(content)

	if matched, _ := regexp.MatchString(`^[$ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€šÃ‚Â£Ãƒâ€šÃ‚Â¥]\s*[\d,]+\.?\d*$`, content); matched {
		return "currency"
	}
	if matched, _ := regexp.MatchString(`^\d+\.?\d*$`, content); matched {
		return "number"
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, content); matched {
		return "date"
	}
	if matched, _ := regexp.MatchString(`^\d{2}\.\d{2}\.\d{4}$`, content); matched {
		return "date"
	}

	return "text"
}

// processDokuWikiLists converts DokuWiki list syntax to HTML with support for nested lists
func (h *DokuwikiHandler) processDokuWikiLists(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var listStack []string // tracks open list tags by level

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")

		// detect list item and its level
		var isListItem bool
		var listType string
		var level int
		var item string

		if match := regexp.MustCompile(`^(  +)(\*|-) (.+)$`).FindStringSubmatch(trimmed); match != nil {
			level = len(match[1]) / 2
			if match[2] == "*" {
				listType = "ul"
			} else {
				listType = "ol"
			}
			item = match[3]
			isListItem = true
		}

		if isListItem {
			// close lists deeper than current level
			for len(listStack) > level {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}

			// check if we need to change list type at current level
			if len(listStack) == level && listStack[level-1] != listType {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}

			// open new lists up to current level
			for len(listStack) < level {
				result = append(result, "<"+listType+">")
				listStack = append(listStack, listType)
			}

			result = append(result, "<li>"+item+"</li>")
		} else {
			// close all open lists
			for len(listStack) > 0 {
				result = append(result, "</"+listStack[len(listStack)-1]+">")
				listStack = listStack[:len(listStack)-1]
			}
			result = append(result, line)
		}
	}

	// close any remaining open lists
	for len(listStack) > 0 {
		result = append(result, "</"+listStack[len(listStack)-1]+">")
		listStack = listStack[:len(listStack)-1]
	}

	return strings.Join(result, "\n")
}
