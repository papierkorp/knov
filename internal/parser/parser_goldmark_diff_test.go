package parser

import (
	"fmt"
	"math/rand"
	"net/url"
	"slices"
	"strings"
	"testing"

	"knov/internal/test/specialchars"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// normalizeDest is a goldmark destination as the browser requests it, as file path: escapes and
// entities resolved like goldmark renders it (util.URLEscape), without query and anchor,
// percent-decoded once.
func normalizeDest(dest string) string {
	dest = string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations([]byte(dest)))))
	if i := strings.IndexAny(dest, "?#"); i != -1 {
		dest = dest[:i]
	}
	if decoded, err := url.PathUnescape(dest); err == nil {
		return decoded
	}
	return dest
}

// goldmarkLinks returns the path of every link and image goldmark's AST finds in content, sorted.
func goldmarkLinks(content string) []string {
	src := []byte(content)
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(src))
	var links []string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n := n.(type) {
			case *ast.Link:
				links = append(links, normalizeDest(string(n.Destination)))
			case *ast.Image:
				links = append(links, normalizeDest(string(n.Destination)))
			}
		}
		return ast.WalkContinue, nil
	})
	slices.Sort(links)
	return links
}

// scannerLinks returns the path of every markdown link and reference definition the link walker
// finds in content, sorted.
func scannerLinks(content string) []string {
	var links []string
	walkLinks(content, func(m linkMatch) string {
		if m.Link.Kind == LinkMarkdown {
			p := m.Link.Path
			if m.Link.External {
				p = normalizeDest(p)
			}
			links = append(links, p)
		}
		return m.whole()
	})
	slices.Sort(links)
	return links
}

