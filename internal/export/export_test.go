package export

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"knov/internal/configmanager"
)

// readZip returns the entry names of the archive in b.
func readZip(t *testing.T, b []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestWriteMarkdown(t *testing.T) {
	prev := configmanager.GetAppConfig()
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prev.DataPath, prev.StoragePath) })
	data := t.TempDir()
	configmanager.SetDataAndStoragePaths(data, t.TempDir())
	// a.txt converted to markdown collides with the existing a.md, so it is renamed
	if err := os.WriteFile(filepath.Join(data, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "a.txt"), []byte("====== title ======"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Write(context.Background(), KindMarkdown, &buf, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	if names := readZip(t, buf.Bytes()); len(names) != 2 || names[0] != "a.md" || names[1] != "a-1.md" {
		t.Errorf("unexpected archive entries: %v, want a.md and the converted a-1.md", names)
	}
}

func TestAvailableRemove(t *testing.T) {
	prev := configmanager.GetAppConfig()
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prev.DataPath, prev.StoragePath) })
	configmanager.SetDataAndStoragePaths(t.TempDir(), t.TempDir())

	if Available() {
		t.Fatal("archive available before it was created")
	}
	if err := os.MkdirAll(dir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte("zip"), 0644); err != nil {
		t.Fatal(err)
	}
	if !Available() {
		t.Fatal("archive not available")
	}
	if err := Remove(); err != nil {
		t.Fatal(err)
	}
	if Available() {
		t.Error("archive still available after remove")
	}
}
