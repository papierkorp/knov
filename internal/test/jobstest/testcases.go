package jobstest

import (
	"fmt"
	"os"
	"slices"
	"time"

	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/pathutils"
	"knov/internal/search"
	"knov/internal/test"
)

func containsFilePath(list []files.File, path string) bool {
	for _, f := range list {
		if pathutils.ToRelative(f.Path) == path {
			return true
		}
	}
	return false
}

// caseMetadataFullRebuild covers job.RunFullRebuild - childFile's Parents/linkerFile's
// UsedLinks were seeded via a raw save that deliberately skips metaDataUpdate's cascade
// (see sampledata.go), so Ancestor/Kids/UsedLinks only exist after the rebuild actually
// recomputes them from the Parents field and real markdown link content.
func caseMetadataFullRebuild() test.CaseResult {
	name := "metadata-full-rebuild"

	if err := job.RunFullRebuild(); err != nil {
		return errCase(name, err)
	}

	child, err := files.MetaDataGet(withPrefix(childFile))
	if err != nil {
		return errCase(name, err)
	}
	parent, err := files.MetaDataGet(withPrefix(parentFile))
	if err != nil {
		return errCase(name, err)
	}
	linker, err := files.MetaDataGet(withPrefix(linkerFile))
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Equal(child.Ancestor, []string{withPrefix(parentFile)}) &&
		slices.Contains(parent.Kids, withPrefix(childFile)) &&
		slices.Contains(linker.UsedLinks, testPath(linkedFile))

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("child.Ancestor=[%s], parent.Kids contains %s, linker.UsedLinks contains %s", withPrefix(parentFile), withPrefix(childFile), testPath(linkedFile)),
		Actual:   fmt.Sprintf("ancestor=%v kids=%v usedLinks=%v", child.Ancestor, parent.Kids, linker.UsedLinks),
		Success:  success,
	}
	if !success {
		cr.Error = "RunFullRebuild did not recompute Ancestor/Kids/UsedLinks from Parents/link content as expected"
	}
	return cr
}

