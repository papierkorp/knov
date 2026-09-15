package parser

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLStripsScripts(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		notWant string
	}{
		{"script tag", `<p>hi</p><script>alert(1)</script>`, "<script"},
		{"onerror attr", `<img src="x" onerror="alert(1)">`, "onerror"},
		{"javascript href", `<a href="javascript:alert(1)">x</a>`, "javascript:"},
		// a quote embedded in an id value must never re-emerge as a live attribute
		// boundary in the output, even though it stays present as escaped text.
		{"id value can't break out of the attribute", `<div id="x&quot; onmouseover=&quot;alert(1)">x</div>`, `onmouseover="`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeHTML(c.in)
			if strings.Contains(got, c.notWant) {
				t.Errorf("sanitizeHTML(%q) = %q, want it stripped of %q", c.in, got, c.notWant)
			}
		})
	}
}

func TestSanitizeHTMLKeepsAppMarkup(t *testing.T) {
	cases := []string{
		`<h1 id="my-heading">Title</h1>`,
		`<div id="table-component-0" hx-get="/api/components/table?filepath=x" hx-trigger="load" hx-swap="outerHTML"></div>`,
		`<span class="todo-state" data-line="3"><i class="fa-solid fa-square"></i></span>`,
		`<button type="button" class="todo-date-clear">&times;</button>`,
		`<a href="#x" class="header-anchor" aria-hidden="true">#</a>`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got := sanitizeHTML(in)
			if got == "" {
				t.Errorf("sanitizeHTML(%q) stripped everything, want it preserved", in)
			}
		})
	}
}

// TestRenderHeadingSkipsPhantomScannedHeading covers the raw-line heading scan
// finding a line goldmark's real CommonMark parsing doesn't treat as a heading
// (a 4+-space-indented "# ..." is a code block per CommonMark, but ATXHeading
// is a documented approximation that doesn't stop at 3 spaces - see its doc
// comment). Without skipping that phantom scan entry by line number, the real
// heading after it would consume the phantom's id slot and render as "real-1"
// instead of "real".
func TestRenderHeadingSkipsPhantomScannedHeading(t *testing.T) {
	src := "text\n\n    # Indented\nmore text\n\n## Real\n"
	rendered, err := NewMarkdownHandler().Render([]byte(src), PathlessRender, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), `id="real"`) {
		t.Errorf("rendered html = %q, want it to contain %s", rendered, `id="real"`)
	}
}

// TestSharedUsedIDsDedupeAcrossDocuments covers renderDocsMarkdown's use case
// (internal/server/render/render_system.go): several documents concatenated onto
// one page, rendered one after the other while sharing the same usedIDs map, with
// the TOC built from each call's returned headings rather than a separate scan.
// The middle document repeats "Added" within itself as well as against the other
// documents, so this pins the accumulated dedup count staying aligned across more
// than one heading per document and more than two documents - not just the single
// cross-document repeat the original version of this test covered.
func TestSharedUsedIDsDedupeAcrossDocuments(t *testing.T) {
	docs := []string{
		"## Added\nfirst doc",
		"## Added\n### Added\nsecond doc",
		"## Added\nthird doc",
	}
	wantIDs := [][]string{
		{"added"},
		{"added-1", "added-2"},
		{"added-3"},
	}

	handler := NewMarkdownHandler()
	usedIDs := make(map[string]int)

	for i, doc := range docs {
		rendered, headings, err := handler.RenderWithUsedIDs([]byte(doc), PathlessRender, false, usedIDs)
		if err != nil {
			t.Fatalf("doc %d: Render: %v", i, err)
		}
		toc := HeadingsToTOC(headings)
		if len(toc) != len(wantIDs[i]) {
			t.Fatalf("doc %d: got %d TOC entries, want %d", i, len(toc), len(wantIDs[i]))
		}

		for j, want := range wantIDs[i] {
			if toc[j].ID != want {
				t.Errorf("doc %d heading %d: TOC id = %q, want %q", i, j, toc[j].ID, want)
			}
			if wantAttr := `id="` + want + `"`; !strings.Contains(string(rendered), wantAttr) {
				t.Errorf("doc %d heading %d: rendered html = %q, want it to contain %q", i, j, rendered, wantAttr)
			}
		}
	}
}
