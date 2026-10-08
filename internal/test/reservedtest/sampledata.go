// Package reservedtest - sample files: one docs file per reserved folder and the file its path
// would read as without the docs/ prefix (its collision partner)
package reservedtest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// sub is the folder inside each reserved folder (and of the collision partners)
const sub = "knov-test"

// reserved are the top-level docs folder names pathutils reads as a path prefix
var reserved = []string{"docs", "media", "files"}

// doc is the docs-relative path of the sample doc in the reserved folder top
func doc(top, name string) string {
	return top + "/" + sub + "/" + name
}

// partner is the metadata path the sample doc in top would read as without the docs/ prefix:
// a media file for media/, the top-level docs file for docs/ and files/
func partner(top, name string) string {
	if top == "media" {
		return "media/" + sub + "/" + name
	}
	return "docs/" + sub + "/" + name
}

// marker is unique text in the sample doc of top (partnerMarker in its collision partner)
func marker(top string) string        { return "reserved-marker-" + top }
func partnerMarker(top string) string { return "partner-marker-" + top }

// fullPath is the real file of a metadata path ("docs/..." or "media/...")
func fullPath(p string) string {
	if rel, ok := strings.CutPrefix(p, "media/"); ok {
		return filepath.Join(pathutils.MediaRoot(), filepath.FromSlash(rel))
	}
	return filepath.Join(pathutils.DocsRoot(), filepath.FromSlash(strings.TrimPrefix(p, "docs/")))
}

func write(full, content string) error {
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

// wipe removes every sample folder.
func wipe() error {
	dirs := []string{filepath.Join(pathutils.DocsRoot(), sub), filepath.Join(pathutils.MediaRoot(), sub)}
	for _, top := range reserved {
		dirs = append(dirs, filepath.Join(pathutils.DocsRoot(), top, sub))
	}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	return nil
}

// resetAndSeed writes, like a git sync, the doc synced.md into every reserved folder and its
// collision partner, then syncs their metadata like the app does after a pull.
func resetAndSeed() error {
	if err := wipe(); err != nil {
		return err
	}
	for _, top := range reserved {
		if err := write(fullPath("docs/"+doc(top, "synced.md")), "# synced\n\n"+marker(top)+"\n"); err != nil {
			return err
		}
		if err := write(fullPath(partner(top, "synced.md")), "# partner\n\n"+partnerMarker(top)+"\n"); err != nil {
			return err
		}
	}
	if err := job.RunFullRebuild(); err != nil {
		return fmt.Errorf("rebuild metadata: %w", err)
	}
	files.RefreshCaches()
	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
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
