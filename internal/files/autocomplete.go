package files

import (
	"path/filepath"
	"sort"
	"strings"
)

// RankAutocompleteMatches filters paths by query and orders them so the most
// relevant matches come first, instead of an arbitrary (e.g. directory-walk)
// order silently truncated at limit:
//  1. filename equals the query
//  2. filename starts with the query
//  3. filename contains the query
//  4. only the containing path contains the query
// Ties within a tier are broken alphabetically. Returns at most limit paths.
func RankAutocompleteMatches(paths []string, query string, limit int) []string {
	query = strings.ToLower(strings.TrimSpace(query))

	type match struct {
		path string
		tier int
	}
	matches := make([]match, 0, len(paths))
	for _, p := range paths {
		base := strings.ToLower(filepath.Base(p))
		switch {
		case query == "":
			matches = append(matches, match{p, 0})
		case base == query:
			matches = append(matches, match{p, 0})
		case strings.HasPrefix(base, query):
			matches = append(matches, match{p, 1})
		case strings.Contains(base, query):
			matches = append(matches, match{p, 2})
		case strings.Contains(strings.ToLower(p), query):
			matches = append(matches, match{p, 3})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].tier != matches[j].tier {
			return matches[i].tier < matches[j].tier
		}
		return matches[i].path < matches[j].path
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	results := make([]string, len(matches))
	for i, m := range matches {
		results[i] = m.path
	}
	return results
}
