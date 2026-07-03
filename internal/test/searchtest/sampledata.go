// Package searchtest - Search suite: seeds real files, metadata and git commits, then
// exercises search.SearchFiles*/SearchDeletedFiles* directly against them.
package searchtest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/search"
	"knov/internal/test"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testDir is the docs-relative fixture folder every case seeds into, wiped and recommitted
// at the start of each run so cases never see stale state from a previous run.
const testDir = "test/search-tests"

const (
	alphaFile = "AlphaUniqueTitle.md"
	betaFile  = "beta-content.md"
	deltaFile = "DeltaDeletedUniqueMarker.md"
)

const (
	betaContentMarker  = "BetaUniqueContentPhrase"
	deltaContentMarker = "DeltaContentMarker"
)

func testPath(name string) string {
	return filepath.Join(testDir, name)
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

func saveMetadata(relPath string) error {
	return files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(relPath),
		Editor: files.EditorTypeToastUI,
	})
}

// commitAll stages every pending change in the data repo and commits it, mirroring
// internal/test.commitGitChanges (unexported there, so this is its own copy).
func commitAll(message string) error {
	if err := configmanager.InitGitRepository(); err != nil {
		return fmt.Errorf("failed to init git repository: %w", err)
	}

	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := gogit.PlainOpen(dataDir)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := worktree.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		return fmt.Errorf("failed to stage files for commit %q: %w", message, err)
	}

	if _, err := worktree.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "knov",
			Email: "knov@localhost",
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	}); err != nil {
		return fmt.Errorf("failed to commit %q: %w", message, err)
	}

	return nil
}

// resetAndSeed wipes the fixture folder, then recreates it with alpha (title search), beta
// (full-content search) and delta (added then deleted, for deleted-file search).
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := writeFile(testPath(alphaFile), "# alpha\ngeneric content\n"); err != nil {
		return err
	}
	if err := saveMetadata(testPath(alphaFile)); err != nil {
		return err
	}

	if err := writeFile(testPath(betaFile), fmt.Sprintf("# beta\n%s\n", betaContentMarker)); err != nil {
		return err
	}
	if err := saveMetadata(testPath(betaFile)); err != nil {
		return err
	}

	if err := writeFile(testPath(deltaFile), fmt.Sprintf("# delta\n%s\n", deltaContentMarker)); err != nil {
		return err
	}
	if err := saveMetadata(testPath(deltaFile)); err != nil {
		return err
	}

	if err := commitAll("searchtest: seed alpha, beta, delta"); err != nil {
		return err
	}

	if err := os.Remove(pathutils.ToDocsPath(testPath(deltaFile))); err != nil {
		return err
	}
	if err := commitAll("searchtest: delete delta"); err != nil {
		return err
	}

	// full-content search reads from the search index, which is only ever populated by
	// the periodic reindex cronjob - index synchronously here so search cases see the
	// freshly-seeded fixture content immediately instead of racing that cronjob.
	if err := search.IndexAllFiles(); err != nil {
		return err
	}

	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
