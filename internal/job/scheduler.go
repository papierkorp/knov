package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/jobStorage"
	"knov/internal/kanban"
	"knov/internal/logging"
)

// gitRepackInterval is fixed rather than config-driven - RunGitRepack is a cheap no-op below
// the loose-object threshold, so how often it's checked barely matters.
const gitRepackInterval = 24 * time.Hour

// backupAutoCheckInterval is fixed, not the user-facing setting - checkAutoBackup is a cheap
// no-op when no auto-backup profile is configured or due. The schedule users actually control is
// each profile's own cron in configmanager.BackupAutoProfiles (when a backup is taken), not this
// check's own cadence.
const backupAutoCheckInterval = 15 * time.Minute

var (
	stopChan                chan bool
	fileInterval            time.Duration
	searchInterval          time.Duration
	metadataRebuildInterval time.Duration

	fileMu          sync.Mutex
	searchMu        sync.Mutex
	rebuildMu       sync.Mutex
	filterMu        sync.Mutex
	notifMu         sync.Mutex
	jobRecordMu     sync.Mutex
	cacheInvalidMu  sync.Mutex
	mediaCleanupMu  sync.Mutex
	gitPullMu       sync.Mutex
	gitPushMu       sync.Mutex
	repairLinksMu   sync.Mutex
	testdataSetupMu sync.Mutex
	testdataCleanMu sync.Mutex
	runMu           sync.Mutex // prevents concurrent manual Run() calls

	bulkDeleteFilesMu    sync.Mutex
	deleteFolderMu       sync.Mutex
	moveFolderMu         sync.Mutex
	kanbanRenameMu       sync.Mutex
	bulkUpdateMetadataMu sync.Mutex
	exportMu             sync.Mutex

	// backupMu also guards restore, since restore starts with a full backup - the two must not
	// run concurrently, or two same-second-precision set names could race on the same target.
	backupMu sync.Mutex

	// asyncRuns holds every StartAsync job currently in flight, keyed by its jobStorage id:
	// cancel is looked up by CancelAsync to stop a specific run, job by GetProgress to read a
	// running job's live Progress counter.
	asyncRunsMu sync.Mutex
	asyncRuns   = map[string]asyncRun{}
)

type asyncRun struct {
	cancel context.CancelFunc
	job    Job
}

// execute runs job under mu, recording start/finish in job history.
// Returns ErrAlreadyRunning if the job is already active, or the job's own error.
func execute(mu *sync.Mutex, job Job) error {
	if !mu.TryLock() {
		logging.LogDebug(logging.KeyApp, "%s job already running, skipping", job.Name())
		return fmt.Errorf("%s: %w", job.Name(), ErrAlreadyRunning)
	}
	defer mu.Unlock()
	return runLocked(context.Background(), job, "")
}

// runLocked runs job and records start/finish in job history, assuming the caller already
// holds job's dedup mutex (and will unlock it). Shared by execute (synchronous callers, id "")
// and StartAsync (background callers, which additionally persist to jobStorage around this and
// pass their jobStorage id so the ring-buffer entry can be matched back to it - see GetHistory).
func runLocked(ctx context.Context, job Job, id string) error {
	slot := recordStart(job.Name(), id)
	defer func() {
		if r := recover(); r != nil {
			recordFinish(slot, JobStatusError, fmt.Sprintf("panic: %v", r), nil)
			panic(r) // re-panic so the runtime still logs it
		}
	}()
	if err := job.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			recordFinish(slot, JobStatusCanceled, "", nil)
		} else {
			recordFinish(slot, JobStatusError, err.Error(), nil)
		}
		return err
	}
	var msg string
	if m, ok := job.(Messenger); ok {
		msg = m.Message()
	}
	var output any
	if o, ok := job.(Outputter); ok {
		output = o.Output()
	}
	recordFinish(slot, JobStatusOK, msg, output)
	return nil
}

// StartAsync runs job in a background goroutine under mu, persisting its progress via
// jobStorage so status can be polled by id and an interrupted run detected on next startup.
// Returns ErrAlreadyRunning synchronously (no race between the check and the goroutine start)
// if mu is already held. args is a job-type-specific JSON blob used to resume the job after a
// crash (see Resumable) - pass "" for jobs that don't need it.
func StartAsync(mu *sync.Mutex, job Job, args string) (string, error) {
	if !mu.TryLock() {
		return "", fmt.Errorf("%s: %w", job.Name(), ErrAlreadyRunning)
	}
	id := generateJobID()
	if err := jobStorage.Create(id, job.Name(), args); err != nil {
		mu.Unlock()
		return "", fmt.Errorf("failed to persist job %s: %w", job.Name(), err)
	}
	runAsync(mu, job, id)
	return id, nil
}

