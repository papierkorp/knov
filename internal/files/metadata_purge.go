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
	virtual, err := GetAllVirtualFiles()
	if err != nil {
		return 0, fmt.Errorf("failed to get virtual files: %w", err)
	}

	valid := make(map[string]struct{}, len(physical)+len(virtual))
	for _, f := range physical {
		valid[pathutils.ToWithPrefix(f.Path)] = struct{}{}
	}
	for _, f := range virtual {
		valid[pathutils.ToWithPrefix(f.Path)] = struct{}{}
	}

	var purged int
	for key := range all {
		if _, ok := valid[key]; !ok {
			if err := metadataStorage.Delete(key); err != nil {
				logging.LogWarning("failed to delete stale metadata for %s: %v", key, err)
				continue
			}
			logging.LogInfo("purged stale metadata: %s", key)
			purged++
		}
	}

	logging.LogInfo("metadata purge complete: removed %d stale entries", purged)
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
			logging.LogWarning("failed to delete duplicate metadata for %s: %v", key, err)
			continue
		}
		logging.LogInfo("purged duplicate metadata: %s", key)
	}

	logging.LogInfo("metadata duplicate purge complete: removed %d duplicate entries", len(duplicates))
	return len(duplicates), nil
}
