package pathutils

import (
	"os"
	"path/filepath"
	"testing"

	"knov/internal/configmanager"
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
