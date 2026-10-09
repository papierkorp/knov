package parser

import (
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"knov/internal/configmanager"
	"knov/internal/pathutils"
	"knov/internal/test/specialchars"
	"knov/internal/utils"
)

// metadataTarget is the file link metadata reads the only link of content in doc as.
func metadataTarget(content, doc string) string {
	if links := NewMarkdownHandler().ExtractLinks([]byte(content), doc); len(links) == 1 {
		return links[0]
	}
	return ""
}

var renderedHTMLAttrRe = regexp.MustCompile(`(?:src|href)="([^"]*)"`)

// renderedLinkTarget is the file the only link of content renders to in doc, as the browser
// requests it - an html src/href from RenderLinks (goldmark drops raw html), the rest through
// the file view pipeline
func renderedLinkTarget(content, doc string) string {
	if strings.HasPrefix(content, "<") {
		m := renderedHTMLAttrRe.FindStringSubmatch(RenderLinks(content, doc))
		if m == nil {
			return ""
		}
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			return ""
		}
		if rel, ok := strings.CutPrefix(u.Path, "/media/"); ok {
			return "media/" + rel
		}
		if rel := pathutils.FileFromURL(u.String()); rel != "" || u.Path == "/files/" {
			return pathutils.ToWithPrefix(rel)
		}
		return u.Path
	}
	h := NewMarkdownHandler()
	parsed, _ := h.Parse([]byte(content), doc)
	out, err := h.Render(parsed, PathlessRender, false)
	if err != nil {
		return ""
	}
	if t := renderedTarget(string(out)); strings.HasPrefix(t, "media/") || t == "" {
		return t
	} else {
		return pathutils.ToWithPrefix(t)
	}
}

// for every special-char name and link kind, link metadata and the rendered page point at the
// same file - also a bare link to a media file and html src/href
func TestRenderedLinkMatchesMetadata(t *testing.T) {
	dir := t.TempDir()
	prevData, prevStorage := configmanager.GetDataPath(), configmanager.GetStoragePath()
	configmanager.SetDataAndStoragePaths(dir, dir)
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prevData, prevStorage) })

	const doc = "docs/sub/n.md"
	for _, p := range specialchars.Names {
		img, pdf := strings.Replace(p, ".md", ".png", 1), strings.Replace(p, ".md", ".pdf", 1)
		// at the media root for the media/ forms and in the doc's folder for the bare ones
		for _, m := range []string{img, pdf, "sub/" + img, "sub/" + pdf} {
			full := filepath.Join(dir, "media", filepath.FromSlash(m))
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		md, wiki, htm := encodeLinkPath(p, LinkMarkdown), encodeLinkPath(p, LinkWiki), encodeLinkPath(p, LinkHTML)
		forms := map[string]string{
			"markdown":             "[x](" + md + ")",
			"markdown <>":          "[x](<" + md + ">)",
			"markdown anchor":      "[x](" + md + "#sec)",
			"markdown relative":    "[x](./" + md + ")",
			"markdown file url":    "[x](" + fileURL(p) + ")",
			"markdown media":       "[x](" + encodeLinkPath("media/"+pdf, LinkMarkdown) + ")",
			"markdown media url":   "[x](" + pathutils.ToMediaURL(pdf) + ")",
			"markdown bare media":  "[x](" + encodeLinkPath(pdf, LinkMarkdown) + ")",
			"image":                "![x](" + encodeLinkPath("media/"+img, LinkMarkdown) + ")",
			"wiki":                 "[[" + wiki + "]]",
			"wiki anchor":          "[[" + wiki + "#sec]]",
			"wiki relative":        "[[./" + wiki + "]]",
			"wiki bare media":      "[[" + encodeLinkPath(pdf, LinkWiki) + "]]",
			"ref def":              "[r][id]\n\n[id]: " + md,
			"ref def bare media":   "[r][id]\n\n[id]: " + encodeLinkPath(pdf, LinkMarkdown),
			"html href":            `<a href="` + htm + `">x</a>`,
			"html href relative":   `<a href="./` + htm + `">x</a>`,
			"html href file url":   `<a href="` + fileURL(p) + `">x</a>`,
			"html src bare media":  `<img src="` + encodeLinkPath(img, LinkHTML) + `">`,
			"html src media url":   `<img src="` + pathutils.ToMediaURL(img) + `">`,
			"html href bare media": `<a href="` + encodeLinkPath(pdf, LinkHTML) + `">x</a>`,
		}
		// an image is rendered by renderImage, which reads "trail.png " as no image
		if strings.TrimSpace(img) == img {
			forms["image bare"] = "![x](" + encodeLinkPath(img, LinkMarkdown) + ")"
		}
		if bare := strings.TrimSuffix(p, ".md"); utils.WithDefaultLinkExt(bare) == p {
			forms["wiki no ext"] = "[[" + encodeLinkPath(bare, LinkWiki) + "]]"
		}
		for form, content := range forms {
			want := metadataTarget(content, doc)
			if got := renderedLinkTarget(content, doc); want == "" || got != want {
				t.Errorf("%s %q: %q renders to %q, link metadata reads %q", form, p, content, got, want)
			}
		}
	}
}

