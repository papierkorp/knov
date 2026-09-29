package configeditor

import (
	"errors"
	"testing"

	"knov/internal/files"
	"knov/internal/pathutils"
)

// newTestKind mirrors how a real editor builds its descriptor.
func newTestKind() Kind { return MustNew("filter/", files.EditorTypeFilter, "index") }

func TestMustNewNormalizesPrefix(t *testing.T) {
	for _, in := range []string{"filter", "filter/"} {
		got, err := MustNew(in, files.EditorTypeFilter, "index").key("my/id")
		if err != nil || got != "filter/my/id" {
			t.Errorf("MustNew(%q).key = %q, %v, want filter/my/id", in, got, err)
		}
	}
}

func TestKeyRejectsTraversal(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../x", "../../x", "a/../../x"} {
		if _, err := newTestKind().key(id); err == nil {
			t.Errorf("key(%q) accepted, want error", id)
		}
	}
}

func TestKeyRejectsReservedFolders(t *testing.T) {
	for _, id := range []string{"docs/x", "media/x", "files/a/x"} {
		if _, err := newTestKind().key(id); !errors.Is(err, pathutils.ErrReservedPath) {
			t.Errorf("key(%q) = %v, want ErrReservedPath", id, err)
		}
	}
	if _, err := newTestKind().key("a/docs/x"); err != nil {
		t.Errorf("key(a/docs/x) rejected: %v", err)
	}
}

func TestKeyCleansID(t *testing.T) {
	for id, want := range map[string]string{"a/../b": "filter/b", "a/": "filter/a", "./a//b": "filter/a/b"} {
		if got, err := newTestKind().key(id); err != nil || got != want {
			t.Errorf("key(%q) = %q, %v, want %q", id, got, err, want)
		}
	}
}

// storage backends are not initialized here, so reaching the paired file
// delete would panic - an error return proves validation runs first.
func TestDeleteRejectsTraversalBeforeTouchingFiles(t *testing.T) {
	for _, id := range []string{"..", "../x", "a/../../x"} {
		if err := newTestKind().Delete(id); err == nil {
			t.Errorf("Delete(%q) accepted, want error", id)
		}
	}
}

func TestMustNewPanicsOnEmptyExtKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustNew with empty extKey did not panic")
		}
	}()
	MustNew("filter/", files.EditorTypeFilter, "")
}

func TestLabelDropsTrailingSlash(t *testing.T) {
	if got := newTestKind().label(); got != "filter" {
		t.Errorf("label = %q, want filter", got)
	}
}

func TestPairedPathIDRoundTrip(t *testing.T) {
	k := newTestKind()
	for _, id := range []string{"my/filter", "top", "a/b/c"} {
		if got := k.IDFromPath(k.PairedPath(id)); got != id {
			t.Errorf("IDFromPath(PairedPath(%q)) = %q, want %q", id, got, id)
		}
	}
}
