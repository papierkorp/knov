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
		for _, m := range []string{img, pdf} {
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
			"markdown file url":    "[x](" + pathutils.ToFileURL(p) + ")",
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
			"html href file url":   `<a href="` + pathutils.ToFileURL(p) + `">x</a>`,
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
