package pdfexport

import (
	"bytes"
	"strings"

	"knov/internal/parser"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// ---------------------------------------------------------------------------
// inline tokenization — flattens an inline AST subtree into a stream of
// styled words that layout.go and table.go word-wrap onto the page.
// ---------------------------------------------------------------------------

// collectTokens walks n's inline children and returns them as styled words.
// Add a case here for any new inline construct (e.g. strikethrough); it only
// needs to know how to fold itself into the base style and recurse.
//
// Consecutive *ast.Text children are buffered and word-split together rather
// than one call to splitWords each: when an inline parser attempt fails (e.g.
// "[-]", which isn't a valid link or task checkbox), goldmark still splits
// the run into several Text nodes at the point it gave up, even though there
// was no whitespace there in the source. Splitting each node independently
// would insert a space at that seam; merging contiguous segments first
// (tracked via byte offsets) restores the original, space-free text.
func (r *renderer) collectTokens(n ast.Node, base style) []token {
	var toks []token
	var pending []byte
	pendingEnd := -1
	// cur starts as base but is folded to the resolved task-list state (see
	// *extast.TaskCheckBox below) so that everything after a checkbox in the
	// same list item picks up its strikethrough/italic/muted styling, exactly
	// like components.css applies li.todo-* to the whole item.
	cur := base

	flushPending := func() {
		if len(pending) > 0 {
			toks = append(toks, splitWords(string(pending), cur)...)
			pending = nil
		}
		pendingEnd = -1
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			seg := t.Segment
			if pendingEnd >= 0 && seg.Start != pendingEnd {
				flushPending()
			}
			pending = append(pending, seg.Value(r.source)...)
			pendingEnd = seg.Stop
			// A line break in the source (soft or hard — the web view
			// renders both as <br> via goldmark's WithHardWraps) must
			// survive into the pdf as a forced break, not collapse into
			// the space that separates ordinary word tokens.
			if t.SoftLineBreak() || t.HardLineBreak() {
				flushPending()
				toks = append(toks, token{breakLine: true})
			}
			continue
		}
		flushPending()

		switch v := c.(type) {
		case *ast.String:
			toks = append(toks, splitWords(string(v.Value), cur)...)
		case *ast.Emphasis:
			st := cur
			if v.Level >= 2 {
				st.bold = true
			} else {
				st.italic = true
			}
			toks = append(toks, r.collectTokens(c, st)...)
		case *ast.CodeSpan:
			st := cur
			st.code = true
			toks = append(toks, splitWords(plainText(c, r.source), st)...)
		case *ast.Link:
			st := cur
			st.link = true
			st.href = string(v.Destination)
			toks = append(toks, r.collectTokens(c, st)...)
		case *ast.AutoLink:
			st := cur
			st.link = true
			st.href = string(v.URL(r.source))
			toks = append(toks, token{text: st.href, style: st})
		case *ast.Image:
			toks = append(toks, splitWords("[image: "+plainText(c, r.source)+"]", cur)...)
		case *extast.TaskCheckBox:
			state := taskStateOpen
			if v.IsChecked {
				state = taskStateDone
			}
			// preprocessTodoStates rewrites the non-GFM cancelled/waiting
			// markers into a valid checkbox plus a leading text placeholder
			// (e.g. "[-]" -> "[x] KNOVTODO:cancelled "), so the real state
			// hides in the very next Text sibling here.
			rest := ""
			consumedNext := false
			next, ok := c.NextSibling().(*ast.Text)
			if ok {
				raw := string(next.Segment.Value(r.source))
				switch {
				case strings.HasPrefix(raw, parser.TodoCancelledPlaceholder):
					state = taskStateCancelled
					rest = raw[len(parser.TodoCancelledPlaceholder):]
					consumedNext = true
				case strings.HasPrefix(raw, parser.TodoWaitingPlaceholder):
					state = taskStateWaiting
					rest = raw[len(parser.TodoWaitingPlaceholder):]
					consumedNext = true
				}
			}
			if r.opts.UseTaskIcons && r.iconFontLoaded {
				glyph, color := taskIconGlyphAndColor(state)
				toks = append(toks, token{text: glyph, style: style{isIcon: true, size: cur.size, iconColor: color}})
			} else {
				toks = append(toks, token{text: taskFallbackMark(state), style: cur})
			}
			cur = taskTextStyle(cur, state)
			if consumedNext {
				toks = append(toks, splitWords(rest, cur)...)
				c = next
			}
		default:
			toks = append(toks, r.collectTokens(c, cur)...)
		}
	}
	flushPending()
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
