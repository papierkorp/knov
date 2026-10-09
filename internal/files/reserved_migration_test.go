package files

import (
	"os"
	"path/filepath"
	"testing"

	"knov/internal/chatStorage"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/metadataStorage"
	"knov/internal/parser"
	"knov/internal/searchStorage"
)

func TestMigrateReservedFolderMetadataAfterPurge(t *testing.T) {
	prev := configmanager.GetAppConfig()
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prev.DataPath, prev.StoragePath) })

	data := t.TempDir()
	storage, err := os.MkdirTemp("", "knov-reserved-migration")
	if err != nil {
		t.Fatal(err)
	}
	// the sqlite files stay open until the process ends, a windows host can not remove them before
	t.Cleanup(func() { os.RemoveAll(storage) })
	configmanager.SetDataAndStoragePaths(data, storage)
	if err := metadataStorage.Init("json", storage); err != nil {
		t.Fatal(err)
	}
	parser.Init()
	if err := searchStorage.Init("sqlite", storage); err != nil {
		t.Fatal(err)
	}
	if err := chatStorage.Init(storage); err != nil {
		t.Fatal(err)
	}
	if err := contentStorage.Init(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(data, "docs", "media"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "docs", "media", "x.md"), []byte("# x"), 0644); err != nil {
		t.Fatal(err)
	}
	// the record of docs/media/x.md from before docs paths carried their docs/ prefix
	if err := metadataStorage.Set("media/x.md", []byte(`{"path":"media/x.md","tags":["kept"]}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := MetaDataPurgeStale(); err != nil {
		t.Fatal(err)
	}
	if moved := MigrateReservedFolderMetadata(); moved != 1 {
		t.Fatalf("expected the record to be migrated after the purge, moved %d", moved)
	}
	m, err := MetaDataGet("docs/media/x.md")
	if err != nil || m == nil || len(m.Tags) != 1 || m.Tags[0] != "kept" {
		t.Fatalf("expected the record at docs/media/x.md, got %v (%v)", m, err)
	}
}
