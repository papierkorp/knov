// Package search provides search functionality
package search

import (
	"fmt"
	"os"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/searchStorage"
)

// InitSearch initializes search by indexing all files
func InitSearch() error {
	engineType := configmanager.GetSearchEngine()
	logging.LogInfo(logging.KeyApp, "initializing search engine: %s", engineType)

	if engineType == "grep" {
		logging.LogInfo(logging.KeyApp, "grep search engine initialized (no indexing needed)")
		return nil
	}

	return IndexAllFiles()
}

// IndexAllFiles indexes all files, skipping the (expensive) FTS reindex for
// files already indexed and unchanged. The trigram fallback index is always
// fully rebuilt from the current file list and swapped in at the end - since
// deleted files are naturally absent from GetAllPhysicalFiles, this is also
// how deleted files stop showing up in trigram search results, without any
// per-delete cleanup. That means a deleted file can still surface via the
// trigram fallback until the next time this runs (the periodic search-reindex
// job) - acceptable since it's a last-resort fallback only consulted when FTS
// finds zero hits, not the primary search path.
func IndexAllFiles() error {
	allFiles, err := files.GetAllPhysicalFiles()
	if err != nil {
		return fmt.Errorf("failed to get all files: %w", err)
	}

	logging.LogInfo(logging.KeySearchReindex, "checking %d files for search indexing", len(allFiles))

	newTrigram := newTrigramIndex()
	indexed, skipped := 0, 0
	for _, file := range allFiles {
		fullPath := pathutils.ToDocsPath(file.Path)

		info, err := os.Stat(fullPath)
		if err != nil {
			logging.LogWarning(logging.KeySearchReindex, "failed to stat file %s for indexing: %v", file.Path, err)
			continue
		}

		// skip the FTS reindex if already indexed and unchanged, but still need
		// its content to rebuild the trigram index below
		if indexedAt, err := searchStorage.GetIndexedAt(file.Path); err == nil && !indexedAt.IsZero() && !info.ModTime().After(indexedAt) {
			content, err := searchStorage.GetIndexedContent(file.Path)
			if err != nil || content == nil {
				logging.LogWarning(logging.KeySearchReindex, "failed to get indexed content for trigram rebuild of %s: %v", file.Path, err)
				continue
			}
			newTrigram.add(file.Path, content)
			skipped++
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogWarning(logging.KeySearchReindex, "failed to read file %s for indexing: %v", file.Path, err)
			continue
		}

		if err := searchStorage.IndexFile(file.Path, content); err != nil {
			logging.LogWarning(logging.KeySearchReindex, "failed to index file %s: %v", file.Path, err)
			continue
		}

		newTrigram.add(file.Path, content)
		indexed++
	}
	replaceTrigramIndex(newTrigram)

	logging.LogInfo(logging.KeySearchReindex, "search indexing complete: %d indexed, %d skipped (up to date)", indexed, skipped)
	return nil
}

