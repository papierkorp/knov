package files

import (
	"os"
	"slices"
	"strings"
	"testing"

	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/parser"
	"knov/internal/test/specialchars"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-files-test")
	if err != nil {
		panic(err)
	}
	if err := configStorage.Init("json", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// setHideFilesByTag sets HideFilesByTag for the duration of the test and restores the previous value on cleanup.
func setHideFilesByTag(t *testing.T, entries ...string) {
	t.Helper()
	prev := configmanager.HideFilesByTag.Get()
	t.Cleanup(func() {
		if err := configmanager.SetSetting(configmanager.HideFilesByTag, strings.Join(prev, ",")); err != nil {
			t.Fatalf("failed to restore HideFilesByTag: %v", err)
		}
	})
	if err := configmanager.SetSetting(configmanager.HideFilesByTag, strings.Join(entries, ",")); err != nil {
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

// SanitizeKanbanTags drops newly added invalid kanban tags but keeps the ones already on the file,
// and a newly added status tag replaces the existing one.
func TestSanitizeKanbanTagsKeepsExisting(t *testing.T) {
	orphan := configmanager.GetKanbanPrefix() + "-status-gone"
	inbox := configmanager.KanbanStatusTag("inbox")

	got, err := SanitizeKanbanTags([]string{"a", orphan}, []string{"a", orphan})
	if err != nil || !slices.Equal(got, []string{"a", orphan}) {
		t.Errorf("existing tags: got %v, %v, want [a %s] and no error", got, err, orphan)
	}
	got, err = SanitizeKanbanTags(nil, []string{"a", orphan})
	if err == nil || !slices.Equal(got, []string{"a"}) {
		t.Errorf("new invalid tag: got %v, %v, want [a] and an error", got, err)
	}
	got, err = SanitizeKanbanTags([]string{"a", orphan}, []string{"a", orphan, inbox})
	if err != nil || !slices.Equal(got, []string{"a", inbox}) {
		t.Errorf("new status: got %v, %v, want [a %s] and no error", got, err, inbox)
	}
}

// a renamed file's link is written so link metadata reads it back as the same file, for every
// special-char name - renamed to it, and renamed away from it again (the links the first rename wrote)
func TestRenamedLinkReadsBack(t *testing.T) {
	h := &parser.MarkdownHandler{}
	for _, name := range specialchars.Names {
		p := "docs/" + name
		for _, link := range []string{"[x](docs/old.md)", "[[docs/old.md]]", "[[docs/old]]", "[x](/files/docs/old.md)", `<a href="/files/docs/old.md">x</a>`} {
			content, ok := parser.RewriteLinks(link, renameLinkFunc("docs/src.md", "docs/old.md", p))
			if got := h.ExtractLinks([]byte(content), "docs/src.md"); !ok || len(got) != 1 || got[0] != p {
				t.Errorf("%q renamed to %q = %q, links %q", link, p, content, got)
				continue
			}
			back, ok := parser.RewriteLinks(content, renameLinkFunc("docs/src.md", p, "docs/new.md"))
			if got := h.ExtractLinks([]byte(back), "docs/src.md"); !ok || len(got) != 1 || got[0] != "docs/new.md" {
				t.Errorf("%q renamed from %q = %q, links %q", content, p, back, got)
			}
		}
	}
}
