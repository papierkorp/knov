// Package search - co-occurrence based related file discovery
package search

import (
	"sort"

	"knov/internal/files"
	"knov/internal/logging"
)

// GetRelatedFiles returns files related to filePath by shared link co-occurrence.
// Scores candidate files by the number of link neighbors they share with the target.
func GetRelatedFiles(filePath string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}

	meta, err := files.MetaDataGet(filePath)
	if err != nil || meta == nil {
		return nil, nil
	}

	// build neighbor set from both outbound and inbound links
	neighbors := make(map[string]struct{}, len(meta.UsedLinks)+len(meta.LinksToHere))
	for _, l := range meta.UsedLinks {
		neighbors[l] = struct{}{}
	}
	for _, l := range meta.LinksToHere {
		neighbors[l] = struct{}{}
	}

	if len(neighbors) == 0 {
		return nil, nil
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		return nil, err
	}

	scores := make(map[string]int)
	for _, f := range allFiles {
		if f.Path == filePath {
			continue
		}
		other, err := files.MetaDataGet(f.Path)
		if err != nil || other == nil {
			continue
		}
		score := 0
		for _, l := range other.UsedLinks {
			if _, ok := neighbors[l]; ok {
				score++
			}
		}
		for _, l := range other.LinksToHere {
			if _, ok := neighbors[l]; ok {
				score++
			}
		}
		if score > 0 {
			scores[f.Path] = score
		}
	}

	type scored struct {
		path  string
		score int
	}
	ranked := make([]scored, 0, len(scores))
	for path, score := range scores {
		ranked = append(ranked, scored{path, score})
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

	logging.LogDebug("found %d related files for %s", len(result), filePath)
	return result, nil
}
