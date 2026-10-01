package files

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"knov/internal/configmanager"
)

func TestExportData(t *testing.T) {
	prev := configmanager.GetAppConfig()
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prev.DataPath, prev.StoragePath) })

	data := t.TempDir()
	storage := filepath.Join(data, "storage") // storage configured inside the data folder
	configmanager.SetDataAndStoragePaths(data, storage)
	for _, rel := range []string{"docs/a/b.md", "media/c.png", ".git/HEAD", "storage/export/files.zip"} {
		path := filepath.Join(data, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var names []string
	add := func(name string, _ time.Time, _ io.Reader) error {
		names = append(names, name)
		return nil
	}
	if err := ExportData(context.Background(), add); err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if want := []string{"docs/a/b.md", "media/c.png"}; !slices.Equal(names, want) {
		t.Errorf("exported %v, want %v (.git and storage folder skipped)", names, want)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExportData(ctx, add); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled export returned %v, want context.Canceled", err)
	}
}