// SearchFilesByTitle searches only file titles/names, ignoring content.
// Separate entry point — loads its own file list.
func SearchFilesByTitle(query string, limit int) ([]files.File, error) {
	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []files.File
	for _, file := range allFiles {
		if strings.Contains(strings.ToLower(file.Name), queryLower) {
			results = append(results, file)
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// SearchFiles performs full text + filename + tag search
func SearchFiles(query string, limit int) ([]files.File, error) {
	if query == "" {
		return []files.File{}, nil
	}

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	var results []files.File
	if configmanager.GetSearchEngine() == "grep" {
		results, err = searchFilesGrep(query, limit, allFiles)
	} else {
		results, err = searchFilesRepository(query, limit, allFiles)
	}
	if err != nil {
		return nil, err
	}

	seenPaths := make(map[string]bool, len(results))
	for _, f := range results {
		seenPaths[f.Path] = true
	}

	queryLower := strings.ToLower(query)
	for _, f := range allFiles {
		if seenPaths[f.Path] {
			continue
		}
		// filename match
		if strings.Contains(strings.ToLower(f.Name), queryLower) {
			results = append(results, f)
			seenPaths[f.Path] = true
			continue
		}
		// tag match
		if f.Metadata != nil {
			for _, tag := range f.Metadata.Tags {
				if strings.Contains(strings.ToLower(tag), queryLower) {
					results = append(results, f)
					seenPaths[f.Path] = true
					break
				}
			}
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func searchFilesRepository(query string, limit int, allFiles []files.File) ([]files.File, error) {
	logging.LogDebug(logging.KeyApp, "searching for: %s (limit: %d)", query, limit)

	// use much higher FTS limit to ensure we get all relevant files before deduplication
	// FTS can return multiple matches per file, so we need a higher limit to find all unique files
	ftsLimit := limit * 10 // multiply by 10 to account for multiple matches per file
	if ftsLimit < 100 {
		ftsLimit = 100 // minimum FTS limit to ensure we don't miss files
	}

	searchResults, err := searchStorage.SearchContent(query, ftsLimit)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "fts search failed, falling back to manual search: %v", err)
		return searchFilesRepositoryFallback(query, limit, allFiles)
	}

	fileMap := make(map[string]files.File, len(allFiles))
	for _, f := range allFiles {
		fileMap[f.Path] = f
	}

	var results []files.File
	seenPaths := make(map[string]bool)
	for _, sr := range searchResults {
		if f, ok := fileMap[sr.Path]; ok && !seenPaths[sr.Path] {
			results = append(results, f)
			seenPaths[sr.Path] = true
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	if len(results) == 0 {
		logging.LogDebug(logging.KeyApp, "fts returned no results for '%s', trying trigram fallback", query)
		return searchFilesTrigram(query, limit, allFiles)
	}

	logging.LogDebug(logging.KeyApp, "found %d results for query: %s", len(results), query)
	return results, nil
}

func searchFilesRepositoryFallback(query string, limit int, allFiles []files.File) ([]files.File, error) {
	queryLower := strings.ToLower(query)
	var results []files.File

	for _, file := range allFiles {
		if limit > 0 && len(results) >= limit {
			break
		}

		contentData, err := searchStorage.GetIndexedContent(file.Path)
		if err != nil || contentData == nil {
			var fullPath string
			if pathutils.IsMedia(file.Path) {
				fullPath = pathutils.ToMediaPath(file.Path)
			} else {
				fullPath = pathutils.ToDocsPath(file.Path)
			}
			contentData, err = os.ReadFile(fullPath)
			if err != nil {
				continue
			}
		}

		if strings.Contains(strings.ToLower(string(contentData)), queryLower) {
			results = append(results, file)
		}
	}

	return results, nil
}

func searchFilesGrep(query string, limit int, allFiles []files.File) ([]files.File, error) {
	logging.LogDebug(logging.KeyApp, "using grep search for: %s (limit: %d)", query, limit)

	queryLower := strings.ToLower(query)
	var results []files.File

	for _, file := range allFiles {
		if limit > 0 && len(results) >= limit {
			break
		}

		var fullPath string
		if pathutils.IsMedia(file.Path) {
			fullPath = pathutils.ToMediaPath(file.Path)
		} else {
			fullPath = pathutils.ToDocsPath(file.Path)
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		if strings.Contains(strings.ToLower(string(content)), queryLower) {
			results = append(results, file)
		}
	}

	logging.LogDebug(logging.KeyApp, "found %d results for query: %s", len(results), query)
	return results, nil
}

// SearchDeletedFilesByTitle searches git history for deleted files matching the query by filename.
func SearchDeletedFilesByTitle(query string, limit int) ([]git.GitHistoryFile, error) {
	return git.SearchGitByTitle(query, limit, true)
}

// SearchDeletedFilesByContent searches git history for deleted files whose content matched the query.
func SearchDeletedFilesByContent(query string, limit int) ([]git.GitHistoryFile, error) {
	return git.SearchDeletedFilesByContent(query, limit)
}
