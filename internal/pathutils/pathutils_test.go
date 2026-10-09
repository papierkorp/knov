package pathutils

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"knov/internal/configmanager"
	"knov/internal/test/specialchars"
)

// parsePath resolves full paths against configmanager's DataPath, so every test
// needs a real (isolated) directory rather than the zero-value "".
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-pathutils-test")
	if err != nil {
		panic(err)
	}
	configmanager.SetDataAndStoragePaths(dir, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// a docs-relative path starting with a prefix name is a docs path through DocsPath
func TestDocsPath(t *testing.T) {
	for _, rel := range []string{"media/x.md", "docs/x.md", "files/x.md", "media", "a/b.md", "media/sub/"} {
		p := DocsPath(rel).String()
		if got, want := ToDocsPath(p), filepath.Join(DocsRoot(), rel); got != want {
			t.Errorf("ToDocsPath(DocsPath(%q)) = %q, want %q", rel, got, want)
		}
		if got := ToFullPath(p); got != filepath.Join(DocsRoot(), rel) {
			t.Errorf("ToFullPath(DocsPath(%q)) = %q", rel, got)
		}
		if got := ToWithPrefix(p); got != "docs/"+rel {
			t.Errorf("ToWithPrefix(DocsPath(%q)) = %q", rel, got)
		}
		if got := ToRelative(p); got != strings.Trim(rel, "/") {
			t.Errorf("ToRelative(DocsPath(%q)) = %q", rel, got)
		}
		if IsMedia(p) {
			t.Errorf("IsMedia(DocsPath(%q)) = true", rel)
		}
	}
}

func TestURLHelpers(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{"file url", ToFileURL, "docs/notes.md", "/files/notes.md"},
		{"file url with space", ToFileURL, "docs/my notes.md", "/files/my%20notes.md"},
		{"file url with subfolder", ToFileURL, "docs/sub/notes.md", "/files/sub/notes.md"},
		{"file url escapes unicode", ToFileURL, "docs/nötes.md", "/files/n%C3%B6tes.md"},
		{"file url of docs/media/x.md is literal", ToFileURL, "docs/media/x.md", "/files/media/x.md"},
		{"file url of docs/docs/x.md is literal", ToFileURL, "docs/docs/x.md", "/files/docs/x.md"},
		{"edit url", ToFileEditURL, "docs/notes.md", "/files/edit/notes.md"},
		{"media url", ToMediaURL, "img.png", "/media/img.png"},
		{"edit table url", ToFileEditTableURL, "docs/data.csv", "/files/edittable/data.csv"},
		{"history url", ToFileHistoryURL, "docs/notes.md", "/files/history/notes.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToRelative(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain docs path", "docs/notes.md", "notes.md"},
		{"plain media path", "media/img.png", "img.png"},
		{"no prefix defaults to docs", "notes.md", "notes.md"},
		{"leading slash stripped", "/docs/notes.md", "notes.md"},
		{"files folder kept", "files/notes.md", "files/notes.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToRelative(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsMediaIsDocs(t *testing.T) {
	if !IsMedia("media/img.png") {
		t.Error("expected media/img.png to be media")
	}
	if IsDocs("media/img.png") {
		t.Error("expected media/img.png to not be docs")
	}
	if !IsDocs("docs/notes.md") {
		t.Error("expected docs/notes.md to be docs")
	}
	if IsMedia("docs/notes.md") {
		t.Error("expected docs/notes.md to not be media")
	}
}

func TestToFullPathContainsTraversal(t *testing.T) {
	docsRoot := getDocsPath()
	got := ToFullPath("../../../../etc/passwd")
	if got != docsRoot {
		t.Errorf("path traversal escaped docs root: got %q, want %q", got, docsRoot)
	}
}

func TestToDocsPathAndToMediaPath(t *testing.T) {
	if got, want := ToDocsPath("notes.md"), filepath.Join(getDocsPath(), "notes.md"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ToMediaPath("img.png"), filepath.Join(getMediaPath(), "img.png"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveRelativeLink(t *testing.T) {
	cases := []struct{ doc, link, want string }{
		{"docs/a/x/n.md", "../b.md", "a/b.md"},
		{"a/x/n.md", "./sub/b.md", "a/x/sub/b.md"},
		{"docs/n.md", "./b.md", "b.md"},
		{"docs/a/n.md", "../sub/", "sub/"},
		{"docs/a/n.md", "sub/b.md", "sub/b.md"},
		{"docs/a/n.md", "/b.md", "/b.md"},
		{"docs/n.md", "../../b.md", "b.md"},
		{"docs/a/x/n.md", ".", "a/x/"},
		{"docs/a/x/n.md", "..", "a/"},
		{"docs/a/n.md", "..", "/"},
		{"docs/n.md", "./", "/"},
	}
	for _, c := range cases {
		if got := ResolveRelativeLink(c.doc, c.link); got != c.want {
			t.Errorf("ResolveRelativeLink(%q, %q) = %q, want %q", c.doc, c.link, got, c.want)
		}
	}
}

// a "../" past the docs root is reported, also one climbing back down again
func TestLinkClimbsAboveRoot(t *testing.T) {
	cases := []struct {
		doc, link string
		want      bool
	}{
		{"docs/a/n.md", "../x.md", false},
		{"docs/a/n.md", "../../x.md", true},
		{"docs/a/n.md", "../../a/x.md", true},
		{"docs/a/b/n.md", "./../../", false},
		{"docs/n.md", "..", true},
		{"docs/n.md", "./x/../y.md", false},
		{"docs/n.md", "x/../../y.md", false}, // not a relative link
	}
	for _, c := range cases {
		if got := LinkClimbsAboveRoot(c.doc, c.link); got != c.want {
			t.Errorf("LinkClimbsAboveRoot(%q, %q) = %v, want %v", c.doc, c.link, got, c.want)
		}
	}
}

func TestRelativeLink(t *testing.T) {
	cases := []struct{ doc, target, want string }{
		{"docs/a/x/n.md", "docs/a/b.md", "../b.md"},
		{"docs/a/x/n.md", "a/x/sub/b.md", "./sub/b.md"},
		{"docs/n.md", "b.md", "./b.md"},
		{"docs/a/n.md", "b/c.md", "../b/c.md"},
		{"docs/a/x/n.md", "a/x", "../x"},
		{"docs/a/x/n.md", "a/x/sub/", "./sub/"},
		{"docs/n.md", "a/x/b.md", "./a/x/b.md"},
		{"docs/a/x/n.md", "/", "../../"},
		{"docs/n.md", "/", "./"},
	}
	// the docs root as metadata path, like parser.LinkTarget reads it
	if got := RelativeLink("docs/a/x/n.md", "docs/"); got != "../../" {
		t.Errorf(`RelativeLink("docs/a/x/n.md", "docs/") = %q, want "../../"`, got)
	}
	for _, c := range cases {
		if got := RelativeLink(c.doc, c.target); got != c.want {
			t.Errorf("RelativeLink(%q, %q) = %q, want %q", c.doc, c.target, got, c.want)
		}
		if got := ResolveRelativeLink(c.doc, RelativeLink(c.doc, c.target)); got != strings.TrimPrefix(c.target, "docs/") {
			t.Errorf("ResolveRelativeLink(%q, RelativeLink) = %q, want %q", c.doc, got, strings.TrimPrefix(c.target, "docs/"))
		}
	}
}

func TestBaseWithoutExt(t *testing.T) {
	cases := map[string]string{
		"docs/notes.md": "notes",
		`docs\notes.md`: "notes",
		"notes.md":      "notes",
		"notes":         "notes",
		"a/b/c.tar.gz":  "c.tar",
	}
	for in, want := range cases {
		if got := BaseWithoutExt(in); got != want {
			t.Errorf("BaseWithoutExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFolderContains(t *testing.T) {
	if !FolderContains("projects", "projects") {
		t.Error("expected folder to contain itself")
	}
	if !FolderContains("projects/sub", "projects") {
		t.Error("expected subfolder to be contained")
	}
	if FolderContains("projectsother", "projects") {
		t.Error("expected sibling with shared prefix to not be contained")
	}
}

func TestPathContains(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "data", "docs")
	if !PathContains(root, root) {
		t.Error("expected root to contain itself")
	}
	sub := filepath.Join(root, "sub", "notes.md")
	if !PathContains(root, sub) {
		t.Error("expected subpath to be contained")
	}
	sibling := filepath.Join(string(filepath.Separator), "data", "docsother")
	if PathContains(root, sibling) {
		t.Error("expected sibling with shared prefix to not be contained")
	}
}

// a docs file in a folder named docs, media or files is a valid new docs path and resolves to itself
func TestCheckNewDocsPath(t *testing.T) {
	dataName := filepath.Base(configmanager.GetAppConfig().DataPath)
	for _, rel := range []string{"x.md", "a/files/x.md", "mediafiles/x.md", "docs/x.md", "media/x.md", "files/a/x.md", "media", dataName + "/x.md"} {
		p := DocsPath(rel).String()
		if err := CheckNewDocsPath(p); err != nil {
			t.Errorf("%s should be allowed: %v", p, err)
		}
		if got, want := ToDocsPath(p), filepath.Join(getDocsPath(), filepath.FromSlash(rel)); got != want {
			t.Errorf("ToDocsPath(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestCheckNewDocsPathNames(t *testing.T) {
	// every corpus name is pinned here, so the links suite can skip what the policy rejects
	invalid := []string{"a#b.md", "a?b.md", "a|b.md", "[1].md", `a\b.md`, " lead.md", "trail.md ", "a]b.md", "x#/a.md", "x /a.md"}
	for _, p := range append(slices.Clone(specialchars.Names), "a]b.md", "x#/a.md", "x /a.md", "a&b.md", "a b/c d.md") {
		want := error(nil)
		if slices.Contains(invalid, p) {
			want = ErrInvalidName
		}
		if err := CheckNewDocsPath(p); err != want {
			t.Errorf("%q: want %v, got %v", p, want, err)
		}
	}
	// an existing name (git sync, manual copy) stays usable, only the new part is checked
	existing := filepath.Join(getDocsPath(), "a#b")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}
	if err := CheckNewDocsPath("a#b/new.md"); err != nil {
		t.Errorf("new file in an existing folder: %v", err)
	}
	if err := CheckNewDocsPath("a#b/new?.md"); err != ErrInvalidName {
		t.Errorf("new invalid name in an existing folder: want ErrInvalidName, got %v", err)
	}
}

func TestCheckTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dst"), 0755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(root, "a#b.md")
	cases := []struct {
		old, new string
		want     error
	}{
		{old, filepath.Join(root, "dst", "a#b.md"), nil},                       // existing bad name moves
		{old, filepath.Join(root, "new", "a#b.md"), nil},                       // into a valid new folder
		{old, filepath.Join(root, "x#", "a#b.md"), ErrInvalidName},             // into an invalid new folder
		{filepath.Join(root, "a.md"), old, ErrInvalidName},                     // renamed to a bad name
		{filepath.Join(root, "a.md"), filepath.Join(root, "dst", "b.md"), nil}, // valid rename
		{old, filepath.Join(root, "dst", "a#b_2.md"), ErrInvalidName},          // a rename keeping the bad char
		{old, filepath.Join(root, "dst", "a_b_2.md"), nil},                     // kanban collision name (CleanName)
	}
	for _, c := range cases {
		if got := CheckTarget(c.old, c.new); !errors.Is(got, c.want) {
			t.Errorf("CheckTarget(%q, %q) = %v, want %v", c.old, c.new, got, c.want)
		}
	}
}

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"a.md":             "a.md",
		"a#b?c|d[e]f\\.md": "a_b_c_d_e_f_.md",
		" a b.md ":         "a b.md",
		"#":                "_",
		" ":                "",
	}
	for in, want := range cases {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFileFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"http://localhost/files/notes/a.md", "docs/notes/a.md"},
		{"http://localhost/files/edit/notes/a.md?x=1#h", "docs/notes/a.md"},
		{"http://localhost/files/edittable/t.md", "docs/t.md"},
		{"http://localhost/files/history/notes/a.md", "docs/notes/a.md"},
		{"http://localhost/files/n%C3%B6tes/a%20b.md", "docs/nötes/a b.md"},
		{"http://localhost/files/new/codemirror", ""},
		{"http://localhost/files/../../etc/a.md", ""},
		{"http://localhost/files/edit/a/%2e%2e/%2E%2E/b.md", ""},
		{"http://localhost/files/a..b/c.md", "docs/a..b/c.md"},
		{"http://localhost/dashboard/home", ""},
		{"http://localhost/files/media/x.md", "docs/media/x.md"},
		{"", ""},
	}
	for _, c := range cases {
		if got := FileFromURL(c.url); got.String() != c.want {
			t.Errorf("FileFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// MetaPath and DocsRel are built by constructors, a folder named like a prefix keeps its name
func TestMetaPathConstructors(t *testing.T) {
	for _, rel := range []string{"x.md", "media/x.md", "docs/x.md", "files/x.md"} {
		m := DocsPath(rel)
		got, ok := m.DocsRel()
		if !ok || got != NewDocsRel(rel) || m.IsMedia() || got.MetaPath() != m {
			t.Errorf("DocsPath(%q) = %q, DocsRel() = %q %v", rel, m, got, ok)
		}
		if _, ok := m.MediaRel(); ok {
			t.Errorf("DocsPath(%q).MediaRel() = ok", rel)
		}
		if parsed, ok := ParseMeta(m.String()); !ok || parsed != m {
			t.Errorf("ParseMeta(%q) = %q %v", m, parsed, ok)
		}
	}
	m := MediaPath("/docs/x.png")
	if rel, ok := m.MediaRel(); !ok || rel != "docs/x.png" || !m.IsMedia() {
		t.Errorf("MediaPath = %q, MediaRel() = %q %v", m, rel, ok)
	}
	if _, ok := m.DocsRel(); ok {
		t.Errorf("MediaPath.DocsRel() = ok")
	}
	for _, s := range []string{"", "x.md", "files/x.md", "docs/../x.md", "media/./x.png"} {
		if m, ok := ParseMeta(s); ok {
			t.Errorf("ParseMeta(%q) = %q, want not a metadata path", s, m)
		}
	}
}
