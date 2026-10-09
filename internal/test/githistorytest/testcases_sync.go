package githistorytest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/job"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/test"
	"knov/internal/test/specialchars"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// caseGitSyncWrittenFiles pushes the app's repo to a throwaway bare remote, lets a second clone
// commit a doc per special-char name plus a doc linking to all of them (files the app never
// wrote itself), pulls them into the app and checks that the link metadata of the linking doc
// points at every pulled file.
func caseGitSyncWrittenFiles(_ *sampleState) test.CaseResult {
	name := "git-sync-written-files"
	syncDir := testDir + "/sync"

	bareDir, branch, cleanup, err := useBareRemote()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()
	defer os.RemoveAll(pathutils.ToDocsPath(pathutils.DocsPath(syncDir).String()))

	git.Push()
	if !waitForBranch(bareDir, branch, 15*time.Second) {
		return errCase(name, fmt.Errorf("push did not land on the bare remote"))
	}

	cloneDir, err := os.MkdirTemp("", "knov-sync-clone-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(cloneDir)
	repo, err := gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{URL: bareDirFileURL(bareDir), ReferenceName: plumbing.NewBranchReferenceName(branch)})
	if err != nil {
		return errCase(name, err)
	}

	var names, targets []string
	linker := "# linker\n\n"
	for _, n := range specialchars.Names {
		if !specialchars.ValidOn(runtime.GOOS, n) {
			continue
		}
		rel := syncDir + "/" + n
		if err := writeSynced(cloneDir, rel, "# synced\n"); err != nil {
			return errCase(name, err)
		}
		link := parser.Link{Kind: parser.LinkMarkdown, Path: "/" + rel}
		linker += "- [x](" + link.Dest() + ")\n"
		names = append(names, rel)
		targets = append(targets, pathutils.DocsPath(rel).String())
	}
	linkerRel := syncDir + "/linker.md"
	if err := writeSynced(cloneDir, linkerRel, linker); err != nil {
		return errCase(name, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errCase(name, err)
	}
	if _, err := wt.Add("docs"); err != nil {
		return errCase(name, err)
	}
	if _, err := wt.Commit("sync test files", &gogit.CommitOptions{Author: &object.Signature{Name: "knov test", Email: "test@knov.invalid", When: time.Now()}}); err != nil {
		return errCase(name, err)
	}
	if err := repo.Push(&gogit.PushOptions{}); err != nil {
		return errCase(name, err)
	}

	if err := git.PullRebase(); err != nil {
		return errCase(name, fmt.Errorf("pull failed: %w", err))
	}

	var gaps []string
	for _, rel := range names {
		if _, err := os.Stat(pathutils.ToFullPath(pathutils.DocsPath(rel).String())); err != nil {
			gaps = append(gaps, "not pulled: "+rel)
		}
	}
	if err := job.RunFullRebuild(); err != nil {
		return errCase(name, fmt.Errorf("rebuild metadata: %w", err))
	}
	files.RefreshCaches()
	m, err := files.MetaDataGet(pathutils.DocsPath(linkerRel))
	if err != nil || m == nil {
		return errCase(name, fmt.Errorf("no metadata for the pulled linking doc (%v)", err))
	}
	for _, target := range targets {
		if !slices.Contains(m.UsedLinks, target) {
			gaps = append(gaps, "link metadata misses "+target)
		}
	}
	return gapsCase(name, "every pulled special-char file is a link target of the pulled linking doc", gaps)
}

// writeSynced writes the docs-relative file rel into the docs folder of the clone at dir.
func writeSynced(dir, rel, content string) error {
	full := filepath.Join(dir, "docs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0644)
}

// gapsCase builds the case result: success without gaps, otherwise every gap on its own line.
func gapsCase(name, expected string, gaps []string) test.CaseResult {
	cr := test.CaseResult{Name: name, Expected: expected, Success: len(gaps) == 0}
	cr.Actual = fmt.Sprintf("%d gaps", len(gaps))
	if !cr.Success {
		cr.Error = "\n    " + strings.Join(gaps, "\n    ")
	}
	return cr
}
