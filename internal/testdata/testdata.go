// Package testdata - Test data setup and management
package testdata

import (
	"embed"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var testFilesFS embed.FS

// SetTestFiles sets the embedded test files filesystem
func SetTestFiles(filesFS embed.FS) {
	testFilesFS = filesFS
}

// SetupTestData creates test files, git operations and metadata
func SetupTestData() error {
	testDir := filepath.Join(contentStorage.GetDocsPath(), "test")
	if err := os.RemoveAll(testDir); err != nil {
		logging.LogError("failed to remove test directory: %v", err)
		return err
	}

	if err := copyTestFiles(); err != nil {
		return err
	}

	if err := createGitOperations("initial test documentation"); err != nil {
		return err
	}

	if err := setupTestMetadata(); err != nil {
		return err
	}

	if err := simulateFileChange(); err != nil {
		return err
	}

	logging.LogInfo("test data setup completed")
	return nil
}

// CleanTestData removes only the test data folder
func CleanTestData() error {
	testDir := filepath.Join(contentStorage.GetDocsPath(), "test")
	if err := os.RemoveAll(testDir); err != nil {
		logging.LogError("failed to remove test directory: %v", err)
		return err
	}

	logging.LogInfo("test data cleaned - removed test directory: %s", testDir)
	return nil
}

func setupTestMetadata() error {
	logging.LogInfo("creating test metadata")

	for _, meta := range getCopiedFilesMetadata() {
		if err := files.MetaDataSave(meta); err != nil {
			logging.LogError("failed to save metadata for %s: %v", meta.Path, err)
		}
	}

	if err := createAutoMetadata(); err != nil {
		return err
	}

	return files.MetaDataLinksRebuild()
}

func createGitOperations(commitMessage string) error {
	logging.LogInfo("creating git operations")

	if err := commitGitChanges(commitMessage); err != nil {
		return err
	}

	if err := createTestStructure(); err != nil {
		return err
	}

	if err := commitGitChanges("add test structure"); err != nil {
		return err
	}

	return nil
}

func commitGitChanges(commitMessage string) error {
	if err := configmanager.InitGitRepository(); err != nil {
		return err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := git.PlainOpen(dataDir)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	if _, err = worktree.Add("."); err != nil {
		logging.LogError("failed to add files: %v", err)
	}

	if _, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	}); err != nil {
		logging.LogError("failed to commit: %v", err)
	}

	return nil
}
