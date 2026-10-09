// Package test - metadata seeding helpers for suite sampledata setup.
package test

import "knov/internal/files"

// SeedMetadata seeds path's metadata for test setup: recomputes derived fields via
// MetaDataSync, then applies whichever user-owned fields meta sets (mirrors the old
// MetaDataSave partial-update convention - a zero-value field is left untouched, not
// cleared). Single-threaded suite setup has no concurrent writer to race, so this
// composition is safe here even though production code must use MetaDataMutate/MetaDataSync
// directly instead of a caller-assembled *Metadata (see files package godoc).
//
// Refreshes the aggregate caches once at the end - see SeedMetadataNoRefresh for seeding
// several files in one setup step, where even a single refresh per file is wasteful.
func SeedMetadata(meta *files.Metadata) error {
	if err := SeedMetadataNoRefresh(meta); err != nil {
		return err
	}
	files.RefreshCaches()
	return nil
}

// SeedMetadataNoRefresh is SeedMetadata without the aggregate cache refresh - each refresh is
// a full disk walk of every file (not just this suite's), so a resetAndSeed that seeds several
// sample files should call this in the loop and refresh once afterwards (files.RefreshCaches()
// or files.RebuildAllCaches(), depending on whether the suite needs the rebuild to have
// happened synchronously before its cases run) instead of paying for that walk per file.
func SeedMetadataNoRefresh(meta *files.Metadata) error {
	if err := files.MetaDataSyncNoRefresh(meta.Path); err != nil {
		return err
	}
	if meta.Editor != "" {
		if err := files.SetEditorNoRefresh(meta.Path, meta.Editor); err != nil {
			return err
		}
	}
	if len(meta.Tags) > 0 {
		if err := files.SetTagsNoRefresh(meta.Path, meta.Tags); err != nil {
			return err
		}
	}
	if len(meta.Parents) > 0 {
		if err := files.SetParentsNoRefresh(meta.Path, meta.Parents); err != nil {
			return err
		}
	}
	if !meta.CreatedAt.IsZero() {
		if err := files.SetCreatedAt(meta.Path, meta.CreatedAt); err != nil {
			return err
		}
	}
	if meta.References != nil {
		if err := files.SetReferences(meta.Path, meta.References); err != nil {
			return err
		}
	}
	return nil
}

// SeedMetadataRaw writes meta as-is with no derived-field recomputation, for suites that
// deliberately seed a pre-cascade state (e.g. testing that a rebuild job computes
// Ancestor/Kids/UsedLinks that a raw seed left empty). Mirrors the old MetaDataSaveRaw.
func SeedMetadataRaw(meta *files.Metadata) error {
	return files.MetaDataMutate(meta.Path, func(m *files.Metadata, existed bool) (bool, error) {
		*m = *meta
		return true, nil
	})
}
