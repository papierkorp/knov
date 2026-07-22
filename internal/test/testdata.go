// Package test - Test data setup and management
package test

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"

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

	deleteTestFilter()

	logging.LogInfo(logging.KeyApp, "test data cleaned")
	return nil
}

func setupTestMetadata() error {
	logging.LogInfo(logging.KeyApp, "creating test metadata")

	for _, meta := range getCopiedFilesMetadata() {
		if err := files.MetaDataSave(meta); err != nil {
			logging.LogError(logging.KeyApp, "failed to save metadata for %s: %v", meta.Path, err)
		}
	}

	if err := createAutoMetadata(); err != nil {
		return fmt.Errorf("failed to create auto metadata: %w", err)
	}

	createTestFilter()

	return files.MetaDataLinksRebuild(logging.KeyApp)
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

func deleteTestFilter() {
	if err := filter.DeleteFilterConfig("test/example_filter"); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete test filter: %v", err)
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

	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := git.PlainOpen(dataDir)
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
