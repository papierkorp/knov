package metadataStorage

import (
	"os"
	"path/filepath"
	"testing"
)

// the cleanup after a migration removes the json files, not the sqlite db that shares the folder
func TestJSONCleanupKeepsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	js, err := newJSONStorageAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"docs/a.md", "docs/sub/b.md", "media/c.png"} {
		if err := js.Set(key, []byte(`{"path":"`+key+`"}`)); err != nil {
			t.Fatal(err)
		}
	}
	db := filepath.Join(dir, metadataDBFile)
	if err := os.WriteFile(db, []byte("db"), 0644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "docs", "keep.txt")
	if err := os.WriteFile(other, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := js.Cleanup(); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{db, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed by the json cleanup: %v", p, err)
		}
	}
	for _, p := range []string{filepath.Join(dir, "docs", "a.md.json"), filepath.Join(dir, "docs", "sub"), filepath.Join(dir, "media")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there after the json cleanup (%v)", p, err)
		}
	}
}
