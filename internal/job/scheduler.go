package job

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/jobStorage"
	"knov/internal/logging"
	"knov/internal/test"
)

// gitRepackInterval is fixed rather than config-driven - RunGitRepack is a cheap no-op below
// the loose-object threshold, so how often it's checked barely matters.
const gitRepackInterval = 24 * time.Hour

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
	bulkUpdateMetadataMu sync.Mutex
)

// execute runs job under mu, recording start/finish in job history.
// Returns ErrAlreadyRunning if the job is already active, or the job's own error.
func execute(mu *sync.Mutex, job Job) error {
	if !mu.TryLock() {
		logging.LogDebug(logging.KeyApp, "%s job already running, skipping", job.Name())
		return fmt.Errorf("%s: %w", job.Name(), ErrAlreadyRunning)
	}
	defer mu.Unlock()
	return runLocked(job)
}

// runLocked runs job and records start/finish in job history, assuming the caller already
// holds job's dedup mutex (and will unlock it). Shared by execute (synchronous callers) and
// StartAsync (background callers, which additionally persist to jobStorage around this).
func runLocked(job Job) error {
	slot := recordStart(job.Name())
	defer func() {
		if r := recover(); r != nil {
			recordFinish(slot, JobStatusError, fmt.Sprintf("panic: %v", r), nil)
			panic(r) // re-panic so the runtime still logs it
		}
	}()
	if err := job.Run(); err != nil {
		recordFinish(slot, JobStatusError, err.Error(), nil)
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
	if sr, ok := output.(*test.SuiteResult); ok && sr != nil {
		if sr.Failed == 0 {
			logging.LogInfo(logging.KeyInAppTests, "suite %s: %d passed, %d failed", sr.Suite, sr.Passed, sr.Failed)
		} else {
			var failedNames []string
			for _, c := range sr.Cases {
				if !c.Success {
					failedNames = append(failedNames, fmt.Sprintf("%s: %s", c.Name, c.Error))
				}
			}
			logging.LogWarning(logging.KeyInAppTests, "suite %s: %d passed, %d failed (%s)", sr.Suite, sr.Passed, sr.Failed, strings.Join(failedNames, ", "))
		}
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
	go func() {
		defer mu.Unlock()
		status, errMsg := jobStorage.StatusDone, ""
		if err := runLocked(job); err != nil {
			status, errMsg = jobStorage.StatusError, err.Error()
		}
		if err := jobStorage.UpdateStatus(id, status, errMsg); err != nil {
			logging.LogError(logging.KeyApp, "failed to persist finished status for job %s (%s): %v", job.Name(), id, err)
		}
	}()
	return id, nil
}

// generateJobID returns a unique id for a StartAsync job record.
func generateJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
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

// RunCacheInvalidate clears the cache and records it in the job history.
func RunCacheInvalidate() error {
	return execute(&cacheInvalidMu, &cacheInvalidateJob{})
}

// RunMediaCleanup deletes orphaned media files with dedup protection.
// Returns the cleanup result alongside any fatal error.
func RunMediaCleanup() (MediaCleanupResult, error) {
	j := &mediaCleanupJob{}
	if err := execute(&mediaCleanupMu, j); err != nil {
		return MediaCleanupResult{}, err
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

// Every in-app test suite registers itself via RegisterSuiteRunner in its own init() (see
// externalsuite.go) instead of getting a dedicated wrapper type + mutex here - each of these
// is a thin, discoverable, named entry point for the API handlers over that shared mechanism.

// RunFilterTest runs the filter test suite and returns its results alongside any error.
func RunFilterTest() (*test.SuiteResult, error) { return RunSuiteTest("filter-test") }

// RunEditorsTest runs the editors test suite and returns its results alongside any error.
func RunEditorsTest() (*test.SuiteResult, error) { return RunSuiteTest("editors-test") }

// RunSearchTest runs the search test suite and returns its results alongside any error.
func RunSearchTest() (*test.SuiteResult, error) { return RunSuiteTest("search-test") }

// RunGitHistoryTest runs the git repo/file history test suite and returns its results alongside any error.
func RunGitHistoryTest() (*test.SuiteResult, error) { return RunSuiteTest("git-history-test") }

// RunChatTest runs the chat test suite and returns its results alongside any error.
func RunChatTest() (*test.SuiteResult, error) { return RunSuiteTest("chat-test") }

// RunDashboardTest runs the dashboard test suite and returns its results alongside any error.
func RunDashboardTest() (*test.SuiteResult, error) { return RunSuiteTest("dashboard-test") }

// RunKanbanTest runs the kanban test suite and returns its results alongside any error.
func RunKanbanTest() (*test.SuiteResult, error) { return RunSuiteTest("kanban-test") }

// RunBrowseTest runs the browse test suite and returns its results alongside any error.
func RunBrowseTest() (*test.SuiteResult, error) { return RunSuiteTest("browse-test") }

// RunMetadataTest runs the metadata test suite and returns its results alongside any error.
func RunMetadataTest() (*test.SuiteResult, error) { return RunSuiteTest("metadata-test") }

// RunConnectionsTest runs the connections test suite and returns its results alongside any error.
func RunConnectionsTest() (*test.SuiteResult, error) { return RunSuiteTest("connections-test") }

// RunJobsTest runs the jobs test suite and returns its results alongside any error.
func RunJobsTest() (*test.SuiteResult, error) { return RunSuiteTest("jobs-test") }

// RunMediaTest runs the media test suite and returns its results alongside any error.
func RunMediaTest() (*test.SuiteResult, error) { return RunSuiteTest("media-test") }

// RunExportTest runs the export/import test suite and returns its results alongside any error.
func RunExportTest() (*test.SuiteResult, error) { return RunSuiteTest("export-test") }

// RunNotificationTest runs the notification test suite and returns its results alongside any error.
func RunNotificationTest() (*test.SuiteResult, error) { return RunSuiteTest("notification-test") }

// RunSettingsTest runs the settings/themes/config test suite and returns its results alongside any error.
func RunSettingsTest() (*test.SuiteResult, error) { return RunSuiteTest("settings-test") }

// RunLogsTest runs the logs test suite and returns its results alongside any error.
func RunLogsTest() (*test.SuiteResult, error) { return RunSuiteTest("logs-test") }

// RunParserTest runs the parser test suite and returns its results alongside any error.
func RunParserTest() (*test.SuiteResult, error) { return RunSuiteTest("parser-test") }

// RunAllTests runs every registered test suite and returns the aggregated results.
func RunAllTests() (*test.SuiteResult, error) { return RunSuiteTest("run-all-tests") }

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