// the link walker reads markdown links like goldmark, which renders them - for the special-char
// corpus and tricky markdown. a reference definition is used exactly once in every input, so it
// is one goldmark link. wikilinks aren't markdown (RenderLinks turns them into markdown links
// first). knownDivergences still read differently - remove one once it's fixed.
func TestScannerMatchesGoldmark(t *testing.T) {
	inputs := []string{
		"[x](a.md) ![y](b.png) [z](https://example.com/a?b#c)",
		"[a [b] c](d.md)",
		"[![i](i.png)](d.md)",
		"[a\nb](d.md)",
		"*[x](a.md)* **[y](b.md)**",
		"`[x](a.md)` [y](b.md) `` a ` [z](c.md) ``",
		"```\n[x](a.md)\n```\n[y](b.md)",
		"[x](<a b.md>) [y](<c d.md> \"t\") [z](<e.md#My Sec>)",
		"[x]( a.md ) [y](a.md 't') [z](a(b).md) [w](<>)",
		"[x](a.md)(b) [y] (c.md)",
		"[r][id] and [s][]\n\n[id]: a.md \"t\"\n[s]: <b c.md>",
		"| a | b |\n|---|---|\n| [x](a.md) | ![y](b.png) |",
		`[x](a\_b.md) [y](a&amp;b.md) [z](a%20b.md)`,
		"[x](\na.md)",
		"[x](a.md\n\"title\")",
		"[r][id]\n\n[id]:\n  a.md",
		`\[x](a.md)`,
		"    [x](a.md)",
		"<!-- [x](a.md) -->",
		"[x](a`b`.md) [y](`c.md`)",
		"[x](a.md \"t\") [z](https://x.y/a \"t\")",
	}
	// more block and inline contexts: code blocks inside and outside lists, a lazy paragraph
	// continuation, html comments, escapes and next-line destinations
	inputs = append(inputs,
		"text\n\n    [x](a.md)\n\n[y](b.md)",
		"    [x](a.md)\n    [y](b.md)\n[z](c.md)",
		"para\n    [x](a.md)",
		"- item\n\n      [x](a.md)\n\n  [y](b.md)",
		"- item\n\n      code\n\n        [x](a.md)",
		"1. item\n   [x](a.md)\n\n       [y](b.md)",
		"\t[x](a.md)\n\n[y](b.md)",
		"> quote\n>\n>     [x](a.md)",
		"a <!-- [x](a.md) --> [y](b.md)",
		"<!--\n[x](a.md)\n-->\n[y](b.md)",
		"<!-- [x](a.md)",
		"\\[x](a.md) [y](b.md)",
		"\\\\[x](a.md)",
		"[x](\n  a.md\n  \"t\")",
		"[x](a.md\n 'title')",
		"[x](<a b.md>\n\"t\")",
		"[x](\n\na.md)",
		"[r][id]\n\n[id]:\n  <a b.md>\n  \"t\"",
		"[r][id]\n\n[id]:\n    a.md",
		"[x](a`b`.md) [y](`c.md`) `d` [z](e.md)",
		"`a` [x](b`.md) ` c.md`",
		// raw html blocks (the app shows them as code, goldmark omits them), markdown after them is read
		"<div>\n[x](a.md)\n</div>\n\n[y](b.md)",
		"<table>\n<tr><td>[x](a.md)</td></tr>\n</table>\n\n[y](b.md)",
		"<p>[x](a.md)</p>\n\n[y](b.md)",
		"<pre>\n[x](a.md)\n</pre>\n\n[y](b.md)",
		"text\n<div>[x](a.md)</div>\n\n[y](b.md)",
		"<a href=\"h.md\">z</a> [x](a.md)",
		// a thematic break or setext underline ends a paragraph, an indented line after it is code
		"---\n    [x](a.md)\n\n[y](b.md)",
		"***\n    [x](a.md)\n\n[y](b.md)",
		"text\n===\n    [x](a.md)\n\n[y](b.md)",
		"text\n---\n    [x](a.md)\n\n[y](b.md)",
		// a block comment runs to the end of the line holding its end, across blank lines; an inline one stays in its paragraph
		"<!--  -->[x](a.md)\n\n[y](b.md)",
		"<!--\n\n[x](a.md)\n-->\n[y](b.md)",
		"# <!-- \n\n[x](a.md) -->\n\n[y](b.md)",
		"a <!-- \n\n[x](a.md) -->",
		"- a\n    - b [x](a.md)\n        - c [y](b.md)",
		"1. step\n\n    ![img](pic.png)\n\n2. next\n\n   [z](c.md)",
		"* a\n\n\t[x](a.md)\n\n\t\t[y](b.md)",
		"# head\n    [x](a.md)",
		"> - quote list\n>\n>       [x](a.md)\n> [y](b.md)",
		"term\n: def\n\n    [x](a.md)",
		"- a\n- b\n\n    [x](a.md)\n\n[y](b.md)",
		// tab-indented and outdented list items: a nested item is a list item, not indented code
		"- a\n\n\t- b\n\n\t\t- c [x](c.md)",
		"- a\n\t- b [x](a.md)\n\t\t- c [y](b.md)",
		"- a\n  - b\n    - c\n\n  - d [x](a.md)\n\n- e [y](b.md)",
		"- a\n\n  - b\n\n    - c\n\n- d [x](a.md)",
		"- a\n    - b\n\n        - c [x](a.md)\n\n    - d [y](b.md)",
		"1.\ta\n\n\t- b [x](a.md)\n\n\t\t[y](b.md)",
		"-\ta\n\n\t\t[x](a.md)\n\n[y](b.md)",
		"- a\n\n\t\t\tcode\n\n\t- b [x](a.md)",
		"> - a\n>\n>   - b\n>\n>     - c [x](a.md)",
		// a tab after ">", a list marker followed by 5+ spaces (the item starts with indented code)
		">\t[x](a.md)", "> \t[x](a.md)", ">\t\t[x](a.md)",
		"1.     [l](a.md)", "-     [l](a.md)", "- a\n\n  1.     [l](a.md)", "-    [l](a.md)", "-   [l](a.md)",
		// a numeric entity in a path, an orphan "](" before a link
		"[x](&#120;.md)", "[x](&#x78;.md)", "[x](a&#35;b.md) [y](c&#63;d.md)", "[x](a&amp;b.md#s)",
		"]([k](a.md))", "x]([k](a.md))", "[a]([k](a.md))", "[a [b] c](d.md)", "a\n\n]([k](a.md))",
	)
	// the app shows these as it scans them (markdown.scanFences is shared by the renderer's code block
	// extraction), goldmark alone reads them differently
	knownDivergences := map[string]string{
		"    ```\n[x](a.md)": "an indented fence marker is a fence for the app's scanner, indented code for goldmark",
		"    ~~~\n[x](a.md)": "an indented fence marker is a fence for the app's scanner, indented code for goldmark",
	}
	// an html comment opened after a list marker or ">" is a block comment that ends with its container, the scanner only
	// knows the ones starting a line
	for _, in := range []string{"- <!-- [x](a.md)\n\n[y](b.md)", "> <!-- [x](a.md)\n\n[y](b.md)", "- <!-- x -->[y](b.md)"} {
		knownDivergences[in] = "a block html comment inside a list item or quote isn't masked"
		inputs = append(inputs, in)
	}
	inputs = append(inputs, "    ```\n[x](a.md)", "    ~~~\n[x](a.md)")
	for _, p := range specialchars.Names {
		if strings.Contains(p, `\`) {
			continue // a "\" is a windows separator for knov, goldmark keeps it
		}
		md := encodeLinkPath(p, LinkMarkdown)
		inputs = append(inputs, "[x]("+md+")", "[x](<"+md+">)", "![x]("+md+")", "[x]("+md+"#sec \"t\")", "[r][id]\n\n[id]: "+md)
	}
	for _, in := range inputs {
		got, want := scannerLinks(in), goldmarkLinks(in)
		reason, known := knownDivergences[in]
		switch {
		case known && slices.Equal(got, want):
			t.Errorf("%q (%s) reads like goldmark now, remove it from knownDivergences", in, reason)
		case !known && !slices.Equal(got, want):
			t.Errorf("%q: link walker reads %q, goldmark %q", in, got, want)
		}
	}
}

// nested lists built from spaces, tabs, blank lines and ">" read the same for the scanner and goldmark.
// seeded and bounded so it runs in a plain go test - a failing input is printed, add it to the cases above.
func TestScannerMatchesGoldmarkNestedLists(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	markers := []string{"- ", "* ", "+ ", "1. ", "2) "}
	indents := []string{"", " ", "  ", "   ", "    ", "      ", "\t", "\t\t"}
	for n := 0; n < 3000; n++ {
		var b strings.Builder
		quoted := rng.Intn(6) == 0
		for i, lines := 0, 2+rng.Intn(6); i < lines; i++ {
			if rng.Intn(3) == 0 {
				if quoted {
					b.WriteString(">")
				}
				b.WriteString("\n")
			}
			if quoted {
				b.WriteString("> ")
			}
			indent := indents[rng.Intn(len(indents))]
			if quoted {
				indent = strings.ReplaceAll(indent, "\t", "    ") // goldmark counts a tab inside a quote differently
			}
			b.WriteString(indent)
			if rng.Intn(4) != 0 {
				b.WriteString(markers[rng.Intn(len(markers))])
			}
			fmt.Fprintf(&b, "t [x](l%d.md)\n", i)
		}
		in := b.String()
		if got, want := scannerLinks(in), goldmarkLinks(in); !slices.Equal(got, want) {
			t.Fatalf("%q: link walker reads %q, goldmark %q", in, got, want)
		}
	}
}
