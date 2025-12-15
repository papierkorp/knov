package files

import (
	"fmt"
	"regexp"
	"strings"
)

type TOCItem struct {
	Level int
	Text  string
	ID    string
	Class string
	Link  string
}

// GenerateTOC extracts h1-h6 headers from HTML and returns TOC items
func GenerateTOC(html string) []TOCItem {
	headerRegex := regexp.MustCompile(`<h([1-6])(?:\s+id="([^"]*)")?[^>]*>(.*?)</h[1-6]>`)
	matches := headerRegex.FindAllStringSubmatch(html, -1)

	toc := make([]TOCItem, 0, len(matches))
	usedIDs := make(map[string]int)

	for _, match := range matches {
		level := int(match[1][0] - '0')
		existingID := match[2]
		text := stripHTMLTags(match[3])

		id := existingID
		if id == "" {
			id = generateID(text, usedIDs)
		}

		toc = append(toc, TOCItem{
			Level: level,
			Text:  text,
			ID:    id,
			Class: fmt.Sprintf("toc-level-%d", level),
			Link:  "#" + id,
		})
	}

	return toc
}

// InjectHeaderIDs adds IDs to headers that don't have them and adds anchor links
func InjectHeaderIDs(html string) string {
	usedIDs := make(map[string]int)
	headerRegex := regexp.MustCompile(`<h([1-6])(\s+id="[^"]*")?([^>]*)>(.*?)</h([1-6])>`)

	return headerRegex.ReplaceAllStringFunc(html, func(match string) string {
		parts := headerRegex.FindStringSubmatch(match)
		level := parts[1]
		existingID := strings.TrimSpace(strings.Trim(parts[2], `" id=`))
		attrs := parts[3]
		content := parts[4]

		var id string
		if existingID != "" {
			id = existingID
		} else {
			text := stripHTMLTags(content)
			id = generateID(text, usedIDs)
		}

		// add anchor link to header content
		anchorLink := `<a href="#` + id + `" class="header-anchor" aria-hidden="true">#</a>`
		newContent := content + anchorLink

		return "<h" + level + ` id="` + id + `"` + attrs + ">" + newContent + "</h" + level + ">"
	})
}

func generateID(text string, usedIDs map[string]int) string {
	id := strings.ToLower(text)
	id = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")

	if id == "" {
		id = "section"
	}

	originalID := id
	count := usedIDs[originalID]
	if count > 0 {
		id = fmt.Sprintf("%s-%d", id, count)
	}
	usedIDs[originalID]++

	return id
}

func stripHTMLTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}
