package files

import (
	"strings"
	"testing"

	"knov/internal/configmanager"
)

// setHideFilesByTag sets HideFilesByTag for the duration of the test and restores the previous value on cleanup.
func setHideFilesByTag(t *testing.T, entries ...string) {
	t.Helper()
	prev := configmanager.HideFilesByTag.Get()
	t.Cleanup(func() {
		if err := configmanager.HideFilesByTag.SetFromString(strings.Join(prev, ",")); err != nil {
			t.Fatalf("failed to restore HideFilesByTag: %v", err)
		}
	})
	if err := configmanager.HideFilesByTag.SetFromString(strings.Join(entries, ",")); err != nil {
		t.Fatalf("failed to set HideFilesByTag: %v", err)
	}
}

func TestFilterByVisibilityHideFilesByTag(t *testing.T) {
	tagged := File{Path: "notes/a.md", Metadata: &Metadata{Tags: []string{"kb-status-inbox"}}}
	untagged := File{Path: "notes/b.md", Metadata: &Metadata{Tags: []string{"other"}}}
	noMetadata := File{Path: "notes/c.md"}

	setHideFilesByTag(t, "kb-status*")
	got := FilterByVisibility([]File{tagged, untagged, noMetadata}, "")
	if len(got) != 2 || got[0].Path != untagged.Path || got[1].Path != noMetadata.Path {
		t.Fatalf("expected tagged file excluded, got %v", got)
	}
}

func TestFilterByVisibilityHideFilesByTagScoped(t *testing.T) {
	tagged := File{Path: "notes/a.md", Metadata: &Metadata{Tags: []string{"kb-status-inbox"}}}
	noMetadata := File{Path: "notes/c.md"}

	setHideFilesByTag(t, "kb-status*::kanban")

	got := FilterByVisibility([]File{tagged, noMetadata}, configmanager.HideScopeKanban)
	if len(got) != 1 || got[0].Path != noMetadata.Path {
		t.Fatalf("expected only the file without metadata visible in kanban scope, got %v", got)
	}

	got = FilterByVisibility([]File{tagged}, configmanager.HideScopeTree)
	if len(got) != 1 {
		t.Fatalf("expected file visible outside kanban scope, got %v", got)
	}
}

func TestFilterByVisibilityNoHideFilesByTag(t *testing.T) {
	tagged := File{Path: "notes/a.md", Metadata: &Metadata{Tags: []string{"kb-status-inbox"}}}

	setHideFilesByTag(t)
	got := FilterByVisibility([]File{tagged}, "")
	if len(got) != 1 {
		t.Fatalf("expected file visible with no HideFilesByTag configured, got %v", got)
	}
}