func caseSearchReindex() test.CaseResult {
	name := "search-reindex"

	if err := job.RunSearchReindex(); err != nil {
		return errCase(name, err)
	}

	results, err := search.SearchFiles(searchMarker, 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range results {
		if f.Name == searchFile {
			found = true
			break
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("search for %q finds %s after reindex", searchMarker, searchFile),
		Actual:   fmt.Sprintf("found=%v among %d results", found, len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = "RunSearchReindex did not index the sample file's content as expected"
	}
	return cr
}

// caseCacheInvalidate primes the file-list cache, adds a new file via a raw save (which
// skips the cache refresh MetaDataSave normally triggers), confirms the cache is stale,
// then checks job.RunCacheInvalidate() makes the next read see the new file.
func caseCacheInvalidate() test.CaseResult {
	name := "cache-invalidate"

	if _, err := files.GetAllFilesCached(); err != nil {
		return errCase(name, err)
	}

	newFile := testPath("jobs-cache-new.md")
	if err := writeFile(newFile, "# jobs-cache-new.md\n\ncontent\n"); err != nil {
		return errCase(name, err)
	}
	if err := files.MetaDataSaveRaw(&files.Metadata{Path: pathutils.ToWithPrefix(newFile), Editor: files.EditorTypeToastUI}); err != nil {
		return errCase(name, err)
	}

	stale, err := files.GetAllFilesCached()
	if err != nil {
		return errCase(name, err)
	}
	staleMissing := !containsFilePath(stale, newFile)

	if err := job.RunCacheInvalidate(); err != nil {
		return errCase(name, err)
	}

	fresh, err := files.GetAllFilesCached()
	if err != nil {
		return errCase(name, err)
	}
	freshPresent := containsFilePath(fresh, newFile)

	success := staleMissing && freshPresent
	cr := test.CaseResult{
		Name:     name,
		Expected: "new file absent from the primed cache, present after RunCacheInvalidate",
		Actual:   fmt.Sprintf("staleMissing=%v freshPresent=%v", staleMissing, freshPresent),
		Success:  success,
	}
	if !success {
		cr.Error = "RunCacheInvalidate did not force a fresh file-list read as expected"
	}
	return cr
}

// caseMediaCleanup covers job.RunMediaCleanup (= media's orphaned-cleanup bullet too) -
// orphanMediaFile has metadata but nothing linking to it, usedMediaFile is linked from a
// real doc's image embed (see sampledata.go's UpdateLinksForSingleFile call).
func caseMediaCleanup() test.CaseResult {
	name := "media-cleanup"

	if err := files.UpdateOrphanedMediaCache(); err != nil {
		return errCase(name, err)
	}

	result, err := job.RunMediaCleanup()
	if err != nil {
		return errCase(name, err)
	}

	_, orphanStatErr := os.Stat(pathutils.ToMediaPath(mediaPath(orphanMediaFile)))
	orphanGone := os.IsNotExist(orphanStatErr)
	orphanMeta, _ := files.MetaDataGet("media/" + mediaPath(orphanMediaFile))

	_, usedStatErr := os.Stat(pathutils.ToMediaPath(mediaPath(usedMediaFile)))
	usedKept := usedStatErr == nil

	success := result.Deleted >= 1 && orphanGone && orphanMeta == nil && usedKept
	cr := test.CaseResult{
		Name:     name,
		Expected: "orphaned media deleted from disk+metadata, used media kept",
		Actual:   fmt.Sprintf("deleted=%d orphanGone=%v orphanMetaNil=%v usedKept=%v", result.Deleted, orphanGone, orphanMeta == nil, usedKept),
		Success:  success,
	}
	if !success {
		cr.Error = "RunMediaCleanup did not delete only the orphaned media file as expected"
	}
	return cr
}

// caseManualTrigger covers job.RunAsync (the admin "run all jobs now" button) - it runs
// file-sync/search-reindex/metadata-rebuild/notification-purge sequentially in a goroutine,
// so this polls job.GetRecentRuns() for notification-purge (the last step, and the only one
// with no independent periodic ticker to race against) to complete.
func caseManualTrigger() test.CaseResult {
	name := "manual-trigger"

	triggeredAt := time.Now()
	if err := job.RunAsync(); err != nil {
		return errCase(name, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var found *job.JobRun
	for time.Now().Before(deadline) {
		for _, run := range job.GetRecentRuns() {
			if run.Name == "notification-purge" && !run.StartedAt.Before(triggeredAt) && run.FinishedAt != nil {
				r := run
				found = &r
				break
			}
		}
		if found != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	success := found != nil && found.Status == job.JobStatusOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "notification-purge step of the manual run completes with status ok",
		Actual:   fmt.Sprintf("found=%v", found),
		Success:  success,
	}
	if !success {
		cr.Error = "RunAsync's manual job sequence did not complete as expected within the timeout"
	}
	return cr
}

// caseJobHistory covers job.GetRecentRuns/IsRunning - triggers a job synchronously and
// checks its history entry.
func caseJobHistory() test.CaseResult {
	name := "job-history"

	if err := job.RunCacheInvalidate(); err != nil {
		return errCase(name, err)
	}

	runs := job.GetRecentRuns()
	recorded := len(runs) > 0 && runs[0].Name == "cache-invalidate" && runs[0].Status == job.JobStatusOK && runs[0].FinishedAt != nil
	stillRunning := job.IsRunning("cache-invalidate")

	success := recorded && !stillRunning
	cr := test.CaseResult{
		Name:     name,
		Expected: "newest history entry is cache-invalidate/ok/finished, IsRunning false afterward",
		Actual:   fmt.Sprintf("recorded=%v stillRunning=%v", recorded, stillRunning),
		Success:  success,
	}
	if !success {
		cr.Error = "GetRecentRuns/IsRunning did not reflect the just-completed job as expected"
	}
	return cr
}
