package dokuwikiconverter

import (
	"io"
	"runtime"
	"strings"
	"testing"

	"knov/internal/parser"
	"knov/internal/test/specialchars"
)

func TestConvertExportEntry(t *testing.T) {
	convert := func(name, content string) (string, string) {
		t.Helper()
		name, r, err := ConvertExportEntry(name, strings.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(r)
		return name, string(data)
	}

	if name, data := convert("docs/page.txt", "====== Title ======"); name != "docs/page.md" || data == "====== Title ======" {
		t.Errorf("dokuwiki file = %q %q, want renamed to .md and converted", name, data)
	}
	if name, data := convert("media/c.png", "png"); name != "media/c.png" || data != "png" {
		t.Errorf("other file = %q %q, want unchanged", name, data)
	}
}

// every special-char name that isn't dokuwiki syntax itself (# | [ ] : > in a link, ? in media,
// dokuwiki trims ids) converts to a link that link metadata reads back as the same file
func TestSpecialCharLinks(t *testing.T) {
	h := &parser.MarkdownHandler{}
	for _, name := range specialchars.Names {
		if specialchars.SplitsOn(runtime.GOOS, name) {
			continue
		}
		page := strings.TrimSuffix(name, ".md")
		if strings.TrimSpace(page) != page {
			continue
		}
		cases := map[string]string{}
		if !strings.ContainsAny(page, "#|[]:>") {
			cases["[["+page+"]]"] = "docs/" + page + ".md"
		}
		if img := strings.ToLower(page) + ".png"; !strings.ContainsAny(img, "|{}?:") {
			cases["{{:"+img+"}}"] = "media/" + img
		}
		for in, want := range cases {
			out := New().ConvertToMarkdown(in)
			if got := h.ExtractLinks([]byte(out), ""); len(got) != 1 || got[0] != want {
				t.Errorf("ConvertToMarkdown(%q) = %q, links %q, want %q", in, out, got, want)
			}
		}
	}
}

// a <catlist> becomes a link to the browse page of its namespace, written through the link codec
// (an html anchor in both formats - a markdown link would read as a docs file, not an app route)
func TestReplaceCatlistTags(t *testing.T) {
	for _, format := range []string{"markdown", "html"} {
		got := New().replaceCatlistTags("<catlist p:it:a&b>", format)
		if want := "<a href=\"/browse/folders/it/a%26b\">/browse/folders/it/a&amp;b</a>\n"; got != want {
			t.Errorf("%s: replaceCatlistTags = %q, want %q", format, got, want)
		}
	}
}
