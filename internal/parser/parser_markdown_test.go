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
