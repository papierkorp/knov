package metadatatest

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/kanban"
	"knov/internal/pathutils"
	"knov/internal/test"
)

var fixedCreatedAt = time.Date(2021, 6, 15, 0, 0, 0, 0, time.UTC)

// caseMetadataGetSetFields saves every settable field on fieldsFile and checks both the
// auto-derived fields (collection/folders/title/size/timestamps, computed by
// recomputeDerivedFields from the file's path/content) and the manual ones
// (tags/editor/createdAt) round-trip.
func caseMetadataGetSetFields() test.CaseResult {
	name := "metadata-get-set-fields"

	err := test.SeedMetadata(&files.Metadata{
		Path:      pathutils.ToWithPrefix(testPath(fieldsFile)),
		Editor:    files.EditorTypeCodeMirror,
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
		got.Editor == files.EditorTypeCodeMirror &&
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
		cr.Error = "MetaDataGet did not return the expected auto/manual field values after SeedMetadata"
	}
	return cr
}

// caseMetadataPartialUpdate continues from caseMetadataGetSetFields' saved state on
// fieldsFile: setting only Tags via SetTags checks that it changes nothing else, in
// particular leaving the previously-set Editor untouched (SetTags/SetEditor are each
// single-field Mutate wrappers - unlike the old struct-merge, there's no "empty means
// unspecified" convention to verify anymore since callers can no longer pass a partial
// struct at all).
func caseMetadataPartialUpdate() test.CaseResult {
	name := "metadata-partial-update"

	if err := files.SetTags(pathutils.ToWithPrefix(testPath(fieldsFile)), []string{"metadatatest-beta"}); err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(pathutils.ToWithPrefix(testPath(fieldsFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := got != nil &&
		slices.Equal(got.Tags, []string{"metadatatest-beta"}) &&
		got.Editor == files.EditorTypeCodeMirror

	cr := test.CaseResult{
		Name:     name,
		Expected: "tags updated to the new value, editor left unchanged (empty Editor on save means unspecified)",
		Actual:   fmt.Sprintf("tags=%v editor=%q", got.Tags, got.Editor),
		Success:  success,
	}
	if !success {
		cr.Error = "SetTags changed more than tags (editor should stay untouched)"
	}
	return cr
}

func caseMetadataDelete() test.CaseResult {
	name := "metadata-delete"

	path := pathutils.ToWithPrefix(testPath(deleteFile))
	if err := test.SeedMetadata(&files.Metadata{Path: path, Editor: files.EditorTypeCodeMirror}); err != nil {
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
	if err := test.SeedMetadata(&files.Metadata{Path: path, Editor: files.EditorTypeCodeMirror}); err != nil {
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
	if err := test.SeedMetadata(&files.Metadata{Path: path, Editor: files.EditorTypeCodeMirror}); err != nil {
		return errCase(name, err)
	}

	refs := []files.Reference{
		{URL: "https://example.com/one", Description: "first reference", AddedAt: time.Now()},
		{URL: "https://example.com/two", Description: "second reference", AddedAt: time.Now()},
	}
	if err := files.SetReferences(path, refs); err != nil {
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

	var filtered []files.Reference
	for _, ref := range metadata.References {
		if ref.URL != "https://example.com/one" {
			filtered = append(filtered, ref)
		}
	}
	if err := files.SetReferences(path, filtered); err != nil {
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

// caseMutateVsSyncRace runs MoveCard in parallel with MetaDataSyncNoRefresh on the same path.
// The final kanban status must remain the moved status - Sync must not clobber user tags.
func caseMutateVsSyncRace() test.CaseResult {
	name := "mutate-vs-sync-race"

	path := pathutils.ToWithPrefix(testPath(raceFile))
	inbox := configmanager.KanbanStatusTag("inbox")
	if err := test.SeedMetadata(&files.Metadata{
		Path:   path,
		Editor: files.EditorTypeCodeMirror,
		Tags:   []string{"metadatatest-race", inbox},
	}); err != nil {
		return errCase(name, err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			if err := files.MetaDataSyncNoRefresh(path); err != nil {
				errCh <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := kanban.MoveCard(testDir, path, "inprogress"); err != nil {
			errCh <- err
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return errCase(name, err)
		}
	}

	got, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}
	status := kanban.StatusFromTags(got.Tags, configmanager.GetKanbanPrefix())
	success := status == "inprogress" && slices.Contains(got.Tags, "metadatatest-race")
	cr := test.CaseResult{
		Name:     name,
		Expected: `status=inprogress and non-kanban tag "metadatatest-race" preserved after parallel MoveCard+Sync`,
		Actual:   fmt.Sprintf("status=%q tags=%v", status, got.Tags),
		Success:  success,
	}
	if !success {
		cr.Error = "parallel MetaDataSync clobbered MoveCard tags"
	}
	return cr
}

// caseMutateMissingPath checks MetaDataMutate creates a bare record when the path has no metadata yet.
func caseMutateMissingPath() test.CaseResult {
	name := "mutate-missing-path"

	path := pathutils.ToWithPrefix(testPath("metadata-missing-created.md"))
	_ = files.MetaDataDelete(path)

	err := files.MetaDataMutate(path, func(m *files.Metadata, existed bool) (bool, error) {
		if existed {
			return false, fmt.Errorf("expected existed=false")
		}
		m.Editor = files.EditorTypeCodeMirror
		return true, nil
	})
	if err != nil {
		return errCase(name, err)
	}

	got, err := files.MetaDataGet(path)
	if err != nil {
		return errCase(name, err)
	}
	success := got != nil && got.Editor == files.EditorTypeCodeMirror
	cr := test.CaseResult{
		Name:     name,
		Expected: "MetaDataMutate with existed=false creates and saves a bare record",
		Actual:   fmt.Sprintf("got=%+v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "MetaDataMutate did not create metadata for a missing path"
	}
	_ = files.MetaDataDelete(path)
	return cr
}

func caseAllEditorTypes() test.CaseResult {
	name := "all-editor-types"

	types := files.AllEditorTypes()
	success := len(types) == 5 &&
		slices.Contains(types, files.EditorTypeCodeMirror) &&
		slices.Contains(types, files.EditorTypeTodo)

	cr := test.CaseResult{
		Name:     name,
		Expected: "5 editor types, including codemirror-editor and todo-editor",
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