// docs files in the top-level docs folders named like a path prefix (docs/docs, docs/media,
// docs/files) are linked as themselves: a /files/<rel> url is the docs-relative path taken
// literally, a bare or ./ link from a doc inside such a folder stays in it, [[docs/media/x]]
// names it with its prefix - while a link written media/... (wiki, bare, ./, /media/) stays media.
func TestLinkTargetReservedFolders(t *testing.T) {
	cases := []struct{ name, doc, link, want string }{
		{"files url", "docs/a.md", "[x](/files/media/y.md)", "docs/media/y.md"},
		{"files url docs", "docs/a.md", "[x](/files/docs/y.md)", "docs/docs/y.md"},
		{"files url files", "docs/a.md", "[x](/files/files/y.md)", "docs/files/y.md"},
		{"files url plain", "docs/a.md", "[x](/files/y.md)", "docs/y.md"},
		{"files url html", "docs/a.md", `<a href="/files/media/y.md">x</a>`, "docs/media/y.md"},
		{"wiki files folder", "docs/a.md", "[[files/y]]", "docs/files/y.md"},
		{"wiki prefixed", "docs/a.md", "[[docs/media/y]]", "docs/media/y.md"},
		{"wiki media", "docs/a.md", "[[media/y.png]]", "media/y.png"},
		{"bare media", "docs/a.md", "[x](media/y.png)", "media/y.png"},
		{"dot media", "docs/a.md", "[x](./media/y.png)", "media/y.png"},
		{"parent media", "docs/s/a.md", "[x](../media/y.png)", "media/y.png"},
		{"root media", "docs/a.md", "[x](/media/y.png)", "media/y.png"},
		{"bare sibling in docs/media", "docs/media/k/a.md", "[x](y.md)", "docs/media/k/y.md"},
		{"dot sibling in docs/media", "docs/media/k/a.md", "[x](./y.md)", "docs/media/k/y.md"},
		{"parent in docs/media", "docs/media/k/a.md", "[x](../y.md)", "docs/media/y.md"},
		{"bare sibling in docs/docs", "docs/docs/k/a.md", "[x](y.md)", "docs/docs/k/y.md"},
		{"bare sibling in docs/files", "docs/files/k/a.md", "[x](y.md)", "docs/files/k/y.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NewMarkdownHandler().ExtractLinks([]byte(c.link), c.doc)
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("%s in %s reads %q, want %q", c.link, c.doc, got, c.want)
			}
		})
	}
}

// a new link to a docs file in a folder named docs or media reads back as that file
func TestFileLinkDestReservedFolders(t *testing.T) {
	for _, rel := range []string{"a.md", "files/a.md", "media/a.md", "docs/a.md"} {
		for _, kind := range []LinkKind{LinkWiki, LinkMarkdown} {
			var link string
			if kind == LinkWiki {
				link = "[[" + FileLinkDest(rel, "", kind) + "]]"
			} else {
				link = "[x](" + FileLinkDest(rel, "", kind) + ")"
			}
			got := NewMarkdownHandler().ExtractLinks([]byte(link), "docs/n.md")
			if len(got) != 1 || got[0] != "docs/"+rel {
				t.Errorf("%s reads %q, want docs/%s", link, got, rel)
			}
		}
	}
}
