package githistorytest

import (
	"fmt"
	"os"
	"time"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/test"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// caseGitRemotePushPullTestAuth points the app's git remote at a throwaway local bare repo
// (file:// transport, no network involved) and exercises EnsureRemote/TestAuth/Push/
// PullRebase against it, then always restores whatever remote was configured before the
// case ran. KNOV_GIT_REMOTE is the only remote setting UpdateEnvFile applies live (branch/
// autopush changes need a restart), so the case works with the currently configured branch.
func caseGitRemotePushPullTestAuth(_ *fixtureState) test.CaseResult {
	name := "git-remote-push-pull-test-auth"

	bareDir, err := os.MkdirTemp("", "knov-searchtest-remote-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(bareDir)

	if _, err := gogit.PlainInit(bareDir, true); err != nil {
		return errCase(name, err)
	}

	origRemote := configmanager.GetGitRemote()
	defer func() {
		_ = configmanager.UpdateEnvFile("KNOV_GIT_REMOTE", origRemote)
		if origRemote == "" {
			removeOriginRemote()
		} else {
			_ = git.EnsureRemote()
		}
	}()

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REMOTE", "file://"+bareDir); err != nil {
		return errCase(name, err)
	}
	if err := git.EnsureRemote(); err != nil {
		return errCase(name, err)
	}

	// git.Push/PullRebase always push/pull refs/heads/<configured branch>, which only
	// exists locally if the repo's actual branch happens to match it (KNOV_GIT_REMOTE_BRANCH
	// isn't a live-editable setting, so it can't be pointed at whatever branch this repo
	// really uses) - create a temporary local ref under that name pointing at HEAD so the
	// push has something to push, then remove it again if this case is the one that added it.
	branch := configmanager.GetGitRemoteBranch()
	createdRef, err := ensureLocalBranchRef(branch)
	if err != nil {
		return errCase(name, err)
	}
	if createdRef {
		defer removeLocalBranchRef(branch)
	}

	git.Push() // fire-and-forget; poll the bare remote below instead of assuming completion

	pushed := waitForBranch(bareDir, branch, 3*time.Second)

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
