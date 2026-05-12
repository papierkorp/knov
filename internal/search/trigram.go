// Package search - trigram index for fuzzy fallback search
package search

import (
	"sort"
	"strings"
	"sync"

	"knov/internal/files"
	"knov/internal/logging"
)

var trigramIdx = &trigramIndex{
	index: make(map[string]map[string]struct{}),
}

type trigramIndex struct {
	mu    sync.RWMutex
	index map[string]map[string]struct{} // trigram → set of file paths
}

// tg extracts overlapping 3-character trigrams from a string
func tg(s string) []string {
	if len(s) < 3 {
		return []string{s}
	}
	result := make([]string, 0, len(s)-2)
	for i := 0; i <= len(s)-3; i++ {
		result = append(result, s[i:i+3])
	}
	return result
}

// add indexes file content into the trigram index
func (ti *trigramIndex) add(path string, content []byte) {
	words := strings.Fields(strings.ToLower(string(content)))

	ti.mu.Lock()
	defer ti.mu.Unlock()

	// track which trigrams we've already added for this file to avoid duplicates
	seen := make(map[string]struct{})
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}#*_`-")
		if len(word) < 3 {
			continue
		}
		for _, t := range tg(word) {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			if ti.index[t] == nil {
				ti.index[t] = make(map[string]struct{})
			}
			ti.index[t][path] = struct{}{}
		}
	}
}

// search returns file paths ranked by trigram overlap with query
func (ti *trigramIndex) search(query string, limit int) []string {
	words := strings.Fields(strings.ToLower(query))

	queryTrigrams := make(map[string]struct{})
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}#*_`-")
		if len(word) < 3 {
			continue
		}
		for _, t := range tg(word) {
			queryTrigrams[t] = struct{}{}
		}
	}

	if len(queryTrigrams) == 0 {
		return nil
	}

	ti.mu.RLock()
	defer ti.mu.RUnlock()

	scores := make(map[string]int)
	for t := range queryTrigrams {
		for path := range ti.index[t] {
			scores[path]++
		}
	}

	// require at least 50% of query trigrams to match
	threshold := len(queryTrigrams) / 2
	if threshold < 1 {
		threshold = 1
	}

	type scored struct {
		path  string
		score int
	}
	ranked := make([]scored, 0, len(scores))
	for path, score := range scores {
		if score >= threshold {
			ranked = append(ranked, scored{path, score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	result := make([]string, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		result = append(result, r.path)
	}
	return result
}

// searchFilesTrigram is the fallback fuzzy search using the trigram index
func searchFilesTrigram(query string, limit int) ([]files.File, error) {
	paths := trigramIdx.search(query, limit)
	if len(paths) == 0 {
		return nil, nil
	}

	allFiles, err := files.GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]files.File, len(allFiles))
	for _, f := range allFiles {
		fileMap[f.Path] = f
	}

	result := make([]files.File, 0, len(paths))
	for _, path := range paths {
		if f, ok := fileMap[path]; ok {
			result = append(result, f)
		}
	}

	logging.LogDebug("trigram search for '%s' returned %d results", query, len(result))
	return result, nil
}
