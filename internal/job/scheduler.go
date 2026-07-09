package job

import (
	"fmt"
	"sync"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/test"
)

var (
	stopChan                chan bool
	fileInterval            time.Duration
	searchInterval          time.Duration
	metadataRebuildInterval time.Duration

	fileMu           sync.Mutex
	searchMu         sync.Mutex
	rebuildMu        sync.Mutex
	filterMu         sync.Mutex
	notifMu          sync.Mutex
	cacheInvalidMu   sync.Mutex
	mediaCleanupMu   sync.Mutex
	gitPullMu        sync.Mutex
	gitPushMu        sync.Mutex
	testdataSetupMu  sync.Mutex
	testdataCleanMu  sync.Mutex
	filterTestMu     sync.Mutex
	editorsTestMu    sync.Mutex
	searchTestMu     sync.Mutex
	gitHistoryTestMu sync.Mutex
	chatTestMu       sync.Mutex
	dashboardTestMu  sync.Mutex
	kanbanTestMu     sync.Mutex
	runAllTestsMu    sync.Mutex
	runMu            sync.Mutex // prevents concurrent manual Run() calls
)

// execute runs job under mu, recording start/finish in job history.
// Returns ErrAlreadyRunning if the job is already active, or the job's own error.
func execute(mu *sync.Mutex, job Job) error {
	if !mu.TryLock() {
		logging.LogDebug("%s job already running, skipping", job.Name())
		return fmt.Errorf("%s: %w", job.Name(), ErrAlreadyRunning)
	}
	slot := recordStart(job.Name())
	defer mu.Unlock()
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
	recordFinish(slot, JobStatusOK, msg, output)
	return nil
}

// Start begins the cronjob scheduler.
func Start() {
	stopChan = make(chan bool)

	fileIntervalStr := configmanager.GetAppConfig().CronjobInterval
	parsedFileInterval, err := time.ParseDuration(fileIntervalStr)
	if err != nil {
		logging.LogWarning("invalid cronjob interval '%s', using default 5m", fileIntervalStr)
		parsedFileInterval = 5 * time.Minute
	}
	fileInterval = parsedFileInterval

	searchIntervalStr := configmanager.GetAppConfig().SearchIndexInterval
	parsedSearchInterval, err := time.ParseDuration(searchIntervalStr)
	if err != nil {
		logging.LogWarning("invalid search index interval '%s', using default 15m", searchIntervalStr)
		parsedSearchInterval = 15 * time.Minute
	}
	searchInterval = parsedSearchInterval

	metadataRebuildIntervalStr := configmanager.GetAppConfig().MetadataRebuildInterval
	parsedMetadataRebuildInterval, err := time.ParseDuration(metadataRebuildIntervalStr)
	if err != nil {
		logging.LogWarning("invalid metadata rebuild interval '%s', using default 30m", metadataRebuildIntervalStr)
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
				logging.LogInfo("file cronjob stopped")
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
				logging.LogInfo("search cronjob stopped")
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
				logging.LogInfo("metadata rebuild cronjob stopped")
				return
			}
		}
	}()

	logging.LogInfo("cronjob scheduler started (file: %v, search: %v, metadata rebuild: %v)", fileInterval, searchInterval, metadataRebuildInterval)
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

// RunGitPull runs a git pull --rebase with dedup protection.
func RunGitPull() error {
	return execute(&gitPullMu, &gitPullJob{})
}

// RunGitPush runs a git push with dedup protection.
func RunGitPush() error {
	return execute(&gitPushMu, &gitPushJob{})
}

// RunTestdataSetup sets up test data with dedup protection.
func RunTestdataSetup() error {
	return execute(&testdataSetupMu, &testdataSetupJob{})
}

// RunTestdataClean cleans test data with dedup protection.
func RunTestdataClean() error {
	return execute(&testdataCleanMu, &testdataCleanJob{})
}

// RunFilterTest runs the filter test suite and returns its results alongside any error.
func RunFilterTest() (*test.SuiteResult, error) {
	j := &filterTestJob{}
	if err := execute(&filterTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunEditorsTest runs the editors test suite and returns its results alongside any error.
func RunEditorsTest() (*test.SuiteResult, error) {
	j := &editorsTestJob{}
	if err := execute(&editorsTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunSearchTest runs the search test suite and returns its results alongside any error.
func RunSearchTest() (*test.SuiteResult, error) {
	j := &searchTestJob{}
	if err := execute(&searchTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunGitHistoryTest runs the git repo/file history test suite and returns its results alongside any error.
func RunGitHistoryTest() (*test.SuiteResult, error) {
	j := &gitHistoryTestJob{}
	if err := execute(&gitHistoryTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunChatTest runs the chat test suite and returns its results alongside any error.
func RunChatTest() (*test.SuiteResult, error) {
	j := &chatTestJob{}
	if err := execute(&chatTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunDashboardTest runs the dashboard test suite and returns its results alongside any error.
func RunDashboardTest() (*test.SuiteResult, error) {
	j := &dashboardTestJob{}
	if err := execute(&dashboardTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunKanbanTest runs the kanban test suite and returns its results alongside any error.
func RunKanbanTest() (*test.SuiteResult, error) {
	j := &kanbanTestJob{}
	if err := execute(&kanbanTestMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

// RunAllTests runs every registered test suite and returns the aggregated results.
func RunAllTests() (*test.SuiteResult, error) {
	j := &runAllTestsJob{}
	if err := execute(&runAllTestsMu, j); err != nil {
		return nil, err
	}
	return j.results, nil
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
		log := logging.LogBuilder("manual_cronjob")
		log.Println("manual run started")
		logging.LogInfo("manually triggering all jobs")
		RunFileSync() // includes filter-reindex as a sub-step
		RunSearchReindex()
		RunMetadataRebuild()
		RunNotificationPurge()
		logging.LogInfo("manual job execution completed")
		log.Println("manual run completed")
	}()
	return nil
}
