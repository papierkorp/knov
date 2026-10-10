// Package pathagreement holds the test that every consumer of a file's path agrees on which file
// a path names. docs-relative paths, metadata paths ("docs/..." / "media/...") and link paths
// are plain strings, and a docs file in a folder called docs, media or files reads as another
// file when a consumer guesses the kind from the prefix.
//
// A new consumer of paths (something that stores, writes, compares or resolves the path of a
// file) has to be registered in consumers below. A consumer returns the metadata path it ends
// up at for the file at loc, after the action the consumer is about - it has to be loc itself, or
// the moved location for a consumer that moves the file.
package pathagreement

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knov/internal/cacheStorage"
	"knov/internal/chatStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/searchStorage"
	"knov/internal/server/render"
)

// locations are the metadata paths a file can live at - the folder names that are also a prefix
// (docs, media, files) are the ones a guessing function gets wrong.
var locations = []string{
	"docs/x.md",
	"docs/docs/x.md",
	"docs/media/x.md",
	"docs/files/x.md",
	"docs/sub/x.md",
	"media/x.png",
	"docs/sp ace/(a) & 'q' x.md",
}

type consumer struct {
	name string
	// docsOnly skips the media locations
	docsOnly bool
	run      func(t *testing.T, loc string) string
}

// consumers registered: link read, rename, filter index, dashboard widget, kanban ancestor select.
// not registered yet (still to add, each one a consumer of file paths): metadata keys and links
// rebuild, media relocation, book entries, search index keys, kanban card order.
var consumers = []consumer{
	{name: "link-read", run: linkRead},
	{name: "rename", run: rename},
	{name: "filter-index", docsOnly: true, run: filterIndex},
	{name: "dashboard-move", docsOnly: true, run: dashboardMove},
	{name: "kanban-ancestor-select", docsOnly: true, run: kanbanAncestorSelect},
}

// knownBugs are the disagreements that are not fixed yet, keyed "consumer loc". The test fails
// when one of them agrees again (remove it) and when any other one disagrees.
var knownBugs = map[string]string{}

func TestPathAgreement(t *testing.T) {
	for _, c := range consumers {
		for _, loc := range locations {
			if c.docsOnly && !strings.HasPrefix(loc, "docs/") {
				continue
			}
			key := c.name + " " + loc
			t.Run(key, func(t *testing.T) {
				setup(t)
				want := loc
				got := c.run(t, loc)
				reason, known := knownBugs[key]
				switch {
				case got != want && known:
					t.Logf("KNOWN BUG (%s): %s -> %q, want %q", reason, key, got, want)
				case got != want:
					t.Errorf("%s -> %q, want %q", key, got, want)
				case known:
					t.Errorf("%s agrees again, remove it from knownBugs", key)
				}
			})
		}
	}
	for key := range knownBugs {
		name, loc, _ := strings.Cut(key, " ")
		if !hasConsumer(name) || !contains(locations, loc) {
			t.Errorf("knownBugs entry %q names no registered consumer/location", key)
		}
	}
}

