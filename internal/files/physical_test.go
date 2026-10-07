package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"knov/internal/pathutils"
)

func TestKeepsInvalidName(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dst"), 0755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(root, "a#b.md")
	cases := []struct {
		err      error
		old, new string
		want     bool
	}{
		{pathutils.ErrInvalidName, old, filepath.Join(root, "dst", "a#b.md"), true},   // existing bad name moves
		{pathutils.ErrInvalidName, old, filepath.Join(root, "new", "a#b.md"), true},   // into a valid new folder
		{pathutils.ErrInvalidName, old, filepath.Join(root, "x#", "a#b.md"), false},   // into an invalid new folder
		{pathutils.ErrInvalidName, filepath.Join(root, "a.md"), old, false},           // renamed to a bad name
		{pathutils.ErrReservedPath, old, filepath.Join(root, "dst", "a#b.md"), false}, // other errors stay
	}
	for _, c := range cases {
		if got := keepsInvalidName(c.err, c.old, c.new); got != c.want {
			t.Errorf("keepsInvalidName(%v, %q, %q) = %v, want %v", c.err, c.old, c.new, got, c.want)
		}
	}
}

func TestRemoveEmptySubdirs(t *testing.T) {
	root := t.TempDir()
	mkdir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, rel), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mkdir("empty/nested/deeper")
	mkdir("keep/nested")
	if err := os.WriteFile(filepath.Join(root, "keep/nested/file.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	mkdir("target/empty")
	mkdir(".stfolder")
	mkdir("fresh")
	mkdir("board/todo/sub")
	old := time.Now().Add(-time.Hour)
	for _, rel := range []string{"empty", "empty/nested", "empty/nested/deeper", "keep", "keep/nested", "target", "target/empty", "board", "board/todo", "board/todo/sub", ".stfolder"} {
		if err := os.Chtimes(filepath.Join(root, rel), old, old); err != nil {
			t.Fatal(err)
		}
	}
	// symlinks may need extra privileges on windows - without one, "target" is just another empty folder
	hasLink := os.Symlink(filepath.Join(root, "target"), filepath.Join(root, "link")) == nil

	if _, err := removeEmptySubdirs(root, time.Now().Add(-time.Minute), []string{filepath.Join(root, "board")}); err != nil {
		t.Fatalf("removeEmptySubdirs failed: %v", err)
	}

	for rel, want := range map[string]bool{
		"":                    true,    // root is kept
		"empty":               false,   // nested empty chain removed
		"keep/nested":         true,    // a deeper file keeps its parents
		"link":                hasLink, // symlink is content, not followed
		"target":              false,   // only the symlink pointed at it
		"keep/nested/file.md": true,
		".stfolder":           true,  // dot folders are kept
		"fresh":               true,  // recently modified folders are kept
		"board/todo":          true,  // status folders of kept folders survive while empty
		"board/todo/sub":      false, // deeper folders of kept folders don't
	} {
		if _, err := os.Lstat(filepath.Join(root, rel)); (err == nil) != want {
			t.Errorf("%q exists = %v, want %v", rel, err == nil, want)
		}
	}
}

func TestRemoveEmptySubdirsMissingDir(t *testing.T) {
	if _, err := removeEmptySubdirs(filepath.Join(t.TempDir(), "missing"), time.Now(), nil); err != nil {
		t.Fatalf("expected a missing dir to be no error, got %v", err)
	}
}
