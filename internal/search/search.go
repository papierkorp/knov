// Package search provides search functionality
package search

import (
	"fmt"
	"os"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/searchStorage"
)

// InitSearch initializes search by indexing all files
func InitSearch() error {
	engineType := configmanager.GetSearchEngine()
	logging.LogInfo("initializing search engine: %s", engineType)

	if engineType == "grep" {
		// Grep doesn't need indexing
		logging.LogInfo("grep search engine initialized (no indexing needed)")
		return nil
	}

	// Default repository search - index all files
	return IndexAllFiles()
}

// IndexAllFiles indexes all files for search
func IndexAllFiles() error {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return fmt.Errorf("failed to get all files: %w", err)
	}

	logging.LogInfo("indexing %d files for search", len(allFiles))

	for _, file := range allFiles {
		fullPath := contentStorage.ToDocsPath(file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogWarning("failed to read file %s for indexing: %v", file.Path, err)
			continue
		}

		// index file using searchStorage
		if err := searchStorage.IndexFile(file.Path, content); err != nil {
			logging.LogWarning("failed to index file %s: %v", file.Path, err)
		}
	}

	logging.LogInfo("finished indexing files")
	return nil
}

// SearchFiles performs text search in indexed files
func SearchFiles(query string, limit int) ([]files.File, error) {
	if query == "" {
		return []files.File{}, nil
	}

	engineType := configmanager.GetSearchEngine()

	// Use grep search if configured
	if engineType == "grep" {
		return searchFilesGrep(query, limit)
	}

	// Default: repository-based search
	return searchFilesRepository(query, limit)
}

// searchFilesRepository performs repository-based search using FTS
func searchFilesRepository(query string, limit int) ([]files.File, error) {
	logging.LogDebug("searching for: %s (limit: %d)", query, limit)

	// use much higher FTS limit to ensure we get all relevant files before deduplication
	// FTS can return multiple matches per file, so we need a higher limit to find all unique files
	ftsLimit := limit * 10 // multiply by 10 to account for multiple matches per file
	if ftsLimit < 100 {
		ftsLimit = 100 // minimum FTS limit to ensure we don't miss files
	}

	// use FTS search from searchStorage for better performance
	searchResults, err := searchStorage.SearchContent(query, ftsLimit)
	if err != nil {
		logging.LogWarning("fts search failed, falling back to manual search: %v", err)
		return searchFilesRepositoryFallback(query, limit)
	}

	// convert searchStorage results to files.File
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	// create a map for quick lookup
	fileMap := make(map[string]files.File)
	for _, file := range allFiles {
		fileMap[file.Path] = file
	}

	// deduplicate results by tracking which paths we've already added
	var results []files.File
	seenPaths := make(map[string]bool)
	for _, searchResult := range searchResults {
		if file, exists := fileMap[searchResult.Path]; exists && !seenPaths[searchResult.Path] {
			results = append(results, file)
			seenPaths[searchResult.Path] = true

			// apply user's requested limit after deduplication
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	logging.LogDebug("found %d results for query: %s", len(results), query)
	return results, nil
}

// searchFilesRepositoryFallback performs manual search when FTS fails
func searchFilesRepositoryFallback(query string, limit int) ([]files.File, error) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []files.File

	for _, file := range allFiles {
		// check if already at limit
		if limit > 0 && len(results) >= limit {
			break
		}

		// get indexed content from searchStorage
		contentData, err := searchStorage.GetIndexedContent(file.Path)
		if err != nil || contentData == nil {
			// try reading file directly if not indexed
			fullPath := contentStorage.ToDocsPath(file.Path)
			contentData, err = os.ReadFile(fullPath)
			if err != nil {
				continue
			}
		}

		content := strings.ToLower(string(contentData))
		if strings.Contains(content, queryLower) {
			results = append(results, file)
		}
	}

	return results, nil
}

// searchFilesGrep performs grep-based search
func searchFilesGrep(query string, limit int) ([]files.File, error) {
	logging.LogDebug("using grep search for: %s (limit: %d)", query, limit)

	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []files.File

	for _, file := range allFiles {
		if limit > 0 && len(results) >= limit {
			break
		}

		fullPath := contentStorage.ToDocsPath(file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		contentLower := strings.ToLower(string(content))
		if strings.Contains(contentLower, queryLower) {
			results = append(results, file)
		}
	}

	logging.LogDebug("found %d results for query: %s", len(results), query)
	return results, nil
}
