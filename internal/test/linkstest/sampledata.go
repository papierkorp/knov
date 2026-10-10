// Package linkstest - sample files and the shared link checks
package linkstest

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
	"knov/internal/test/specialchars"
)

// testDir is the docs-relative sample folder (and its media mirror), wiped and reseeded at the
// start of each run. Every corpus name gets its own folder, so a leading space stays leading.
const testDir = "test/links-tests"

// targetsFolder is the folder name every link target lives under - the filter index case
// selects them by it
const targetsFolder = "links-targets"

// pngMagic is enough of a PNG signature for content sniffing to detect image/png.
var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// names are the corpus names that can be real files on this host.
var names = slices.DeleteFunc(slices.Clone(specialchars.Names), func(n string) bool {
	return !specialchars.ValidOn(runtime.GOOS, n)
})

// target is the docs-relative path of the i-th seeded doc, mediaTarget the media-relative path
// of its seeded media file.
func target(i int) string {
	return fmt.Sprintf("%s/%s/c%02d/%s", testDir, targetsFolder, i, names[i])
}

func mediaTarget(i int) string {
	return fmt.Sprintf("%s/c%02d/%s", testDir, i, imgName(names[i]))
}

func imgName(name string) string {
	return strings.Replace(name, ".md", ".png", 1)
}

// marker is unique text in the i-th seeded doc, to find it in a composed book.
func marker(i int) string {
	return fmt.Sprintf("links-marker-c%02d", i)
}

func writeDoc(rel string, content []byte) error {
	full := pathutils.ToDocsPath(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, content, 0644)
}

// saveDoc writes a doc and syncs it like the editor save handlers do.
func saveDoc(rel, content string) error {
	if err := writeDoc(rel, []byte(content)); err != nil {
		return err
	}
	if err := files.MetaDataSync(pathutils.GuessMeta(rel)); err != nil {
		return err
	}
	return files.UpdateLinksForSingleFile(pathutils.GuessMeta(rel))
}

// resetAndSeed wipes the sample folders and writes one doc and one media file per corpus name.
func resetAndSeed() error {
	for _, dir := range []string{pathutils.ToDocsPath(testDir), pathutils.ToMediaPath(testDir)} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	for i := range names {
		if err := saveDoc(target(i), "# target\n\n"+marker(i)+"\n\n## My Section\n"); err != nil {
			return err
		}
		full := pathutils.ToMediaPath(mediaTarget(i))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
		if err := contentStorage.WriteFile(full, pngMagic, 0644); err != nil {
			return err
		}
		if err := files.MetaDataSync(pathutils.MediaPath(mediaTarget(i))); err != nil {
			return err
		}
	}
	files.RefreshCaches()
	return nil
}

// metadataGaps checks that link metadata of the doc src (docs-relative) resolves to every
// wanted target (a metadata path, docs/ or media/ prefixed): the target exists, it's in src's
// used links and src is in a doc target's linked from. Returns one line per gap.
func metadataGaps(src string, want []string) []string {
	srcKey := pathutils.GuessMeta(src)
	meta, err := files.MetaDataGet(srcKey)
	if err != nil || meta == nil {
		return []string{fmt.Sprintf("%s: no metadata (%v)", src, err)}
	}
	used := pathutils.Strings(meta.UsedLinks)
	var gaps []string
	for _, w := range want {
		if _, err := os.Stat(pathutils.ToFullPath(w)); err != nil {
			gaps = append(gaps, fmt.Sprintf("%q: file missing", w))
		}
		if !slices.Contains(used, w) {
			gaps = append(gaps, fmt.Sprintf("%q: not in used links", w))
		}
		if strings.HasPrefix(w, "docs/") {
			if m, _ := files.MetaDataGet(pathutils.GuessMeta(w)); m == nil || !slices.Contains(m.LinksToHere, srcKey) {
				gaps = append(gaps, fmt.Sprintf("%q: src not in its linked from", w))
			}
		}
	}
	if extra := slices.DeleteFunc(used, func(u string) bool { return slices.Contains(want, u) }); len(gaps) > 0 && len(extra) > 0 {
		gaps = append(gaps, fmt.Sprintf("used links read instead: %q", extra))
	}
	return gaps
}

// renderGaps checks that the rendered src links to every wanted target, see renderedTargets.
func renderGaps(src string, want []string) []string {
	fc, err := files.GetFileContent(pathutils.ToDocsPath(src))
	if err != nil {
		return []string{fmt.Sprintf("%s: render failed (%v)", src, err)}
	}
	rendered := renderedTargets(fc.HTML)
	var gaps []string
	for _, w := range want {
		if !slices.Contains(rendered, w) {
			gaps = append(gaps, fmt.Sprintf("%q: not linked in the rendered page", w))
		}
	}
	if extra := slices.DeleteFunc(rendered, func(r string) bool { return slices.Contains(want, r) }); len(gaps) > 0 && len(extra) > 0 {
		gaps = append(gaps, fmt.Sprintf("rendered links instead: %q", extra))
	}
	return gaps
}

func linkGaps(src string, want []string) []string {
	return append(metadataGaps(src, want), renderGaps(src, want)...)
}

var (
	renderedHrefRe    = regexp.MustCompile(`<(?:a href|img src)="([^"]*)"`)
	renderedPreviewRe = regexp.MustCompile(`/api/media/preview\?path=([^&"]*)`)
)

// renderedTargets returns the metadata path of every link, image and media preview in rendered
// html, as the browser requests it.
func renderedTargets(out string) []string {
	var targets []string
	for _, m := range renderedPreviewRe.FindAllStringSubmatch(out, -1) {
		p, _ := url.QueryUnescape(m[1])
		targets = append(targets, "media/"+p)
	}
	for _, m := range renderedHrefRe.FindAllStringSubmatch(out, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		if rel, ok := strings.CutPrefix(u.Path, "/media/"); ok {
			targets = append(targets, "media/"+rel)
		} else if rel := pathutils.FileFromURL(u.String()); rel != "" {
			targets = append(targets, rel.String())
		}
	}
	return targets
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

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
