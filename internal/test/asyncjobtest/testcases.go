package asyncjobtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/jobStorage"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// caseDedupMutexHeldUntilPersisted covers StartAsync/runAsync's dedup guarantee: a second
// StartAsync on the same mutex fails synchronously while a run is in flight, and the mutex is
// only released once jobStorage reflects the terminal status - not right after job.Run()
// returns. The final mu.Lock() below only succeeds once runAsync's deferred mu.Unlock() runs,
// which happens strictly after its jobStorage.UpdateStatus call (same goroutine, program
// order) - so the status read right after is guaranteed to already be persisted, not a race.
func caseDedupMutexHeldUntilPersisted() test.CaseResult {
	name := "startasync-dedup-mutex-held-until-persisted"

	var mu sync.Mutex
	release := make(chan struct{})
	probe := &probeJob{name: "asyncjobtest-dedup-probe", release: release}

	id, err := job.StartAsync(&mu, probe, "")
	if err != nil {
		return errCase(name, err)
	}

	_, dupErr := job.StartAsync(&mu, &probeJob{name: "asyncjobtest-dedup-probe-2"}, "")
	dedupOK := errors.Is(dupErr, job.ErrAlreadyRunning)

	recBefore, err := jobStorage.Get(id)
	if err != nil {
		return errCase(name, err)
	}
	stillRunning := recBefore != nil && recBefore.Status == jobStorage.StatusRunning

	close(release)
	mu.Lock()
	mu.Unlock()

	recAfter, err := jobStorage.Get(id)
	if err != nil {
		return errCase(name, err)
	}
	persistedDone := recAfter != nil && recAfter.Status == jobStorage.StatusDone

	success := dedupOK && stillRunning && persistedDone
	cr := test.CaseResult{
		Name:     name,
		Expected: "concurrent StartAsync on the same mutex fails while running, and jobStorage already shows done by the time the mutex is released",
		Actual:   fmt.Sprintf("dedupErrIsAlreadyRunning=%v stillRunningBeforeRelease=%v doneOnceUnlocked=%v", dedupOK, stillRunning, persistedDone),
		Success:  success,
	}
	if !success {
		cr.Error = "StartAsync did not hold the dedup mutex until jobStorage was updated as expected"
	}
	return cr
}

// casePanicRecoveryBridgesToJobStorage covers a panic in Job.Run() being recovered twice -
// once in runLocked (recorded to the in-app job history, then re-panicked) and once in
// runAsync (which turns it into a jobStorage error status) - instead of crashing the process.
func casePanicRecoveryBridgesToJobStorage() test.CaseResult {
	name := "runasync-panic-recovery"

	const panicMsg = "asyncjobtest deliberate probe panic"
	var mu sync.Mutex
	probe := &probeJob{name: "asyncjobtest-panic-probe", panic: panicMsg}

	id, err := job.StartAsync(&mu, probe, "")
	if err != nil {
		return errCase(name, err)
	}

	// blocks until the panic has been recovered and jobStorage updated - see
	// caseDedupMutexHeldUntilPersisted for why this ordering is guaranteed, not racy.
	mu.Lock()
	mu.Unlock()

	rec, err := jobStorage.Get(id)
	if err != nil {
		return errCase(name, err)
	}
	statusOK := rec != nil && rec.Status == jobStorage.StatusError
	msgOK := rec != nil && strings.Contains(rec.Error, panicMsg)

	// the mutex must have been released despite the panic, or this would fail immediately.
	_, reuseErr := job.StartAsync(&mu, &probeJob{name: "asyncjobtest-panic-probe-reuse"}, "")
	reuseOK := reuseErr == nil

	success := statusOK && msgOK && reuseOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "a panicking job ends up jobStorage status=error with the panic message, mutex released",
		Actual:   fmt.Sprintf("statusError=%v messageContainsPanic=%v mutexReleased=%v", statusOK, msgOK, reuseOK),
		Success:  success,
	}
	if !success {
		cr.Error = "a panic in Job.Run() was not recovered/persisted as expected"
	}
	return cr
}

