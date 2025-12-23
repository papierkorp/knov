package utils

import (
	"fmt"
	"regexp"
	"strings"

	"knov/internal/logging"
)

// GenerateID generates a unique ID from header text with collision handling
func GenerateID(text string, usedIDs map[string]int) string {
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

	logging.LogDebug("GenerateID: '%s' -> '%s' (count: %d, usedIDs: %v)", text, id, count, usedIDs)

	return id
}
