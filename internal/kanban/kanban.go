// Package kanban provides business logic for the kanban board feature.
package kanban

import (
	"os"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// Card holds the data for a single kanban card.
type Card struct {
	FilePath   string
	Title      string
	Collection string
	Status     string
	Tags       []string
	CreatedAt  string
	LastEdited string
}

// Column holds a status column and its ordered cards.
type Column struct {
	Status string
	Cards  []Card
}

// SortBy defines how cards within each column are ordered.
type SortBy string

const (
	SortCreatedAt    SortBy = "createdAt"   // oldest first
	SortEditedAt     SortBy = "editedAt"    // most recently edited first
	SortAlphabetical SortBy = "alphabetical" // by title A→Z
	SortSize         SortBy = "size"        // smallest first
)

// BuildBoard runs the filter, applies optional search, sorts by sortBy, and returns columns with cards.
func BuildBoard(collection string, cfg *filter.Config, searchQuery string, sortBy SortBy) ([]Column, error) {
	columns := configmanager.GetKanbanColumns()
	prefix := configmanager.GetKanbanPrefix()

	result, err := filter.FilterFilesWithConfig(cfg)
	if err != nil {
		logging.LogError("kanban: filter failed for collection %s: %v", collection, err)
		result = &filter.Result{}
	}

	cardsByStatus := make(map[string][]Card, len(columns))
	for _, col := range columns {
		cardsByStatus[col] = []Card{}
	}

	lq := strings.ToLower(searchQuery)
	for _, file := range result.Files {
		if file.Metadata == nil {
			continue
		}
		meta := file.Metadata

		status := StatusFromTags(meta.Tags, prefix)
		if status == "" || !slices.Contains(columns, status) {
			continue
		}

		if lq != "" {
			title := strings.ToLower(meta.Title)
			fp := strings.ToLower(file.Path)
			if !strings.Contains(title, lq) && !strings.Contains(fp, lq) {
				continue
			}
		}

		cardsByStatus[status] = append(cardsByStatus[status], Card{
			FilePath:   pathutils.ToRelative(file.Path),
			Title:      meta.Title,
			Collection: meta.Collection,
			Status:     status,
			Tags:       meta.Tags,
			CreatedAt:  meta.CreatedAt.Format("2006-01-02"),
			LastEdited: meta.LastEdited.Format("2006-01-02"),
		})
	}

	// precompute file sizes once if needed
	var fileSizes map[string]int64
	if sortBy == SortSize {
		fileSizes = make(map[string]int64, len(result.Files))
		for col := range cardsByStatus {
			for _, c := range cardsByStatus[col] {
				if fi, err := os.Stat(pathutils.ToDocsPath(c.FilePath)); err == nil {
					fileSizes[c.FilePath] = fi.Size()
				}
			}
		}
	}

	// only load stored order for custom sort
	var storedOrder Order
	if sortBy == "" {
		storedOrder, _ = GetOrder(collection)
	}

	for col, cards := range cardsByStatus {
		switch sortBy {
		case "":
			// baseline: createdAt, then apply drag-drop order on top
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].CreatedAt < cards[j].CreatedAt
			})
			if storedPaths, ok := storedOrder[col]; ok {
				paths := make([]string, len(cards))
				for i, c := range cards {
					paths[i] = c.FilePath
				}
				ordered := ApplyOrder(storedPaths, paths)
				posMap := make(map[string]int, len(ordered))
				for i, fp := range ordered {
					posMap[fp] = i
				}
				sort.SliceStable(cards, func(i, j int) bool {
					return posMap[cards[i].FilePath] < posMap[cards[j].FilePath]
				})
			}
		case SortCreatedAt:
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].CreatedAt < cards[j].CreatedAt
			})
		case SortEditedAt:
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].LastEdited > cards[j].LastEdited // most recent first
			})
		case SortAlphabetical:
			sort.Slice(cards, func(i, j int) bool {
				return strings.ToLower(cards[i].Title) < strings.ToLower(cards[j].Title)
			})
		case SortSize:
			sort.Slice(cards, func(i, j int) bool {
				return fileSizes[cards[i].FilePath] < fileSizes[cards[j].FilePath]
			})
		}
		cardsByStatus[col] = cards
	}

	cols := make([]Column, 0, len(columns))
	for _, col := range columns {
		cols = append(cols, Column{Status: col, Cards: cardsByStatus[col]})
	}
	return cols, nil
}

// MoveCard updates the kanban status tag on a file and returns the previous status (empty if none).
func MoveCard(filePath, newStatus string) (oldStatus string, err error) {
	normalizedPath := pathutils.ToWithPrefix(filePath)
	meta, err := files.MetaDataGet(normalizedPath)
	if err != nil || meta == nil {
		return "", err
	}

	oldStatus = StatusFromTags(meta.Tags, configmanager.GetKanbanPrefix())

	newTag := configmanager.KanbanStatusTag(newStatus)
	filtered := meta.Tags[:0:0]
	for _, t := range meta.Tags {
		if !configmanager.IsKanbanTag(t) {
			filtered = append(filtered, t)
		}
	}
	meta.Tags = append(filtered, newTag)

	if err := files.MetaDataSaveRaw(meta); err != nil {
		return "", err
	}
	logging.LogInfo("kanban: moved card %s to status %s", filePath, newStatus)
	return oldStatus, nil
}

// TagsForCollection returns all unique non-kanban tags present on kanban cards in the collection.
func TagsForCollection(collection string) ([]string, error) {
	prefix := configmanager.GetKanbanPrefix()
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	tagSet := make(map[string]struct{})
	for _, file := range allFiles {
		if file.Metadata == nil || file.Metadata.Collection != collection {
			continue
		}
		if StatusFromTags(file.Metadata.Tags, prefix) == "" {
			continue
		}
		for _, t := range file.Metadata.Tags {
			if !configmanager.IsKanbanTag(t) {
				tagSet[t] = struct{}{}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	slices.Sort(tags)
	return tags, nil
}

// Excerpt returns the first maxRunes runes of meaningful body text from a file,
// stripping front matter and common markdown syntax.
func Excerpt(fullPath string, maxRunes int) string {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	body := parser.StripFrontMatter(data)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "```") {
			continue
		}
		line = strings.NewReplacer("**", "", "__", "", "*", "", "_", "").Replace(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= maxRunes {
			return line
		}
		return string([]rune(line)[:maxRunes]) + "…"
	}
	return ""
}

// StatusFromTags extracts the kanban status value from a tag list; returns "" if absent.
func StatusFromTags(tags []string, prefix string) string {
	statusPrefix := prefix + "-status-"
	for _, t := range tags {
		if strings.HasPrefix(t, statusPrefix) {
			return strings.TrimPrefix(t, statusPrefix)
		}
	}
	return ""
}

// TagFromList returns the first kanban tag found in the list, or "".
func TagFromList(tags []string) string {
	prefix := configmanager.GetKanbanPrefix() + "-"
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) {
			return t
		}
	}
	return ""
}

// TagNotifyMsg returns a human-readable message for a kanban tag change, or "" when unchanged.
func TagNotifyMsg(oldTag, newTag string) string {
	switch {
	case oldTag == "" && newTag != "":
		return "kanban tag added: " + newTag
	case oldTag != "" && newTag != "" && oldTag != newTag:
		return "kanban status changed: " + oldTag + " → " + newTag
	default:
		return ""
	}
}
