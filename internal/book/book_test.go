package book

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/test/specialchars"
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
	contentHandler.Init()
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
		if err := contentStorage.WriteFile(pathutils.GuessMeta(p).FullPath(), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	FilterResolver = func(id string) ([]pathutils.MetaPath, error) {
		if id == "empty" {
			return []pathutils.MetaPath{"docs/img.png"}, nil // only a skipped binary: nothing to inline
		}
		if id != "tasks" {
			return nil, errors.New("filter not found")
		}
		return []pathutils.MetaPath{"docs/a.md", "docs/C#.md", "docs/img.png", "docs/b.txt"}, nil
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

// a picked special-char file path (with or without a section) is stored encoded, read back by
// Parse as the same file, shown decoded again in the editor and included by ComposeEntries
func TestFileRefRoundTrip(t *testing.T) {
	for _, p := range specialchars.Names {
		// a typed entry is trimmed, so a name with leading / trailing spaces can't be an entry
		if !specialchars.ValidOn(runtime.GOOS, p) || strings.TrimSpace(p) != p {
			continue
		}
		full := pathutils.GuessMeta(p).FullPath()
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := contentStorage.WriteFile(full, []byte("content of "+p+"\n\n## My Section\n\nsection of "+p+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		for _, section := range []string{"", "My Section"} {
			entries := Parse(ToMarkdown([]Entry{{Type: EntryFile, Value: EncodeFileRef(p, section)}}))
			if len(entries) != 1 || entries[0].Type != EntryFile {
				t.Errorf("Parse(ToMarkdown(%q, %q)) = %+v", p, section, entries)
				continue
			}
			if gotPath, gotSection := DecodeFileRef(entries[0].Value); gotPath != p || gotSection != parser.AnchorID(section) {
				t.Errorf("DecodeFileRef(%q) = %q, %q, want %q, %q", entries[0].Value, gotPath, gotSection, p, section)
			}
			if path := parser.ResolveWikiTarget(entries[0].Value); path != p {
				t.Errorf("ResolveWikiTarget(%q) = %q, want %q", entries[0].Value, path, p)
			}
			want := "content of " + p
			if section != "" {
				want = "section of " + p
			}
			if got := ComposeEntries("test.book", entries); !strings.Contains(got, want) {
				t.Errorf("ComposeEntries(%q) = %q, want %q included", entries[0].Value, got, want)
			}
		}
	}
}

// a "|" or "]]" in a typed section doesn't end the entry's wikilink, it is stored as heading id
func TestFileRefSectionSpecialChars(t *testing.T) {
	entries := Parse(ToMarkdown([]Entry{{Type: EntryFile, Value: EncodeFileRef("a.md", "x|y]]z%")}}))
	if len(entries) != 1 {
		t.Fatalf("Parse = %+v", entries)
	}
	if p, s := DecodeFileRef(entries[0].Value); p != "a.md" || s != parser.AnchorID("x|y]]z%") {
		t.Errorf("DecodeFileRef(%q) = %q, %q", entries[0].Value, p, s)
	}
}

// a chapter's bare, "./" and "../" links are resolved against the chapter, its wikilinks and "/"
// links stay docs-root - the book renders pathless
func TestComposeRelativeLinks(t *testing.T) {
	full := pathutils.GuessMeta("rel/sub/ch.md").FullPath()
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := contentStorage.WriteFile(full, []byte("[a](../a.md) [[./b]] ![c](./c.png) [d](d.md) [e](/e.md) [[f]]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	want := "[a](rel/a.md) [[rel/sub/b]] ![c](rel/sub/c.png) [d](rel/sub/d.md) [e](/e.md) [[f]]"
	if got := ComposeEntries("test.book", []Entry{{Type: EntryFile, Value: EncodeFileRef("rel/sub/ch.md", "")}}); got != want {
		t.Errorf("ComposeEntries = %q, want %q", got, want)
	}
}
