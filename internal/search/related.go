// Package search - co-occurrence based related file discovery
package search

import (
	"knov/internal/files"
	"knov/internal/logging"
)

// GetRelatedFiles returns pre-computed related files stored in metadata during rebuild.
func GetRelatedFiles(filePath string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}

	meta, err := files.MetaDataGet(filePath)
	if err != nil || meta == nil {
		return nil, nil
	}

	result := meta.Related
	if len(result) > limit {
		result = result[:limit]
	}

	logging.LogDebug("found %d related files for %s", len(result), filePath)
	return result, nil
}
