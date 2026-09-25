package configeditor

import (
	"testing"

	"knov/internal/files"
)

// newTestKind mirrors how a real editor builds its descriptor.
func newTestKind() Kind { return MustNew("filter/", files.EditorTypeFilter, "index") }

func TestMustNewNormalizesPrefix(t *testing.T) {
	for _, in := range []string{"filter", "filter/"} {
		if got, _ := MustNew(in, files.EditorTypeFilter, "index").key("my/id"); got != "filter/my/id" {
			t.Errorf("MustNew(%q).key = %q, want filter/my/id", in, got)
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
