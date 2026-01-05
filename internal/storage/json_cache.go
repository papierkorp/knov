// Package storage - JSON cache storage implementation
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"knov/internal/logging"
)

// JSONCacheStorage implements CacheStorage using JSON files with in-memory search
type JSONCacheStorage struct {
	*baseJSONStorage
	searchIndex map[string]*searchEntry
	indexMutex  sync.RWMutex
}

type searchEntry struct {
	Content  string
	Tags     string
	Title    string
	Metadata map[string]string
}

// NewJSONCacheStorage creates a new JSON cache storage
func NewJSONCacheStorage(basePath string) (*JSONCacheStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &JSONCacheStorage{
		baseJSONStorage: &baseJSONStorage{
			basePath: basePath,
		},
		searchIndex: make(map[string]*searchEntry),
	}, nil
}

// Search performs in-memory search
func (js *JSONCacheStorage) Search(query string, opts SearchOptions) (*SearchResults, error) {
	js.indexMutex.RLock()
	defer js.indexMutex.RUnlock()

	start := time.Now()
	queryLower := strings.ToLower(query)

	var results []*SearchResult

	for key, entry := range js.searchIndex {
		score := js.calculateScore(entry, queryLower)
		if score > 0 {
			snippet := js.extractSnippet(entry.Content, query, 200)
			results = append(results, &SearchResult{
				Key:      key,
				Score:    score,
				Snippet:  snippet,
				Metadata: entry.Metadata,
			})
		}
	}

	// simple sort by score (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// apply pagination
	if opts.Limit == 0 {
		opts.Limit = 50
	}

	start_idx := opts.Offset
	end_idx := opts.Offset + opts.Limit

	if start_idx >= len(results) {
		results = []*SearchResult{}
	} else {
		if end_idx > len(results) {
			end_idx = len(results)
		}
		results = results[start_idx:end_idx]
	}

	took := time.Since(start)

	logging.LogDebug("search for '%s' returned %d results in %v", query, len(results), took)

	return &SearchResults{
		Results: results,
		Total:   len(results),
		Took:    took,
	}, nil
}

func (js *JSONCacheStorage) calculateScore(entry *searchEntry, query string) float64 {
	score := 0.0

	contentLower := strings.ToLower(entry.Content)
	tagsLower := strings.ToLower(entry.Tags)
	titleLower := strings.ToLower(entry.Title)

	// title matches are worth more
	if strings.Contains(titleLower, query) {
		score += 10.0
	}

	// tag matches
	if strings.Contains(tagsLower, query) {
		score += 5.0
	}

	// content matches
	if strings.Contains(contentLower, query) {
		score += 1.0
	}

	return score
}

func (js *JSONCacheStorage) extractSnippet(content, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	pos := strings.Index(lowerContent, lowerQuery)
	if pos == -1 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := pos - 50
	if start < 0 {
		start = 0
	}

	end := pos + len(query) + 50
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}

// IndexContent adds content to search index
func (js *JSONCacheStorage) IndexContent(key string, content []byte, metadata map[string]string) error {
	js.indexMutex.Lock()
	defer js.indexMutex.Unlock()

	js.searchIndex[key] = &searchEntry{
		Content:  string(content),
		Tags:     metadata["tags"],
		Title:    metadata["title"],
		Metadata: metadata,
	}

	logging.LogDebug("indexed content for key: %s", key)
	return nil
}

// Delete removes from storage and search index
func (js *JSONCacheStorage) Delete(key string) error {
	if err := js.baseJSONStorage.Delete(key); err != nil {
		return err
	}

	js.indexMutex.Lock()
	delete(js.searchIndex, key)
	js.indexMutex.Unlock()

	return nil
}

// Rebuild recreates the search index
func (js *JSONCacheStorage) Rebuild() error {
	js.indexMutex.Lock()
	js.searchIndex = make(map[string]*searchEntry)
	js.indexMutex.Unlock()

	allKeys, err := js.List("")
	if err != nil {
		return err
	}

	for _, key := range allKeys {
		data, err := js.Get(key)
		if err != nil || data == nil {
			continue
		}

		// try to parse as search entry
		var entry map[string]interface{}
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		content := fmt.Sprintf("%v", entry["content"])
		metadata := make(map[string]string)
		if meta, ok := entry["metadata"].(map[string]interface{}); ok {
			for k, v := range meta {
				metadata[k] = fmt.Sprintf("%v", v)
			}
		}

		js.IndexContent(key, []byte(content), metadata)
	}

	logging.LogInfo("search index rebuilt with %d entries", len(allKeys))
	return nil
}

// Invalidate removes cache entries matching pattern
func (js *JSONCacheStorage) Invalidate(pattern string) error {
	allKeys, err := js.List("")
	if err != nil {
		return err
	}

	count := 0
	for _, key := range allKeys {
		if strings.Contains(key, pattern) {
			if err := js.Delete(key); err != nil {
				logging.LogWarning("failed to invalidate key %s: %v", key, err)
			} else {
				count++
			}
		}
	}

	logging.LogDebug("invalidated %d cache entries matching pattern: %s", count, pattern)
	return nil
}
