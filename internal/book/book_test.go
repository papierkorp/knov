package book

import (
	"errors"
	"os"
	"strings"
	"testing"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-book-test")
	if err != nil {
		panic(err)
	}
	configmanager.SetDataAndStoragePaths(dir, dir)
	if err := contentStorage.Init(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestParseFilterEntry(t *testing.T) {
	cases := []struct {
		name, in string
		want     Entry
	}{
		{"book filter", "<!-- filter: tasks -->", Entry{Type: EntryFilter, Value: "tasks"}},
		{"id is trimmed", "<!-- filter:   tasks  -->", Entry{Type: EntryFilter, Value: "tasks"}},
		{"no spaces", "<!-- filter:tasks-->", Entry{Type: EntryFilter, Value: "tasks"}},
		{"empty id stays verbatim", "<!-- filter:  -->", Entry{Type: EntryUnknown, Value: "<!-- filter:  -->"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Parse(c.in + "\n")
			if len(got) != 1 || got[0] != c.want {
				t.Fatalf("Parse(%q) = %+v, want [%+v]", c.in, got, c.want)
			}
			if again := Parse(ToMarkdown(got)); len(again) != 1 || again[0] != c.want {
				t.Errorf("round-trip = %+v, want [%+v]", again, c.want)
			}
		})
	}
}

// a filter value that would break the comment line (crafted POST) is not written
func TestToMarkdownDropsMalformedFilter(t *testing.T) {
	for _, v := range []string{"a\nb", "a --> b"} {
		if md := ToMarkdown([]Entry{{Type: EntryFilter, Value: v}}); md != "" {
			t.Errorf("ToMarkdown(filter %q) = %q, want empty", v, md)
		}
	}
}

func TestExpandFilters(t *testing.T) {
	orig := FilterResolver
	defer func() { FilterResolver = orig }()

	for p, content := range map[string]string{"a.md": "body a\n", "C#.md": "body c#\n", "img.png": "PNG", "b.txt": "body b\n"} {
		if err := contentStorage.WriteFile(pathutils.ToDocsPath(p), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	FilterResolver = func(id string) ([]string, error) {
		if id == "empty" {
			return []string{"img.png"}, nil // only a skipped binary: nothing to inline
		}
		if id != "tasks" {
			return nil, errors.New("filter not found")
		}
		return []string{"a.md", "C#.md", "img.png", "b.txt"}, nil
	}

	got := expandFilters("my.book", []Entry{
		{Type: EntryTitle, Value: "T"},
		{Type: EntryFilter, Value: "tasks"},
		{Type: EntryFilter, Value: "gone"},
		{Type: EntryFilter, Value: "empty"},
	})

	// title kept, text matches inlined in order ("#" in a name is not an anchor), binary
	// skipped, unresolvable filter becomes a visible warning
	want := []Entry{
		{Type: EntryTitle, Value: "T"},
		{Type: EntryUnknown, Value: "body a"},
		{Type: EntryUnknown, Value: "body c#"},
		{Type: EntryUnknown, Value: "body b"},
	}
	if len(got) != len(want)+2 {
		t.Fatalf("expandFilters = %+v, want %+v + warning + empty note", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
	if warn := got[len(got)-2]; warn.Type != EntryUnknown || !strings.Contains(warn.Value, "`gone`") {
		t.Errorf("unresolvable filter = %+v, want warning marker", warn)
	}
	if last := got[len(got)-1]; last.Type != EntryUnknown || !strings.Contains(last.Value, "no files match filter `empty`") {
		t.Errorf("empty filter = %+v, want empty-state note", last)
	}
}

// a picked file path is stored encoded so the wikilink reads back as that file, and is shown
// decoded again in the editor
func TestFileRefRoundTrip(t *testing.T) {
	for _, v := range []string{"a%41.md", "docs/a[1].md#My Section", "a b.md|alias"} {
		stored := EncodeFileRef(v)
		if got := DecodeFileRef(stored); got != v {
			t.Errorf("DecodeFileRef(EncodeFileRef(%q)) = %q", v, got)
		}
		path, _ := parser.ResolveWikiTarget(stored)
		if want, _, _ := strings.Cut(strings.Split(v, "|")[0], "#"); path != want {
			t.Errorf("ResolveWikiTarget(%q) = %q, want %q", stored, path, want)
		}
	}
}