// caseRecoverInterruptedResumable covers RecoverInterrupted replaying a resumable job type
// (bulk-delete-files) with its persisted args after a simulated crash - a jobStorage row left
// "running" (as if the process died mid-run) with no in-memory goroutine behind it. Note:
// RecoverInterrupted scans every row currently marked running, including any real job a live
// app instance happens to have in flight - same caveat other suites accept for real global state.
func caseRecoverInterruptedResumable() test.CaseResult {
	name := "recoverinterrupted-resumable"

	const fileName = "asyncjob-resume-target.md"
	if err := writeFile(testPath(fileName), "# asyncjob-resume-target.md\n\ncontent\n"); err != nil {
		return errCase(name, err)
	}
	if err := test.SeedMetadataNoRefresh(&files.Metadata{Path: withPrefix(fileName), Editor: files.EditorTypeCodeMirror}); err != nil {
		return errCase(name, err)
	}
	fullPath := pathutils.ToDocsPath(testPath(fileName))

	args, err := json.Marshal(struct {
		FullPaths []string `json:"fullPaths"`
		GroupType string   `json:"groupType"`
		GroupVal  string   `json:"groupVal"`
	}{FullPaths: []string{fullPath}, GroupType: "asyncjobtest", GroupVal: "resume-probe"})
	if err != nil {
		return errCase(name, err)
	}

	id := fmt.Sprintf("asyncjobtest-resume-%d", time.Now().UnixNano())
	if err := jobStorage.Create(id, job.JobTypeBulkDeleteFiles, string(args)); err != nil {
		return errCase(name, err)
	}

	job.RecoverInterrupted()

	rec, err := waitForTerminal(id, 10*time.Second)
	if err != nil {
		return errCase(name, err)
	}
	statusOK := rec != nil && rec.Status == jobStorage.StatusDone

	_, statErr := os.Stat(fullPath)
	fileGone := os.IsNotExist(statErr)

	success := statusOK && fileGone
	var status string
	if rec != nil {
		status = rec.Status
	}
	cr := test.CaseResult{
		Name:     name,
		Expected: "resumable job type replayed with its persisted args: jobStorage status=done, target file deleted",
		Actual:   fmt.Sprintf("status=%s fileGone=%v", status, fileGone),
		Success:  success,
	}
	if !success {
		cr.Error = "RecoverInterrupted did not resume the persisted bulk-delete-files job as expected"
	}
	return cr
}

// caseRecoverInterruptedNonResumable covers RecoverInterrupted marking an unrecognized job type
// interrupted and surfacing it via a pending notification, instead of silently dropping it.
func caseRecoverInterruptedNonResumable() test.CaseResult {
	name := "recoverinterrupted-non-resumable"

	if err := drainPending(); err != nil {
		return errCase(name, err)
	}

	const unregisteredType = "asyncjobtest-unregistered-type"
	id := fmt.Sprintf("asyncjobtest-interrupted-%d", time.Now().UnixNano())
	if err := jobStorage.Create(id, unregisteredType, ""); err != nil {
		return errCase(name, err)
	}

	job.RecoverInterrupted()

	rec, err := jobStorage.Get(id)
	if err != nil {
		return errCase(name, err)
	}
	statusOK := rec != nil && rec.Status == jobStorage.StatusInterrupted

	notified, err := notificationStorage.ConsumePending()
	if err != nil {
		return errCase(name, err)
	}
	notifyOK := notified != nil && strings.Contains(notified.Message, unregisteredType)

	success := statusOK && notifyOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "unrecognized job type ends up jobStorage status=interrupted with a pending notification",
		Actual:   fmt.Sprintf("statusInterrupted=%v notified=%v", statusOK, notifyOK),
		Success:  success,
	}
	if !success {
		cr.Error = "RecoverInterrupted did not mark the unrecognized job type interrupted as expected"
	}
	return cr
}

// caseRemoveEmptyDirTreeSuccess covers files.RemoveEmptyDirTree removing a directory tree left
// with only empty subdirectories (the normal post-delete case).
func caseRemoveEmptyDirTreeSuccess() test.CaseResult {
	name := "removeemptydirtree-empty"

	root := pathutils.ToDocsPath(testPath("rmdir-empty"))
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0755); err != nil {
		return errCase(name, err)
	}

	if err := files.RemoveEmptyDirTree(root); err != nil {
		return errCase(name, err)
	}

	_, statErr := os.Stat(root)
	gone := os.IsNotExist(statErr)

	cr := test.CaseResult{
		Name:     name,
		Expected: "an empty (nested) directory tree is fully removed",
		Actual:   fmt.Sprintf("gone=%v", gone),
		Success:  gone,
	}
	if !gone {
		cr.Error = "RemoveEmptyDirTree did not remove an empty directory tree as expected"
	}
	return cr
}

// caseRemoveEmptyDirTreeRefusesNonEmpty covers files.RemoveEmptyDirTree refusing to remove a
// directory tree that still has a file in it - a partial-delete failure (e.g. a file written
// into the tree after the delete's snapshot was taken) must surface as an error, not silently
// remove the folder anyway.
func caseRemoveEmptyDirTreeRefusesNonEmpty() test.CaseResult {
	name := "removeemptydirtree-refuses-nonempty"

	root := pathutils.ToDocsPath(testPath("rmdir-nonempty"))
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		return errCase(name, err)
	}
	leftover := filepath.Join(nested, "leftover.md")
	if err := os.WriteFile(leftover, []byte("still here\n"), 0644); err != nil {
		return errCase(name, err)
	}

	refused := files.RemoveEmptyDirTree(root) != nil

	_, statErr := os.Stat(leftover)
	fileSurvived := statErr == nil

	success := refused && fileSurvived
	cr := test.CaseResult{
		Name:     name,
		Expected: "a directory tree with a leftover file errors instead of being removed, file survives",
		Actual:   fmt.Sprintf("errored=%v fileSurvived=%v", refused, fileSurvived),
		Success:  success,
	}
	if !success {
		cr.Error = "RemoveEmptyDirTree did not refuse to remove a non-empty directory tree as expected"
	}
	return cr
}
