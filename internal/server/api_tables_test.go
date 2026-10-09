package server

import (
	"strings"
	"testing"
)

// links in table cells open the same file as in the rendered page - bare, "./" and windows
// backslash paths; raw html (src / href) is omitted like on the page, not passed through
func TestSimpleToTableDataCellLinks(t *testing.T) {
	for _, c := range []struct{ cell, want string }{
		{`[x](a.md)`, `href="/files/sub/a.md"`},
		{`[x](./a.md)`, `href="/files/sub/a.md"`},
		{`[x](/a.md)`, `href="/files/a.md"`},
		{`[x](dir\a.md)`, `href="/files/sub/dir/a.md"`},
		{`[x](..\a.md)`, `href="/files/a.md"`},
		{`<a href="a.md">x</a>`, `<!-- raw HTML omitted -->`},
		{`<img src="p.png">`, `<!-- raw HTML omitted -->`},
	} {
		got := simpleToTableData([]string{"h"}, [][]string{{c.cell}}, nil, "docs/sub/n.md").Rows[0][0].Content
		if !strings.Contains(got, c.want) {
			t.Errorf("%q renders %q, want it to contain %q", c.cell, got, c.want)
		}
	}
}
