package parser

import (
	"fmt"
	"html"
	"regexp"
)

type TOCItem struct {
	Level int
	Text  string
	ID    string
	Class string
	Link  string
}

// GenerateTOC extracts h1-h6 headers from rendered HTML and returns TOC items.
// Every heading the markdown renderer emits carries an id (see SlugHeading), so
// the id is read straight off the tag; a heading without one (hand-written raw
// HTML) is skipped rather than given a slug that would match no anchor.
//
// The regex expects `id` right after the level (`<hN id="...">`), which is the
// format knovNodeRenderer.renderHeading emits - keep the two in sync.
func GenerateTOC(htmlStr string) []TOCItem {
	headerRegex := regexp.MustCompile(`<h([1-6])(?:\s+id="([^"]*)")?[^>]*>(.*?)</h[1-6]>`)
	matches := headerRegex.FindAllStringSubmatch(htmlStr, -1)

	toc := make([]TOCItem, 0, len(matches))

	for _, match := range matches {
		id := match[2]
		if id == "" {
			continue
		}
		level := int(match[1][0] - '0')

		// remove header anchor links before extracting text
		content := match[3]
		content = regexp.MustCompile(`<a\s+href="#[^"]*"\s+class="header-anchor"[^>]*>#</a>`).ReplaceAllString(content, "")
		text := stripHTMLTags(content)

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

func stripHTMLTags(s string) string {
	stripped := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
	return html.UnescapeString(stripped)
}