// resumeAsync re-runs job in the background under mu, reusing id's existing "running"
// jobStorage row from before a restart instead of creating a new one - so a client still
// polling the pre-restart job id observes the resumed run's real outcome instead of a dead
// end pointing at an abandoned id.
func resumeAsync(mu *sync.Mutex, job Job, id string) error {
	if !mu.TryLock() {
		return fmt.Errorf("%s: %w", job.Name(), ErrAlreadyRunning)
	}
	runAsync(mu, job, id)
	return nil
}

// runAsync runs job under mu (already locked by the caller) in a background goroutine and
// persists its outcome to jobStorage under id. A panic in job.Run() is recovered here rather
// than left to crash the process - runLocked already re-panics after recording it in the
// in-app job history, which is fine for execute's synchronous, request-scoped callers (a
// panic there is caught by the HTTP server's own per-request recovery) but would take down
// the whole process for this bare background goroutine.
//
// A cancelable context is registered under id for the duration of the run, so a concurrent
// CancelAsync(id) call can request early termination - see CancelAsync.
func runAsync(mu *sync.Mutex, job Job, id string) {
	ctx, cancel := context.WithCancel(context.Background())
	asyncRunsMu.Lock()
	asyncRuns[id] = asyncRun{cancel: cancel, job: job}
	asyncRunsMu.Unlock()

	go func() {
		defer mu.Unlock()
		defer cancel()
		defer func() {
			asyncRunsMu.Lock()
			delete(asyncRuns, id)
			asyncRunsMu.Unlock()
		}()

		status, errMsg := jobStorage.StatusDone, ""
		func() {
			defer func() {
				if r := recover(); r != nil {
					status, errMsg = jobStorage.StatusError, fmt.Sprintf("panic: %v", r)
				}
			}()
			switch err := runLocked(ctx, job, id); {
			case errors.Is(err, context.Canceled):
				status, errMsg = jobStorage.StatusCanceled, ""
			case err != nil:
				status, errMsg = jobStorage.StatusError, err.Error()
			}
		}()
		if err := jobStorage.UpdateStatus(id, status, errMsg); err != nil {
			logging.LogError(logging.KeyApp, "failed to persist finished status for job %s (%s): %v", job.Name(), id, err)
		}
		notifyAsyncFinished(job.Name(), status, errMsg)
	}()
}

// CancelAsync requests cancellation of the StartAsync job with the given id by canceling its
// context. Cancellation is cooperative - whether/how soon the job actually stops depends on its
// Run() checking ctx.Err() (e.g. between loop iterations); a job that doesn't check it keeps
// running to completion unaffected. Returns ErrNotRunning if id has no job currently in flight
// (already finished, or never existed).
func CancelAsync(id string) error {
	asyncRunsMu.Lock()
	run, ok := asyncRuns[id]
	asyncRunsMu.Unlock()
	if !ok {
		return fmt.Errorf("job %s: %w", id, ErrNotRunning)
	}
	run.cancel()
	return nil
}

