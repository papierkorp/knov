package githistorytest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/pathutils"
	"knov/internal/test"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// caseGitRemotePushPullTestAuth points the app's git remote at a throwaway local bare repo
// (file:// transport, no network involved) and exercises EnsureRemote/TestAuth/Push/
// PullRebase against it, then always restores whatever remote was configured before the
// case ran. Uses SetGitRemoteForTest rather than UpdateEnvFile since this only needs to
// take effect in-process, not survive a restart; the case works with the currently
// configured branch since KNOV_GIT_REMOTE_BRANCH isn't swapped.
func caseGitRemotePushPullTestAuth(_ *sampleState) test.CaseResult {
	name := "git-remote-push-pull-test-auth"

	bareDir, branch, cleanup, err := useBareRemote()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	git.Push() // fire-and-forget; poll the bare remote below instead of assuming completion

	pushed := waitForBranch(bareDir, branch, 15*time.Second)

	// test-auth (ls-remote) only after the push - go-git errors on ls-remote against a
	// truly empty bare repo (zero refs), which a fresh PlainInit(bare) always is
	authResult, err := git.TestAuth()
	if err != nil {
		return errCase(name, fmt.Errorf("test-auth against local bare remote failed: %w", err))
	}

	pullErr := git.PullRebase()

	success := pushed && pullErr == nil
	cr := test.CaseResult{
		Name:     name,
		Expected: "test-auth connects, push lands the current branch on the bare remote, pull is a no-op (already up to date)",
		Actual:   fmt.Sprintf("test-auth=%q pushed=%v pullErr=%v", authResult, pushed, pullErr),
		Success:  success,
	}
	if !success {
		cr.Error = "push/pull/test-auth against local bare remote did not behave as expected"
	}
	return cr
}

// useBareRemote points the app's git remote at a throwaway local bare repo (file:// transport) and
// makes sure refs/heads/<configured branch> exists locally, since git.Push/PullRebase always use that
// ref, which only exists if the repo's real branch happens to match it (KNOV_GIT_REMOTE_BRANCH
// isn't live-editable) - cleanup restores the remote and removes a ref it added.
func useBareRemote() (bareDir, branch string, cleanup func(), err error) {
	bareDir, err = os.MkdirTemp("", "knov-searchtest-remote-*")
	if err != nil {
		return "", "", nil, err
	}
	if _, err := gogit.PlainInit(bareDir, true); err != nil {
		os.RemoveAll(bareDir)
		return "", "", nil, err
	}

	origRemote := configmanager.GetGitRemote()
	cleanups := []func(){func() { os.RemoveAll(bareDir) }, func() {
		configmanager.SetGitRemoteForTest(origRemote)
		if origRemote == "" {
			removeOriginRemote()
		} else {
			_ = git.EnsureRemote()
		}
	}}
	cleanup = func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	configmanager.SetGitRemoteForTest(bareDirFileURL(bareDir))
	if err := git.EnsureRemote(); err != nil {
		cleanup()
		return "", "", nil, err
	}

	branch = configmanager.GetGitRemoteBranch()
	createdRef, err := ensureLocalBranchRef(branch)
	if err != nil {
		cleanup()
		return "", "", nil, err
	}
	if createdRef {
		cleanups = append(cleanups, func() { removeLocalBranchRef(branch) })
	}
	return bareDir, branch, cleanup, nil
}

// bareDirFileURL builds a file:// URL for a local bare repo path. On Windows the path
// starts with a drive letter (C:\...) which needs a third slash and forward slashes,
// otherwise the drive letter's colon is parsed as a port.
func bareDirFileURL(dir string) string {
	slashDir := pathutils.ToSlash(dir)
	if filepath.VolumeName(dir) != "" {
		return "file:///" + slashDir
	}
	return "file://" + slashDir
}

// removeOriginRemote drops the "origin" remote entirely, used when the case's defer
// restores a repo that had no remote configured before the case ran.
func removeOriginRemote() {
	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := gogit.PlainOpen(dataDir)
	if err != nil {
		return
	}
	_ = repo.DeleteRemote("origin")
}

// ensureLocalBranchRef makes sure refs/heads/<branch> exists in the local repo, pointing
// at the current HEAD commit if it doesn't already exist. Returns whether it created the
// ref (so the caller only cleans up refs it added, not a real pre-existing branch).
func ensureLocalBranchRef(branch string) (bool, error) {
	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := gogit.PlainOpen(dataDir)
	if err != nil {
		return false, err
	}

	refName := plumbing.NewBranchReferenceName(branch)
	if _, err := repo.Reference(refName, true); err == nil {
		return false, nil // already exists
	}

	head, err := repo.Head()
	if err != nil {
		return false, err
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, head.Hash())); err != nil {
		return false, err
	}
	return true, nil
}

// removeLocalBranchRef deletes the temporary ref created by ensureLocalBranchRef.
func removeLocalBranchRef(branch string) {
	dataDir := configmanager.GetAppConfig().DataPath
	repo, err := gogit.PlainOpen(dataDir)
	if err != nil {
		return
	}
	_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch))
}

// waitForBranch polls the bare repo at bareDir until branch appears or timeout elapses.
func waitForBranch(bareDir, branch string, timeout time.Duration) bool {
	ref := plumbing.NewBranchReferenceName(branch)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		repo, err := gogit.PlainOpen(bareDir)
		if err == nil {
			if _, err := repo.Reference(ref, true); err == nil {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
