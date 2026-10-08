package files

import (
	"os"
	"path/filepath"
	"testing"

	"knov/internal/parser"
)

func TestRewriteMediaLinks(t *testing.T) {
	moved := map[string]string{"uploads/a b.png": "uploads/a b.png", "wiki/img/c.jpg": "wiki/img/c.jpg", "wiki/img/c (1).jpg": "wiki/img/c (1).jpg", "wiki/img/c(1).jpg": "wiki/img/c(1).jpg",
		"docs/d.png": "docs/d.png", "img/e.png": "img/e.png", "wiki/F.png": "wiki/F.png", "wiki/f.png": "wiki/f-1.png",
		"wiki/archive.tar.gz": "wiki/archive.tar.gz", "wiki/report v2.docx": "wiki/report v2.docx", "files/setup.exe": "files/setup.exe"}
	paths := []string{"wiki/page.md", "wiki/img/e.png"}
	for src := range moved {
		paths = append(paths, src)
	}
	idx := newRelocateIndex(paths, moved)
	relink := idx.relinkFunc("wiki/page.md")

	cases := map[string]string{
		`![x](/uploads/a%20b.png =300x)`:     `![x](/media/uploads/a%20b.png)`,
		`![x](<../uploads/a b.png> "title")`: `![x](</media/uploads/a%20b.png> "title")`,
		`![x](img/c.jpg?w=1)`:                `![x](/media/wiki/img/c.jpg?w=1)`,
		`[[/docs/wiki/img/c.jpg|pic]]`:       `[[media/wiki/img/c.jpg|pic]]`,
		`<img src="/wiki/img/c.jpg">`:        `<img src="/media/wiki/img/c.jpg">`,
		`![x](/img/c.jpg)`:                   `![x](/media/wiki/img/c.jpg)`,
		`![x](<img/c (1).jpg>)`:              `![x](</media/wiki/img/c%20%281%29.jpg>)`,
		`![x](img/c(1).jpg)`:                 `![x](/media/wiki/img/c%281%29.jpg)`,
		`![x](other.png) [y](note.md)`:       `![x](other.png) [y](note.md)`,
		"```\n![x](img/c.jpg)\n```":          "```\n![x](img/c.jpg)\n```",
		// a real "docs" folder wins over the docs-root prefix
		`![x](/docs/d.png)`: `![x](/media/docs/d.png)`,
		// a bare markdown path is docs-root relative like it renders, doc-relative only as fallback
		`![x](img/e.png)`:         `![x](/media/img/e.png)`,
		`![x](uploads/a%20b.png)`: `![x](/media/uploads/a%20b.png)`,
		// a "./" link is relative to the doc's folder only, wiki/img/e.png isn't moved
		`![x](./img/e.png)`: `![x](./img/e.png)`,
		// wiki/img/e.png exists and isn't moved, so an html link isn't redirected to img/e.png
		`<img src="img/e.png">`: `<img src="img/e.png">`,
		// relative html links never walk up to a parent folder
		`<img src="uploads/a%20b.png">`: `<img src="uploads/a%20b.png">`,
		// knov's own /files/ view url
		`![x](/files/img/e.png)`: `![x](/media/img/e.png)`,
		// no case-insensitive match
		`![x](/WIKI/f.PNG)`: `![x](/WIKI/f.PNG)`,
		// non-image files: plain links, wiki links and html href
		`[dl](archive.tar.gz)`:           `[dl](/media/wiki/archive.tar.gz)`,
		`[doc](<report v2.docx>)`:        `[doc](</media/wiki/report%20v2.docx>)`,
		`[[wiki/report v2.docx|report]]`: `[[media/wiki/report v2.docx|report]]`,
		// wiki links are docs-root relative, not relative to the doc's folder
		`[[report v2.docx|report]]`:   `[[report v2.docx|report]]`,
		`<a href="/files/setup.exe">`: `<a href="/media/files/setup.exe">`,
		// inline code is left alone
		"`![x](img/c.jpg)` ![x](img/c.jpg)":         "`![x](img/c.jpg)` ![x](/media/wiki/img/c.jpg)",
		"`` a ` ![x](img/c.jpg) `` ![x](img/c.jpg)": "`` a ` ![x](img/c.jpg) `` ![x](/media/wiki/img/c.jpg)",
		// reference-style definitions, footnotes are left alone
		`[pic]: img/c.jpg "title"`: `[pic]: /media/wiki/img/c.jpg "title"`,
		`[^1]: img/c.jpg`:          `[^1]: img/c.jpg`,
		// crlf line endings keep their "\r"
		"[pic]: img/c.jpg\r\nx": "[pic]: /media/wiki/img/c.jpg\r\nx",
		// unencoded spaces stay in the path, only a title / size ends it
		`![x](/uploads/a b.png "t")`: `![x](/media/uploads/a%20b.png "t")`,
		// trailing whitespace isn't part of the path
		`![x](img/c.jpg )`:              `![x](/media/wiki/img/c.jpg )`,
		`<img alt="a" src="img/c.jpg">`: `<img alt="a" src="/media/wiki/img/c.jpg">`,
	}
	for in, want := range cases {
		if got, _ := parser.RewriteLinks(in, relink); got != want {
			t.Errorf("RewriteLinks(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSniffMimeType(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		content []byte
		want    string
	}{
		"setup.exe":   {[]byte{0x4d, 0x5a, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00}, "application/octet-stream"},
		"report.docx": {[]byte{0x50, 0x4b, 0x03, 0x04, 0x14, 0x00, 0x06, 0x00}, "application/zip"},
		"archive.tar": {append([]byte("file.txt"), make([]byte, 504)...), "application/octet-stream"},
		"notes.json":  {[]byte(`{"a": 1}`), "text/plain; charset=utf-8"},
		"empty.bin":   {nil, ""},
	}
	for name, c := range cases {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, c.content, 0644); err != nil {
			t.Fatal(err)
		}
		if got := sniffMimeType(p); got != c.want {
			t.Errorf("sniffMimeType(%s) = %q, want %q", name, got, c.want)
		}
	}
}