// generateJobID returns a unique id for a StartAsync job record.
func generateJobID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		logging.LogWarning(logging.KeyApp, "generateJobID: failed to read random bytes: %v", err)
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// Start begins the cronjob scheduler.
func Start() {
	stopChan = make(chan bool)

	fileIntervalStr := configmanager.GetAppConfig().CronjobInterval
	parsedFileInterval, err := time.ParseDuration(fileIntervalStr)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "invalid cronjob interval '%s', using default 5m", fileIntervalStr)
		parsedFileInterval = 5 * time.Minute
	}
	fileInterval = parsedFileInterval

	searchIntervalStr := configmanager.GetAppConfig().SearchIndexInterval
	parsedSearchInterval, err := time.ParseDuration(searchIntervalStr)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "invalid search index interval '%s', using default 15m", searchIntervalStr)
		parsedSearchInterval = 15 * time.Minute
	}
	searchInterval = parsedSearchInterval

	metadataRebuildIntervalStr := configmanager.GetAppConfig().MetadataRebuildInterval
	parsedMetadataRebuildInterval, err := time.ParseDuration(metadataRebuildIntervalStr)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "invalid metadata rebuild interval '%s', using default 30m", metadataRebuildIntervalStr)
		parsedMetadataRebuildInterval = 30 * time.Minute
	}
	metadataRebuildInterval = parsedMetadataRebuildInterval

	go func() {
		ticker := time.NewTicker(fileInterval)
		defer ticker.Stop()
		RunFileSync() // run once on startup
		for {
			select {
			case <-ticker.C:
				RunFileSync()
			case <-stopChan:
				logging.LogInfo(logging.KeyApp, "file cronjob stopped")
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(searchInterval)
		defer ticker.Stop()
		RunSearchReindex() // run once on startup
		for {
			select {
			case <-ticker.C:
				RunSearchReindex()
			case <-stopChan:
				logging.LogInfo(logging.KeyApp, "search cronjob stopped")
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(metadataRebuildInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				RunMetadataRebuild()
			case <-stopChan:
				logging.LogInfo(logging.KeyApp, "metadata rebuild cronjob stopped")
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(gitRepackInterval)
		defer ticker.Stop()
		RunGitRepack() // check once on startup - cheap no-op unless already over threshold
		for {
			select {
			case <-ticker.C:
				RunGitRepack()
			case <-stopChan:
				logging.LogInfo(logging.KeyApp, "git repack cronjob stopped")
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(backupAutoCheckInterval)
		defer ticker.Stop()
		checkAutoBackup() // check once on startup - covers a due backup missed while the app was down
		for {
			select {
			case <-ticker.C:
				checkAutoBackup()
			case <-stopChan:
				logging.LogInfo(logging.KeyApp, "backup auto-check stopped")
				return
			}
		}
	}()

	logging.LogInfo(logging.KeyApp, "cronjob scheduler started (file: %v, search: %v, metadata rebuild: %v, git repack: %v)", fileInterval, searchInterval, metadataRebuildInterval, gitRepackInterval)
}

// Stop stops the cronjob scheduler.
func Stop() {
	if stopChan != nil {
		close(stopChan)
	}
}

// RunFileSync runs the file-sync job with dedup protection.
func RunFileSync() error {
	return execute(&fileMu, &fileJob{})
}

// RunSearchReindex runs the search-reindex job with dedup protection.
func RunSearchReindex() error {
	return execute(&searchMu, &searchIndexJob{})
}

// RunMetadataRebuild runs the scheduled metadata-links-rebuild job with dedup protection.
func RunMetadataRebuild() error {
	return execute(&rebuildMu, &rebuildJob{})
}

// RunFilterReindex runs the filter-reindex job with dedup protection.
func RunFilterReindex() error {
	return execute(&filterMu, &filterJob{})
}

// RunNotificationPurge runs the notification-purge job with dedup protection.
func RunNotificationPurge() error {
	return execute(&notifMu, &notifJob{})
}

// RunJobRecordPurge trims the persistent jobStorage table (async-job records) with dedup protection.
func RunJobRecordPurge() error {
	return execute(&jobRecordMu, &jobRecordPurgeJob{})
}

// RunCacheInvalidate clears the cache and records it in the job history.
func RunCacheInvalidate() error {
	return execute(&cacheInvalidMu, &cacheInvalidateJob{})
}

// RunMediaCleanup deletes the selected orphaned media files with dedup protection.
// Returns the cleanup result alongside any fatal error.
func RunMediaCleanup(paths []string) (MediaCleanupResult, error) {
	j := &mediaCleanupJob{paths: paths}
	if err := execute(&mediaCleanupMu, j); err != nil {
		return MediaCleanupResult{}, err
	}
	return j.result, nil
}

// RunMediaRelocate moves the selected misplaced media files from the docs folder into the media folder with
// dedup protection. Shares media cleanup's mutex - moved files look orphaned until their docs
// are relinked, so a cleanup running in between could delete them.
func RunMediaRelocate(paths []string) (files.MediaRelocateResult, error) {
	j := &mediaRelocateJob{paths: paths}
	if err := execute(&mediaCleanupMu, j); err != nil {
		return files.MediaRelocateResult{}, err
	}
	return j.result, nil
}

// RunRepairBrokenLinks applies the selected broken-link repairs with dedup protection.
func RunRepairBrokenLinks(entries []string) (RepairBrokenLinksResult, error) {
	j := &repairBrokenLinksJob{entries: entries}
	if err := execute(&repairLinksMu, j); err != nil {
		return RepairBrokenLinksResult{}, err
	}
	return j.result, nil
}

// RunMigrateRelativeLinks rewrites the selected bare links to their docs-root form with dedup
// protection - shares the lock with the broken links repair, both rewrite links.
func RunMigrateRelativeLinks(changes []files.RelativeLinkChange) (MigrateRelativeLinksResult, error) {
	j := &migrateRelativeLinksJob{changes: changes}
	if err := execute(&repairLinksMu, j); err != nil {
		return MigrateRelativeLinksResult{}, err
	}
	return j.result, nil
}

// RunMigrateReservedFolders moves the legacy metadata records and filter / tracker ids of docs files
// in docs/docs, docs/media and docs/files to their docs/ keys. Shares repairLinksMu with the other
// admin migrations.
func RunMigrateReservedFolders() (MigrateReservedFoldersResult, error) {
	j := &migrateReservedFoldersJob{}
	if err := execute(&repairLinksMu, j); err != nil {
		return MigrateReservedFoldersResult{}, err
	}
	return j.result, nil
}

// RunGitPull runs a git pull --rebase with dedup protection.
func RunGitPull() error {
	return execute(&gitPullMu, &gitPullJob{})
}

// RunGitPush runs a git push with dedup protection.
func RunGitPush() error {
	return execute(&gitPushMu, &gitPushJob{})
}

// RunGitRepack packs loose git objects when their count crosses the threshold. Shares fileMu
// with the periodic file-sync job rather than its own mutex - that job does its own git pull
// and commit, and go-git's repack rewriting the object store while those add objects is unsafe.
// A repack skipped because fileMu is busy just retries on the next tick.
func RunGitRepack() error {
	return execute(&fileMu, &gitRepackJob{})
}

// RunMoveFolder moves a folder to a new parent and updates its files' links with dedup protection.
func RunMoveFolder(currentPath, newPath string) (BulkUpdateResult, error) {
	j := &moveFolderJob{currentPath: currentPath, newPath: newPath}
	if err := execute(&moveFolderMu, j); err != nil {
		return BulkUpdateResult{}, err
	}
	return j.result, nil
}

// RunKanbanRenameStatus renames a kanban status (settings, tags, foldersync folders, card order,
// events) with dedup protection.
func RunKanbanRenameStatus(oldStatus, newStatus string) (kanban.RenameResult, error) {
	j := &kanbanRenameStatusJob{oldStatus: oldStatus, newStatus: newStatus}
	if err := execute(&kanbanRenameMu, j); err != nil {
		return kanban.RenameResult{}, err
	}
	return j.result, nil
}

// RunBulkUpdateMetadata applies a metadata patch to the given (pre-resolved/filter-matched)
// files with dedup protection.
func RunBulkUpdateMetadata(matched []files.File, patch files.BulkUpdatePatch) (BulkUpdateResult, error) {
	j := &bulkUpdateMetadataJob{matched: matched, patch: patch}
	if err := execute(&bulkUpdateMetadataMu, j); err != nil {
		return BulkUpdateResult{}, err
	}
	return j.result, nil
}

// RunTestdataSetup sets up test data with dedup protection.
func RunTestdataSetup() error {
	return execute(&testdataSetupMu, &testdataSetupJob{})
}

// RunTestdataClean cleans test data with dedup protection.
func RunTestdataClean() error {
	return execute(&testdataCleanMu, &testdataCleanJob{})
}

// RunAsync starts a manual run of all jobs in a background goroutine.
// Acquires runMu synchronously so the caller gets ErrAlreadyRunning immediately
// if a run is already in progress — no race between the check and the goroutine start.
func RunAsync() error {
	if !runMu.TryLock() {
		return fmt.Errorf("manual run: %w", ErrAlreadyRunning)
	}
	go func() {
		defer runMu.Unlock()
		logging.LogInfo(logging.KeyManualCronjob, "manual run started")

		steps := []struct {
			name string
			run  func() error
		}{
			{"file-sync", RunFileSync}, // includes filter-reindex as a sub-step
			{"search-reindex", RunSearchReindex},
			{"metadata-rebuild", RunMetadataRebuild},
			{"job-record-purge", RunJobRecordPurge},
			{"notification-purge", RunNotificationPurge},
		}

		ok := 0
		for _, step := range steps {
			switch err := step.run(); {
			case err == nil:
				logging.LogInfo(logging.KeyManualCronjob, "%s: ok", step.name)
				ok++
			case errors.Is(err, ErrAlreadyRunning):
				logging.LogWarning(logging.KeyManualCronjob, "%s: skipped (already running)", step.name)
			default:
				logging.LogError(logging.KeyManualCronjob, "%s: failed: %v", step.name, err)
			}
		}

		logging.LogInfo(logging.KeyManualCronjob, "manual run completed: %d/%d ok", ok, len(steps))
	}()
	return nil
}
