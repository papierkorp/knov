package pdfexport

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// ---------------------------------------------------------------------------
// inline tokenization — flattens an inline AST subtree into a stream of
// styled words that layout.go and table.go word-wrap onto the page.
// ---------------------------------------------------------------------------

// collectTokens walks n's inline children and returns them as styled words.
// Add a case here for any new inline construct (e.g. strikethrough); it only
// needs to know how to fold itself into the base style and recurse.
func (r *renderer) collectTokens(n ast.Node, base style) []token {
	var toks []token
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			toks = append(toks, splitWords(string(v.Segment.Value(r.source)), base)...)
		case *ast.String:
			toks = append(toks, splitWords(string(v.Value), base)...)
		case *ast.Emphasis:
			st := base
			if v.Level >= 2 {
				st.bold = true
			} else {
				st.italic = true
			}
			toks = append(toks, r.collectTokens(c, st)...)
		case *ast.CodeSpan:
			st := base
			st.code = true
			toks = append(toks, splitWords(plainText(c, r.source), st)...)
		case *ast.Link:
			st := base
			st.link = true
			st.href = string(v.Destination)
			toks = append(toks, r.collectTokens(c, st)...)
		case *ast.AutoLink:
			st := base
			st.link = true
			st.href = string(v.URL(r.source))
			toks = append(toks, token{text: st.href, style: st})
		case *ast.Image:
			toks = append(toks, splitWords("[image: "+plainText(c, r.source)+"]", base)...)
		default:
			toks = append(toks, r.collectTokens(c, base)...)
		}
	}
	return toks
}

func splitWords(s string, st style) []token {
	fields := strings.Fields(s)
	toks := make([]token, 0, len(fields))
	for _, f := range fields {
		toks = append(toks, token{text: f, style: st})
	}
	return toks
}

func plainText(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		} else {
			buf.WriteString(plainText(c, source))
		}
	}
	return buf.String()
}