func hasConsumer(name string) bool {
	for _, c := range consumers {
		if c.name == name {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// setup points every storage at a fresh scratch dir.
func setup(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "knov-pathagreement")
	if err != nil {
		t.Fatal(err)
	}
	// storages stay open until the process ends, a windows host can not remove them before
	t.Cleanup(func() {
		files.WaitForCacheRefreshes()
		os.RemoveAll(dir)
	})
	configmanager.SetDataAndStoragePaths(dir, dir)
	for _, err := range []error{
		configStorage.Init("json", dir),
		cacheStorage.Init("json", dir),
		metadataStorage.Init("json", dir),
		contentStorage.Init(),
		chatStorage.Init(dir),
		searchStorage.Init("sqlite", dir),
		notificationStorage.Init(dir),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	contentHandler.Init()
	files.InvalidateFileListCache()
	files.OnFileMoved = nil
}

// put writes content to the file at the metadata path meta.
func put(t *testing.T, meta, content string) {
	t.Helper()
	full := pathutils.ToDocsPath(meta) // the meta path names its root, no guessing
	if strings.HasPrefix(meta, "media/") {
		full = pathutils.ToMediaPath(meta)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := contentStorage.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// movedTo is the location the file at loc is moved to: the same folder, named y.
func movedTo(loc string) string {
	return strings.Replace(loc, "x.", "y.", 1)
}

// linkForms are the ways the file at loc is linked from a doc, as the app writes them.
func linkForms(loc string) map[string]parser.Link {
	if rel, ok := strings.CutPrefix(loc, "media/"); ok {
		return map[string]parser.Link{
			"markdown": {Kind: parser.LinkMarkdown, Text: "l", Path: "/media/" + rel},
			"wiki":     {Kind: parser.LinkWiki, Path: "media/" + rel},
		}
	}
	rel := strings.TrimPrefix(loc, "docs/")
	return map[string]parser.Link{
		"markdown": {Kind: parser.LinkMarkdown, Text: "l", Path: parser.FilesLinkPath(loc)},
		"wiki":     {Kind: parser.LinkWiki, Path: parser.DocsWikiPath(rel)},
	}
}

// resolved reads the links of content written in docPath and returns the target of each, joined.
func resolved(docPath, content string) string {
	var targets []string
	parser.RewriteLinks(content, func(l parser.Link) (string, bool) {
		targets = append(targets, parser.LinkTarget(docPath, l).String())
		return "", false
	})
	return strings.Join(targets, "|")
}

// both returns want when both link forms read as want, else what they read as.
func both(want, got string) string {
	if got == want+"|"+want {
		return want
	}
	return got
}

func linkRead(t *testing.T, loc string) string {
	put(t, loc, "target")
	src := linkSource(loc)
	return both(loc, resolved("docs/doc.md", src))
}

func linkSource(loc string) string {
	forms := linkForms(loc)
	return forms["markdown"].String() + "\n" + forms["wiki"].String() + "\n"
}

// rename moves the linked file and reads the links of the doc again.
func rename(t *testing.T, loc string) string {
	put(t, loc, "target")
	put(t, "docs/doc.md", linkSource(loc))
	if err := files.MetaDataInitializeAll(); err != nil {
		t.Fatal(err)
	}
	if err := files.UpdateLinksForSingleFile("docs/doc.md"); err != nil {
		t.Fatal(err)
	}
	to := movedTo(loc)
	var err error
	if rel, ok := strings.CutPrefix(loc, "media/"); ok {
		err = files.MoveMediaFileNoRefresh(rel, strings.TrimPrefix(to, "media/"))
	} else {
		err = files.MoveFileNoRefresh(logging.KeyApp, meta(t, loc), meta(t, to))
	}
	if err != nil {
		t.Fatalf("move %s: %v", loc, err)
	}
	b, err := os.ReadFile(pathutils.ToDocsPath("doc.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := resolved("docs/doc.md", string(b))
	// the consumer ends at the moved location, the test compares with loc
	return back(loc, both(to, got))
}

// back maps the moved location the consumer ended at to loc, so the caller compares with loc.
func back(loc, got string) string {
	if got == movedTo(loc) {
		return loc
	}
	return got
}

// filterIndex generates the index of a filter that selects every file and reads the link of the
// file at loc.
func filterIndex(t *testing.T, loc string) string {
	put(t, loc, "target")
	if err := files.MetaDataInitializeAll(); err != nil {
		t.Fatal(err)
	}
	files.InvalidateFileListCache()
	if err := filter.GenerateFilterIndex("agree", &filter.Config{Logic: "and"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(pathutils.ToDocsPath("agree" + configmanager.ExtensionForEditor("index")))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	parser.RewriteLinks(string(b), func(l parser.Link) (string, bool) {
		if tgt := parser.LinkTarget("docs/agree.md", l); strings.HasSuffix(tgt.String(), "x.png") || strings.HasSuffix(tgt.String(), "x.md") || strings.HasSuffix(tgt.String(), "'q' x.md") {
			got = append(got, tgt.String())
		}
		return "", false
	})
	return strings.Join(got, "|")
}

// dashboardMove moves the file at loc while a widget shows it and another shows docs/x.md (or
// docs/other.md) - the widget of the moved file follows it, the other one stays.
func dashboardMove(t *testing.T, loc string) string {
	put(t, loc, "target")
	decoy := "docs/x.md"
	if loc == decoy {
		decoy = "docs/other.md"
	}
	put(t, decoy, "decoy")
	rel := func(meta string) string { return strings.TrimPrefix(meta, "docs/") }
	widget := func(id, meta string) dashboard.Widget {
		return dashboard.Widget{ID: id, Type: dashboard.WidgetTypeFileContent, Config: dashboard.WidgetConfig{FileContent: &dashboard.FileContentConfig{FilePath: rel(meta)}}}
	}
	d := &dashboard.Dashboard{Name: "agree", Layout: dashboard.OneColumn, Widgets: []dashboard.Widget{widget("moved", loc), widget("decoy", decoy)}}
	if err := dashboard.Create(d); err != nil {
		t.Fatal(err)
	}
	files.OnFileMoved = dashboard.PatchFilePathForMove
	if err := files.MoveFileNoRefresh(logging.KeyApp, meta(t, loc), meta(t, movedTo(loc))); err != nil {
		t.Fatalf("move %s: %v", loc, err)
	}
	got, err := dashboard.Get("agree")
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Widgets[1].Config.FileContent.FilePath; p != rel(decoy) {
		return fmt.Sprintf("the widget of %s became %s", decoy, p)
	}
	return back(loc, pathutils.DocsPath(got.Widgets[0].Config.FileContent.FilePath).String())
}

// kanbanAncestorSelect offers the file at loc as ancestor in the select and filters the cards with
// the picked option value as "child-of" - a card whose parent is loc has to match.
func kanbanAncestorSelect(t *testing.T, loc string) string {
	opts := render.RenderAncestorOptions([]pathutils.MetaPath{meta(t, loc)})
	_, rest, _ := strings.Cut(opts, `value="`)
	value, _, _ := strings.Cut(rest, `"`)
	value = html.UnescapeString(value) // the browser sends the unescaped attribute
	card := files.File{Path: "docs/card.md", Metadata: &files.Metadata{Parents: []pathutils.MetaPath{meta(t, loc)}}}
	got := filter.FilterFileList([]files.File{card}, []filter.Criteria{{Metadata: "child-of", Operator: "equals", Value: value}}, "and")
	if len(got) == 1 {
		return loc
	}
	return fmt.Sprintf("no card matched option value %q", value)
}

func TestMain(m *testing.M) {
	parser.Init()
	os.Exit(m.Run())
}

// meta reads s as the metadata path it has to be.
func meta(t *testing.T, s string) pathutils.MetaPath {
	t.Helper()
	m, ok := pathutils.ParseMeta(s)
	if !ok {
		t.Fatalf("%q is no metadata path", s)
	}
	return m
}
