// Package test - Test data setup and management
package test

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	knovgit "knov/internal/git"
	"knov/internal/logging"
	"knov/internal/tracker"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var docsFS embed.FS

// SetDocsFiles sets the embedded docs filesystem
func SetDocsFiles(fs embed.FS) {
	docsFS = fs
}

// SetupTestData creates test files, git operations and metadata
func SetupTestData() error {
	testDir := filepath.Join(contentStorage.GetDocsPath(), "test")
	if err := os.RemoveAll(testDir); err != nil {
		return fmt.Errorf("failed to remove test directory: %w", err)
	}

	if err := copyTestFiles(); err != nil {
		return fmt.Errorf("failed to copy test files: %w", err)
	}

	if err := createGitOperations("initial test documentation"); err != nil {
		return fmt.Errorf("failed to create git operations: %w", err)
	}

	if err := setupTestMetadata(); err != nil {
		return fmt.Errorf("failed to setup test metadata: %w", err)
	}

	if err := simulateFileChange(); err != nil {
		return fmt.Errorf("failed to simulate file changes: %w", err)
	}

	logging.LogInfo(logging.KeyApp, "test data setup completed")
	return nil
}

// CleanTestData removes the test data folder and associated config entries
func CleanTestData() error {
	testDir := filepath.Join(contentStorage.GetDocsPath(), "test")
	if err := os.RemoveAll(testDir); err != nil {
		return fmt.Errorf("failed to remove test directory: %w", err)
	}

	// media suites (mediatest, jobstest) mirror their docs test folder under media/test -
	// wipe that too, since it's never touched by the docs-side removal above.
	mediaTestDir := filepath.Join(contentStorage.GetMediaPath(), "test")
	if err := os.RemoveAll(mediaTestDir); err != nil {
		return fmt.Errorf("failed to remove media test directory: %w", err)
	}

	deleteTestFilter()
	deleteTestTracker()

	logging.LogInfo(logging.KeyApp, "test data cleaned")
	return nil
}

func setupTestMetadata() error {
	logging.LogInfo(logging.KeyApp, "creating test metadata")

	for _, meta := range getCopiedFilesMetadata() {
		if err := SeedMetadata(meta); err != nil {
			logging.LogError(logging.KeyApp, "failed to save metadata for %s: %v", meta.Path, err)
		}
	}

	if err := createAutoMetadata(); err != nil {
		return fmt.Errorf("failed to create auto metadata: %w", err)
	}

	createTestFilter()
	createTestTracker()

	return files.MetaDataLinksRebuild(context.Background(), logging.KeyApp, nil)
}

func createTestFilter() {
	cfg := &filter.Config{
		Criteria: []filter.Criteria{{
			Metadata: "tags",
			Operator: "contains",
			Value:    "test-files",
			Action:   "include",
		}},
		Logic: "and",
		Limit: 50,
	}
	if err := filter.SaveFilterConfig(cfg, "test/example_filter"); err != nil {
		logging.LogError(logging.KeyApp, "failed to create test filter: %v", err)
	}
}

// createTestTracker seeds an example tracker under test/ with a couple of counters
// backdated across three months, so the editor and its generated stats file have
// something to show - including the trend sparkline, which needs more than one
// month of data.
func createTestTracker() {
	id := "test/example_tracker"
	if err := tracker.SetMeta(id, "Example Tracker", []tracker.CounterInput{
		{Title: "coffees", Columns: tracker.AllColumns},
		{Title: "pushups", Columns: tracker.AllColumns},
	}); err != nil {
		logging.LogError(logging.KeyApp, "failed to create test tracker: %v", err)
		return
	}

	config, err := tracker.GetConfig(id)
	if err != nil || config == nil {
		logging.LogError(logging.KeyApp, "failed to load test tracker after create: %v", err)
		return
	}

	now := time.Now()
	monthlyDeltas := []int{3, 7, 5} // 2 months ago, last month, this month
	for _, c := range config.Counters {
		for i, delta := range monthlyDeltas {
			monthsAgo := len(monthlyDeltas) - 1 - i
			day := now.AddDate(0, -monthsAgo, 0)
			if _, err := tracker.TickDay(id, c.ID, day, delta); err != nil {
				logging.LogError(logging.KeyApp, "failed to seed test tracker counter %s: %v", c.Title, err)
			}
		}
	}
}

// deleteTestTracker removes every tracker config under the test/ prefix - the
// example_tracker seeded by createTestTracker plus any left behind by the test
// suites, so cleaning test data leaves nothing test-related.
func deleteTestTracker() {
	ids, err := tracker.GetAllTrackers()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list trackers for cleanup: %v", err)
		return
	}
	for _, id := range ids {
		if !strings.HasPrefix(id, "test/") {
			continue
		}
		if err := tracker.DeleteConfig(id); err != nil {
			logging.LogError(logging.KeyApp, "failed to delete test tracker %s: %v", id, err)
		}
	}
}

// deleteTestFilter removes every filter config under the test/ prefix - the example_filter
// seeded by createTestFilter plus any left behind by the test suites (e.g.
// test/editors-tests/edtest-filter), so cleaning test data leaves nothing test-related.
func deleteTestFilter() {
	ids, err := filter.GetAllFilters()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list filters for cleanup: %v", err)
		return
	}
	for _, id := range ids {
		if !strings.HasPrefix(id, "test/") {
			continue
		}
		if err := filter.DeleteFilterConfig(id); err != nil {
			logging.LogError(logging.KeyApp, "failed to delete test filter %s: %v", id, err)
		}
	}
}

func createGitOperations(commitMessage string) error {
	if err := commitGitChanges(commitMessage); err != nil {
		return err
	}

	if err := createTestStructure(); err != nil {
		return fmt.Errorf("failed to create test structure: %w", err)
	}

	if err := commitGitChanges("add test structure"); err != nil {
		return err
	}

	return nil
}

func commitGitChanges(commitMessage string) error {
	if err := configmanager.InitGitRepository(); err != nil {
		return fmt.Errorf("failed to init git repository: %w", err)
	}

	repo, err := knovgit.OpenRepository()
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if _, err = worktree.Add("."); err != nil {
		logging.LogError(logging.KeyApp, "failed to stage files for commit %q: %v", commitMessage, err)
	}

	if _, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	}); err != nil {
		logging.LogError(logging.KeyApp, "failed to commit %q: %v", commitMessage, err)
	}

	return nil
}
