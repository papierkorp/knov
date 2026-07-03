// Package githistorytest - Git repo/file history suite: seeds real files, metadata and a
// sequence of git commits, then exercises internal/git's history/diff/restore/remote
// functions directly against them.
package githistorytest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/pathutils"
	"knov/internal/test"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and recommitted
// at the start of each run so cases never see stale state from a previous run. Nested under
// "test/" so the admin "Clean Test Data" button (which wipes docs/test) removes it too.
const testDir = "test/git-history-tests"

// collection is derived from a file's top-level folder (files.CollectionFromPath) - every
// sample file here lives under "test/", so they all share the "test" collection. The
// collection-filter case checks that collection alone (no other suite's folder is a plausible
// false positive, since a bogus collection name matches nothing).
const testCollection = "test"

const (
	gammaFile = "gamma-versioned.md"
	etaFile   = "EtaDeletedMarker.md"
)

const (
	gammaContentV1 = "gamma version one content"
	gammaContentV2 = "gamma version two content, changed"
)

// sampleState records the commit hashes produced while seeding, so history/diff/restore
// cases can reference specific versions without re-deriving them from git log.
type sampleState struct {
	gammaCommit1 string // commit that added gamma with v1 content
	gammaCommit2 string // commit that changed gamma to v2 content
}

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

// saveMetadata saves metadata for relPath. Collection is not settable here - MetaDataSave
// always derives it from the file's top-level folder (files.CollectionFromPath).
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

// resetAndSeed wipes the sample folder, then recreates it with a sequence of commits:
//  1. gamma-v1 and eta added and committed (gammaCommit1)
//  2. gamma edited to v2 and committed (gammaCommit2) -> gamma has 2 versions to diff/restore
//  3. eta deleted and committed (most recent) -> latest-changes pagination ordering
func resetAndSeed() (*sampleState, error) {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return nil, err
	}

	if err := writeFile(testPath(gammaFile), gammaContentV1); err != nil {
		return nil, err
	}
	if err := saveMetadata(testPath(gammaFile)); err != nil {
		return nil, err
	}

	if err := writeFile(testPath(etaFile), "# eta\n"); err != nil {
		return nil, err
	}
	if err := saveMetadata(testPath(etaFile)); err != nil {
		return nil, err
	}

	if err := commitAll("githistorytest: seed gamma-v1, eta"); err != nil {
		return nil, err
	}
	state := &sampleState{}
	commit1, err := git.GetCurrentCommit()
	if err != nil {
		return nil, err
	}
	state.gammaCommit1 = commit1

	if err := writeFile(testPath(gammaFile), gammaContentV2); err != nil {
		return nil, err
	}
	if err := commitAll("githistorytest: gamma v2"); err != nil {
		return nil, err
	}
	commit2, err := git.GetCurrentCommit()
	if err != nil {
		return nil, err
	}
	state.gammaCommit2 = commit2

	if err := os.Remove(pathutils.ToDocsPath(testPath(etaFile))); err != nil {
		return nil, err
	}
	if err := commitAll("githistorytest: delete eta"); err != nil {
		return nil, err
	}

	if err := files.SaveAllCollectionsToSystemData(); err != nil {
		return nil, err
	}

	return state, nil
}

func readFile(relPath string) (string, error) {
	full := pathutils.ToDocsPath(relPath)
	b, err := contentStorage.ReadFile(full)
	return string(b), err
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
