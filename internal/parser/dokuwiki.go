// Package parser handles dokuwiki parsing
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseDokuWiki converts DokuWiki syntax to HTML
func ParseDokuWiki(content string) string {
	// process code blocks first (before other formatting)
	content = processDokuWikiCodeBlocks(content)

	// headers
	content = regexp.MustCompile(`======\s*(.+?)\s*======`).ReplaceAllString(content, "<h1>$1</h1>")
	content = regexp.MustCompile(`=====\s*(.+?)\s*=====`).ReplaceAllString(content, "<h2>$1</h2>")
	content = regexp.MustCompile(`====\s*(.+?)\s*====`).ReplaceAllString(content, "<h3>$1</h3>")
	content = regexp.MustCompile(`===\s*(.+?)\s*===`).ReplaceAllString(content, "<h4>$1</h4>")
	content = regexp.MustCompile(`==\s*(.+?)\s*==`).ReplaceAllString(content, "<h5>$1</h5>")

	// bold
	content = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(content, "<strong>$1</strong>")

	// italic
	content = regexp.MustCompile(`//(.+?)//`).ReplaceAllString(content, "<em>$1</em>")

	// underline
	content = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(content, "<u>$1</u>")

	// monospace
	content = regexp.MustCompile(`''(.+?)''`).ReplaceAllString(content, "<code>$1</code>")

	// line breaks
	content = strings.ReplaceAll(content, "\\\\", "<br>")

	// process folded sections
	content = processDokuWikiFolded(content)

	// replace tables with HTMX placeholder
	content = replaceTablesWithHTMX(content)

	// process links using shared function
	content = ProcessDokuWikiLinks(content)

	// paragraphs
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
func processDokuWikiCodeBlocks(content string) string {
	// <code language> ... </code>
	content = regexp.MustCompile(`(?s)<code\s+([a-zA-Z0-9_-]+)>(.*?)</code>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<code\s+([a-zA-Z0-9_-]+)>(.*?)</code>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, language, escapeHTML(code))
	})

	// <code> ... </code> (no language)
	content = regexp.MustCompile(`(?s)<code>(.*?)</code>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<code>(.*?)</code>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		code := strings.TrimSpace(matches[1])
		return fmt.Sprintf(`<pre><code class="language-plaintext">%s</code></pre>`, escapeHTML(code))
	})

	// <sxh language> ... </sxh> (syntaxhighlighter4)
	content = regexp.MustCompile(`(?s)<sxh\s+([a-zA-Z0-9_-]+)>(.*?)</sxh>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<sxh\s+([a-zA-Z0-9_-]+)>(.*?)</sxh>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, language, escapeHTML(code))
	})

	// <codify language> ... </codify>
	content = regexp.MustCompile(`(?s)<codify\s+([a-zA-Z0-9_-]+)>(.*?)</codify>`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(?s)<codify\s+([a-zA-Z0-9_-]+)>(.*?)</codify>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		language := strings.TrimSpace(matches[1])
		code := strings.TrimSpace(matches[2])
		return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, language, escapeHTML(code))
	})

	return content
}

// escapeHTML escapes HTML special characters in code
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// processDokuWikiFolded converts folded plugin syntax to HTML details/summary
func processDokuWikiFolded(content string) string {
	// ++++ Title | Content ++++
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
func replaceTablesWithHTMX(content string) string {
	// detect tables and replace with HTMX loader
	lines := strings.Split(content, "\n")
	var result []string
	var inTable bool

	for _, line := range lines {
		if strings.HasPrefix(line, "^") || strings.HasPrefix(line, "|") {
			if !inTable {
				// start of table - add HTMX placeholder
				result = append(result, `<div id="table-wrapper" hx-get="/api/components/table?filepath={{FILEPATH}}&page=1&size=25" hx-trigger="load" hx-swap="innerHTML">Loading table...</div>`)
				inTable = true
			}
			// skip table lines
		} else {
			if inTable {
				inTable = false
			}
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// ProcessDokuWikiLinks converts DokuWiki-style links to HTML
func ProcessDokuWikiLinks(content string) string {
	// [[link]] or [[link|title]]
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

		// ensure extension
		if !strings.HasSuffix(link, ".md") && !strings.HasSuffix(link, ".txt") {
			link += ".txt"
		}

		return `<a href="/files/` + link + `">` + title + `</a>`
	})

	return content
}

// ExtractDokuWikiLinks extracts all links from dokuwiki content
func ExtractDokuWikiLinks(content string) []string {
	var links []string
	linkSet := make(map[string]bool)

	// [[link]] or [[link|title]] pattern
	re := regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			// clean up path
			if idx := strings.Index(link, "|"); idx != -1 {
				link = link[:idx]
			}
			link = strings.TrimSpace(link)

			if !linkSet[link] {
				linkSet[link] = true
				links = append(links, link)
			}
		}
	}

	return links
}

// ParseDokuWikiTable extracts table data from dokuwiki content
func ParseDokuWikiTable(content string) (*TableData, error) {
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

			cells := splitMixedDelimiters(line)
			isHeaderRow := !headerParsed && isMajorityHeaderCells(line)

			if isHeaderRow {
				for idx, cell := range cells {
					cellContent := strings.TrimSpace(cell)
					align := detectCellAlignment(cell)
					dataType := detectCellType(cellContent)

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
					align := detectCellAlignment(cell)
					dataType := detectCellType(cellContent)

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

func splitMixedDelimiters(line string) []string {
	var cells []string
	var current strings.Builder

	for _, ch := range line {
		if ch == '^' || ch == '|' {
			if current.Len() > 0 || len(cells) > 0 {
				cells = append(cells, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		cells = append(cells, current.String())
	}

	return cells
}

func isMajorityHeaderCells(line string) bool {
	carets := strings.Count(line, "^")
	pipes := strings.Count(line, "|")
	return carets > pipes
}

func detectCellAlignment(cell string) string {
	trimmed := strings.TrimSpace(cell)
	if trimmed == "" {
		return ""
	}

	leftSpaces := len(cell) - len(strings.TrimLeft(cell, " "))
	rightSpaces := len(cell) - len(strings.TrimRight(cell, " "))

	if leftSpaces > 1 && rightSpaces > 1 {
		return "center"
	} else if rightSpaces > 1 {
		return "right"
	}
	return "left"
}

func detectCellType(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "text"
	}

	if matched, _ := regexp.MatchString(`^-?\d+\.?\d*$`, content); matched {
		return "number"
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, content); matched {
		return "date"
	}
	if matched, _ := regexp.MatchString(`^[$€£¥]\s*\d`, content); matched {
		return "currency"
	}
	return "text"
}
