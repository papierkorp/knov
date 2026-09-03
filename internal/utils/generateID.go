package utils

import (
	"fmt"
	"regexp"
	"strings"

	"knov/internal/logging"
)

// GenerateID generates a unique ID from header text with collision handling.
//
// This is the one slug function for header ids: the markdown renderer/TOC
// (parser.InjectHeaderIDs), the pdf exporter and internal/book all call it.
// internal/book resolves a `.book` "path#section" entry by slugging the anchor
// through here and matching it against the rendered header id, so changing the
// slug rules silently breaks book section references - keep it in sync or route
// both sides through a shared helper.
func GenerateID(text string, usedIDs map[string]int) string {
	id := strings.ToLower(text)
	id = regexp.MustCompile(`[^\p{L}\p{N}]+`).ReplaceAllString(id, "-")
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

	logging.LogDebug(logging.KeyApp, "GenerateID: '%s' -> '%s' (count: %d, usedIDs: %v)", text, id, count, usedIDs)

	return id
}
