// Package files handles file operations and metadata
package files

import (
	"fmt"

	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/pathutils"
)

// MetaDataPurgeStale removes metadata entries for files that no longer exist.
// Returns the number of removed entries.
func MetaDataPurgeStale() (int, error) {
	all, err := metadataStorage.GetAll()
	if err != nil {
		return 0, fmt.Errorf("failed to list metadata: %w", err)
	}

	physical, err := GetAllPhysicalFiles()
	if err != nil {
		return 0, fmt.Errorf("failed to get physical files: %w", err)
	}

	valid := make(map[string]struct{}, len(physical))
	for _, f := range physical {
		valid[pathutils.ToWithPrefix(f.Path)] = struct{}{}
	}

	// media files have metadata too — don't treat them as stale
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get media files for stale purge, skipping media: %v", err)
	} else {
		for _, f := range mediaFiles {
			valid[pathutils.ToWithPrefix(f.Path)] = struct{}{}
		}
	}

	var purged int
	for key := range all {
		if _, ok := valid[key]; !ok {
			if err := metadataStorage.Delete(key); err != nil {
				logging.LogWarning(logging.KeyApp, "failed to delete stale metadata for %s: %v", key, err)
				continue
			}
			logging.LogInfo(logging.KeyApp, "purged stale metadata: %s", key)
			purged++
		}
	}

	logging.LogInfo(logging.KeyApp, "metadata purge complete: removed %d stale entries", purged)
	return purged, nil
}

// MetaDataPurgeDuplicates removes metadata entries that are duplicates of another
// entry when normalized. Returns the number of removed entries.
func MetaDataPurgeDuplicates() (int, error) {
	all, err := metadataStorage.GetAll()
	if err != nil {
		return 0, fmt.Errorf("failed to list metadata: %w", err)
	}

	canonical := make(map[string]string) // normalizedPath -> canonical key
	var duplicates []string

	for key := range all {
		norm := pathutils.ToWithPrefix(key)
		if existing, ok := canonical[norm]; ok {
			if key == norm {
				duplicates = append(duplicates, existing)
				canonical[norm] = key
			} else {
				duplicates = append(duplicates, key)
			}
		} else {
			canonical[norm] = key
		}
	}

	for _, key := range duplicates {
		if err := metadataStorage.Delete(key); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to delete duplicate metadata for %s: %v", key, err)
			continue
		}
		logging.LogInfo(logging.KeyApp, "purged duplicate metadata: %s", key)
	}

	logging.LogInfo(logging.KeyApp, "metadata duplicate purge complete: removed %d duplicate entries", len(duplicates))
	return len(duplicates), nil
}
