package searchStorage

import (
	"os"
	"slices"
	"testing"

	"knov/internal/configmanager"
	"knov/internal/pathutils"
)

// a deleted docs file and a deleted media file of the same relative name are two rows of the
// deleted-files index, keyed by their metadata paths
func TestIndexDeletedFileKeepsDocsAndMediaApart(t *testing.T) {
	dir, err := os.MkdirTemp("", "knov-searchstorage-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	configmanager.SetDataAndStoragePaths(dir, dir)
	ss, err := newSQLiteStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.db.Close()

	for _, p := range []pathutils.MetaPath{pathutils.DocsPath("x.txt"), pathutils.MediaPath("x.txt")} {
		if err := ss.IndexDeletedFile(p, []byte("deletedmarker "+p.String())); err != nil {
			t.Fatal(err)
		}
	}
	results, err := ss.SearchDeletedContent("deletedmarker", 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range results {
		got = append(got, r.Path)
	}
	slices.Sort(got)
	if want := []string{"docs/x.txt", "media/x.txt"}; !slices.Equal(got, want) {
		t.Errorf("deleted index rows = %q, want %q", got, want)
	}
}
