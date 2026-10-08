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

func TestURLHelpers(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{"file url", ToFileURL, "notes.md", "/files/notes.md"},
		{"file url with space", ToFileURL, "my notes.md", "/files/my%20notes.md"},
		{"file url with subfolder", ToFileURL, "sub/notes.md", "/files/sub/notes.md"},
		{"file url strips leading slash", ToFileURL, "/notes.md", "/files/notes.md"},
		{"file url escapes unicode", ToFileURL, "nötes.md", "/files/n%C3%B6tes.md"},
		{"edit url", ToFileEditURL, "notes.md", "/files/edit/notes.md"},
		{"media url", ToMediaURL, "img.png", "/media/img.png"},
		{"edit table url", ToFileEditTableURL, "data.csv", "/files/edittable/data.csv"},
		{"history url", ToFileHistoryURL, "notes.md", "/files/history/notes.md"},
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
		{"files prefix stripped", "files/notes.md", "notes.md"},
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

func TestCheckNewDocsPath(t *testing.T) {
	for _, p := range []string{"docs/docs/x.md", "docs/media/x.md", "docs/files/a/x.md", "media"} {
		if CheckNewDocsPath(p) != ErrReservedPath {
			t.Errorf("%s should be reserved", p)
		}
	}
	// every reserved name must really be read as a prefix, or the list drifted from parsePath
	for _, name := range configmanager.ReservedDocsFolders() {
		if ToRelative(name+"/x.md") == name+"/x.md" {
			t.Errorf("%s is reserved but not stripped by parsePath", name)
		}
	}
	dataName := filepath.Base(configmanager.GetAppConfig().DataPath)
	for _, p := range []string{"x.md", "a/files/x.md", "mediafiles/x.md", "docs/" + dataName + "/x.md"} {
		if err := CheckNewDocsPath(p); err != nil {
			t.Errorf("%s should not be reserved: %v", p, err)
		}
	}
	existing := filepath.Join(getDocsPath(), "media", "existing.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := CheckNewDocsPath("docs/media/existing.md"); err != nil {
		t.Errorf("existing file should stay writable: %v", err)
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
		{"http://localhost/files/notes/a.md", "notes/a.md"},
		{"http://localhost/files/edit/notes/a.md?x=1#h", "notes/a.md"},
		{"http://localhost/files/edittable/t.md", "t.md"},
		{"http://localhost/files/history/notes/a.md", "notes/a.md"},
		{"http://localhost/files/n%C3%B6tes/a%20b.md", "nötes/a b.md"},
		{"http://localhost/files/new/codemirror", ""},
		{"http://localhost/files/../../etc/a.md", ""},
		{"http://localhost/files/edit/a/%2e%2e/%2E%2E/b.md", ""},
		{"http://localhost/files/a..b/c.md", "a..b/c.md"},
		{"http://localhost/dashboard/home", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := FileFromURL(c.url); got != c.want {
			t.Errorf("FileFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
