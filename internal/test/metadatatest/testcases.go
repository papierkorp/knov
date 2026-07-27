package metadatatest

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

var fixedCreatedAt = time.Date(2021, 6, 15, 0, 0, 0, 0, time.UTC)

// caseMetadataGetSetFields saves every settable field on fieldsFile and checks both the
// auto-derived fields (collection/folders/title/size/timestamps, computed by metaDataUpdate
// from the file's path/content) and the manual ones (tags/editor/createdAt) round-trip.
func caseMetadataGetSetFields() test.CaseResult {
	name := "metadata-get-set-fields"

	err := files.MetaDataSave(&files.Metadata{
		Path:      pathutils.ToWithPrefix(testPath(fieldsFile)),
		Editor:    files.EditorTypeToastUI,
		Tags:      []string{"metadatatest-alpha"},
		CreatedAt: fixedCreatedAt,
	})
	if err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(pathutils.ToWithPrefix(testPath(fieldsFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := got != nil &&
		got.Collection == "test" &&
		slices.Equal(got.Folders, []string{"test", "metadata-tests"}) &&
		got.Editor == files.EditorTypeToastUI &&
		slices.Equal(got.Tags, []string{"metadatatest-alpha"}) &&
		got.CreatedAt.Equal(fixedCreatedAt) &&
		got.Title != "" &&
		got.Size > 0 &&
		!got.LastEdited.IsZero()

	cr := test.CaseResult{
		Name:     name,
		Expected: `collection="test", folders=[test,metadata-tests], editor/tags/createdAt as saved, title/size/lastEdited auto-populated`,
		Actual:   fmt.Sprintf("%+v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "MetaDataGet did not return the expected auto/manual field values after MetaDataSave"
	}
	return cr
}

// caseMetadataPartialUpdate continues from caseMetadataGetSetFields' saved state on
// fieldsFile: a second save with a new Tags value but an empty Editor checks that
// metaDataUpdate only overwrites Tags/Editor/Parents when the new value is non-empty
// (empty means "unspecified", not "clear it" - same semantics kanbantest relies on for tags).
func caseMetadataPartialUpdate() test.CaseResult {
	name := "metadata-partial-update"

	if err := files.MetaDataSave(&files.Metadata{
		Path: pathutils.ToWithPrefix(testPath(fieldsFile)),
		Tags: []string{"metadatatest-beta"},
	}); err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(pathutils.ToWithPrefix(testPath(fieldsFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := got != nil &&
		slices.Equal(got.Tags, []string{"metadatatest-beta"}) &&
		got.Editor == files.EditorTypeToastUI

	cr := test.CaseResult{
		Name:     name,
		Expected: "tags updated to the new value, editor left unchanged (empty Editor on save means unspecified)",
		Actual:   fmt.Sprintf("tags=%v editor=%q", got.Tags, got.Editor),
		Success:  success,
	}
	if !success {
		cr.Error = "MetaDataSave did not apply partial-update semantics as expected"
	}
	return cr
}

func caseMetadataDelete() test.CaseResult {
	name := "metadata-delete"

	path := pathutils.ToWithPrefix(testPath(deleteFile))
	if err := files.MetaDataSave(&files.Metadata{Path: path, Editor: files.EditorTypeToastUI}); err != nil {
		return errCase(name, err)
	}
	if err := files.MetaDataDelete(path); err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}

	success := got == nil
	cr := test.CaseResult{
		Name:     name,
		Expected: "MetaDataGet returns nil after MetaDataDelete",
		Actual:   fmt.Sprintf("got=%v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "MetaDataDelete did not remove the metadata entry"
	}
	return cr
}

func caseMetadataExportAll() test.CaseResult {
	name := "metadata-export-all"

	path := pathutils.ToWithPrefix(testPath(exportFile))
	if err := files.MetaDataSave(&files.Metadata{Path: path, Editor: files.EditorTypeToastUI}); err != nil {
		return errCase(name, err)
	}

	all, err := files.MetaDataExportAll()
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, m := range all {
		if m.Path == path {
			found = true
			break
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("exported metadata includes %s", path),
		Actual:   fmt.Sprintf("found=%v among %d entries", found, len(all)),
		Success:  found,
	}
	if !found {
		cr.Error = "MetaDataExportAll did not include the sample file"
	}
	return cr
}

// caseReferencesAdd replicates handleAPIAddMetadataReference's inline append+save
// (internal/server/api_metadata.go) - there's no exported AddReference to call directly.
func caseReferencesAdd() test.CaseResult {
	name := "references-add"

	path := pathutils.ToWithPrefix(testPath(referencesFile))
	if err := files.MetaDataSave(&files.Metadata{Path: path, Editor: files.EditorTypeToastUI}); err != nil {
		return errCase(name, err)
	}

	metadata, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}
	metadata.References = append(metadata.References, files.Reference{
		URL:         "https://example.com/one",
		Description: "first reference",
		AddedAt:     time.Now(),
	})
	metadata.References = append(metadata.References, files.Reference{
		URL:         "https://example.com/two",
		Description: "second reference",
		AddedAt:     time.Now(),
	})
	if err := files.MetaDataSave(metadata); err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}

	success := len(got.References) == 2 && got.References[0].URL == "https://example.com/one" && got.References[1].URL == "https://example.com/two"
	cr := test.CaseResult{
		Name:     name,
		Expected: "2 references saved in append order",
		Actual:   fmt.Sprintf("%d references: %v", len(got.References), got.References),
		Success:  success,
	}
	if !success {
		cr.Error = "reference add did not persist as expected"
	}
	return cr
}

// caseReferencesRemove continues from caseReferencesAdd's saved state on referencesFile,
// replicating handleAPIDeleteMetadataReference's inline filter+save.
func caseReferencesRemove() test.CaseResult {
	name := "references-remove"

	path := pathutils.ToWithPrefix(testPath(referencesFile))
	metadata, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}

	filtered := metadata.References[:0]
	for _, ref := range metadata.References {
		if ref.URL != "https://example.com/one" {
			filtered = append(filtered, ref)
		}
	}
	metadata.References = filtered
	if err := files.MetaDataSave(metadata); err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}

	success := len(got.References) == 1 && got.References[0].URL == "https://example.com/two"
	cr := test.CaseResult{
		Name:     name,
		Expected: "only the non-matching reference remains after remove",
		Actual:   fmt.Sprintf("%d references: %v", len(got.References), got.References),
		Success:  success,
	}
	if !success {
		cr.Error = "reference remove did not filter as expected"
	}
	return cr
}

func caseAllEditorTypes() test.CaseResult {
	name := "all-editor-types"

	types := files.AllEditorTypes()
	success := len(types) == 7 &&
		slices.Contains(types, files.EditorTypeToastUI) &&
		slices.Contains(types, files.EditorTypeTodo)

	cr := test.CaseResult{
		Name:     name,
		Expected: "7 editor types, including toastui-editor and todo-editor",
		Actual:   fmt.Sprintf("%d types: %v", len(types), types),
		Success:  success,
	}
	if !success {
		cr.Error = "AllEditorTypes did not return the expected set"
	}
	return cr
}

// caseSanitizeKanbanTags exercises SanitizeKanbanTags with one non-kanban tag, one valid
// status tag, and one invalid status tag - the invalid one should be dropped and reported.
func caseSanitizeKanbanTags() test.CaseResult {
	name := "sanitize-kanban-tags"

	validTag := configmanager.KanbanStatusTag("inbox")
	invalidTag := configmanager.GetKanbanPrefix() + "-status-not-a-real-status"

	cleaned, err := files.SanitizeKanbanTags([]string{"unrelated", validTag, invalidTag})

	success := err != nil && strings.Contains(err.Error(), invalidTag) &&
		slices.Contains(cleaned, "unrelated") && slices.Contains(cleaned, validTag) && !slices.Contains(cleaned, invalidTag)

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("cleaned=[unrelated, %s], error mentions %s", validTag, invalidTag),
		Actual:   fmt.Sprintf("cleaned=%v err=%v", cleaned, err),
		Success:  success,
	}
	if !success {
		cr.Error = "SanitizeKanbanTags did not drop the invalid status tag as expected"
	}
	return cr
}
