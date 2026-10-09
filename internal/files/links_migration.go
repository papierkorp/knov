package files

import (
	"fmt"
	"os"

	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// RelativeLinkChange is a bare markdown or html link (parser.IsBareLink) whose target changed when
// bare links started to be read from their doc's folder instead of the docs root: Link is its
// path as written, OldTarget and NewTarget the metadata paths it pointed at before and now,
// OldTargetExists whether the old target is an existing file or folder.
type RelativeLinkChange struct {
	SourceFile      string `json:"sourceFile"`
	Link            string `json:"link"`
	OldTarget       string `json:"oldTarget"`
	NewTarget       string `json:"newTarget"`
	OldTargetExists bool   `json:"oldTargetExists"`
}

// ScanRelativeLinks returns every bare link in the docs whose target is another file read from
// its doc's folder than it was read from the docs root - once per doc and old target, without
// changing anything.
func ScanRelativeLinks() ([]RelativeLinkChange, error) {
	paths, err := contentStorage.ListFiles()
	if err != nil {
		return nil, err
	}
	changes := []RelativeLinkChange{} // an empty scan is [], not null
	for _, rel := range paths {
		if !parser.IsMarkdownExtension(rel) {
			continue
		}
		// docs/ prefixed, so a docs folder named "media", "docs" or "files" isn't stripped
		src := "docs/" + rel
		data, err := os.ReadFile(pathutils.ToDocsPath(src))
		if err != nil {
			logging.LogWarning(logging.KeyRepairLinks, "failed to read %s: %v", src, err)
			continue
		}
		seen := map[string]bool{}
		parser.RewriteLinks(string(data), func(l parser.Link) (string, bool) {
			if !parser.IsBareLink(l) {
				return "", false
			}
			old, cur := parser.DocsRootLinkTarget(src, l), parser.LinkTarget(src, l)
			if old != cur && !seen[old.String()] {
				seen[old.String()] = true
				changes = append(changes, RelativeLinkChange{SourceFile: src, Link: l.Path, OldTarget: old.String(), NewTarget: cur.String(), OldTargetExists: fileExists(pathutils.ToFullPath(old.String()))})
			}
			return "", false
		})
	}
	return changes, nil
}

// MigrateRelativeLinks rewrites the bare links of sourceFile that pointed at oldTarget while bare
// links were read from the docs root to their docs-root form ("/a.md", "/media/x.png", a /files/
// or /media/ url in html), so they keep pointing at it, and resyncs its link metadata. Returns
// false (with no error) if no such link was found.
func MigrateRelativeLinks(sourceFile, oldTarget string) (bool, error) {
	fullPath := pathutils.ToFullPath(sourceFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", sourceFile, err)
	}
	content, changed := parser.RewriteLinks(string(data), func(l parser.Link) (string, bool) {
		if !parser.IsBareLink(l) || parser.DocsRootLinkTarget(sourceFile, l).String() != oldTarget {
			return "", false
		}
		// written like a "/" link, which reads from the docs root
		l.Path = "/" + l.Path
		return rebuildLinkTarget(sourceFile, l, oldTarget), true
	})
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", sourceFile, err)
	}
	if err := UpdateLinksForSingleFile(sourceFile); err != nil {
		logging.LogWarning(logging.KeyRepairLinks, "failed to rebuild links for %s: %v", sourceFile, err)
	}
	return true, nil
}
