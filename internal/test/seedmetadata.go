// Package test - metadata seeding helpers for suite sampledata setup.
package test

import "knov/internal/files"

// SeedMetadata seeds path's metadata for test setup: recomputes derived fields via
// MetaDataSync, then applies whichever user-owned fields meta sets (mirrors the old
// MetaDataSave partial-update convention - a zero-value field is left untouched, not
// cleared). Single-threaded suite setup has no concurrent writer to race, so this
// composition is safe here even though production code must use MetaDataMutate/MetaDataSync
// directly instead of a caller-assembled *Metadata (see files package godoc).
func SeedMetadata(meta *files.Metadata) error {
	if err := files.MetaDataSync(meta.Path); err != nil {
		return err
	}
	if meta.Editor != "" {
		if err := files.SetEditor(meta.Path, meta.Editor); err != nil {
			return err
		}
	}
	if len(meta.Tags) > 0 {
		if err := files.SetTags(meta.Path, meta.Tags); err != nil {
			return err
		}
	}
	if len(meta.Parents) > 0 {
		if err := files.SetParents(meta.Path, meta.Parents); err != nil {
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
