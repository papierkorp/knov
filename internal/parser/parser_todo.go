package parser

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"knov/internal/configmanager"
)

// lineNumberForNode walks up to the nearest ancestor block node and resolves its
// source position to a 0-indexed line number within source.
func lineNumberForNode(n ast.Node, source []byte) int {
	for cur := n; cur != nil; cur = cur.Parent() {
		if cur.Type() != ast.TypeBlock {
			continue
		}
		lines := cur.Lines()
		if lines != nil && lines.Len() > 0 {
			seg := lines.At(0)
			return bytes.Count(source[:seg.Start], []byte("\n"))
		}
	}
	return -1
}

// todoStateIcons maps a todo state (stored on a ListItem node's todoStateAttr by
// todoStateTransformer) to its fontawesome icon class.
var todoStateIcons = map[string]string{
	"open":      "fa-circle",
	"done":      "fa-circle-check",
	"cancelled": "fa-circle-xmark",
	"waiting":   "fa-clock",
}

// todoStateAttr is the ListItem node attribute (set by todoStateTransformer, read by
// renderListItem and renderTaskCheckBox) holding a todo item's resolved state.
const todoStateAttr = "todoState"

// todoStateOf returns the todo state todoStateTransformer stored on a ListItem node, and
// whether it has one at all (i.e. whether it's a todo item).
func todoStateOf(listItem ast.Node) (string, bool) {
	if listItem == nil {
		return "", false
	}
	v, ok := listItem.AttributeString(todoStateAttr)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// listItemOf walks up from a task checkbox to its enclosing ListItem.
func listItemOf(n ast.Node) ast.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == ast.KindListItem {
			return p
		}
	}
	return nil
}

// renderTaskCheckBox renders GFM [ ] / [x] checkboxes as styled, clickable todo-state icons.
// data-line records the 0-indexed source line so the view can toggle state in place.
func (r *knovNodeRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	line := lineNumberForNode(node, source)
	state, _ := todoStateOf(listItemOf(node))
	fmt.Fprintf(w, `<span class="todo-state todo-state-%s" data-line="%d"><i class="fa-solid %s"></i></span> `,
		state, line, todoStateIcons[state])
	return ast.WalkContinue, nil
}

// renderListItem writes the <li> for every list item, tagging todo items with their
// state (already resolved by todoStateTransformer before rendering started) as a class.
func (r *knovNodeRenderer) renderListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		fmt.Fprintf(w, "</li>\n")
		return ast.WalkContinue, nil
	}

	block := node.FirstChild()
	if state, isTodo := todoStateOf(node); isTodo {
		fmt.Fprintf(w, `<li class="todo-%s"`, state)
	} else {
		_, _ = w.WriteString("<li")
	}
	if node.Attributes() != nil {
		html.RenderAttributes(w, node, html.ListItemAttributeFilter)
	}
	_ = w.WriteByte('>')
	if _, ok := block.(*ast.TextBlock); !ok {
		_, _ = w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}

// todoStateTransformer resolves every todo list item's state (open/done/cancelled/waiting)
// once, right after parsing and before any rendering: for each ListItem whose first block
// starts with a task checkbox, it reads the checkbox's checked flag and any KNOVTODO:
// placeholder left by PreprocessTodoStates, strips the placeholder, splits a trailing date
// stamp into its own node, and stores the resolved state as an attribute on the ListItem.
//
// AST transformers such as this one always run after inline parsing finishes, so the
// TaskCheckBox nodes goldmark's tasklist extension creates (via its own inline parser) are
// already in place by the time this runs. Resolving state here - rather than in
// renderListItem, with the result handed to renderTaskCheckBox through a field on the
// renderer - means each render func reads state straight off the node it's given, with no
// dependency on the order the renderer happens to visit nodes in.
type todoStateTransformer struct{}

func (todoStateTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindListItem {
			return ast.WalkContinue, nil
		}
		block := n.FirstChild()
		checkbox, isTodo := firstChildTaskCheckBox(block)
		if !isTodo {
			return ast.WalkContinue, nil
		}
		n.SetAttributeString(todoStateAttr, todoItemState(checkbox, source))
		stripTodoPlaceholder(checkbox, source)
		wrapTrailingTodoDate(block, source)
		return ast.WalkContinue, nil
	})
}

// firstChildTaskCheckBox reports whether a list item's first block (block) opens with a
// task checkbox, i.e. the item is a todo line.
func firstChildTaskCheckBox(block ast.Node) (*extast.TaskCheckBox, bool) {
	if block == nil {
		return nil, false
	}
	cb, ok := block.FirstChild().(*extast.TaskCheckBox)
	return cb, ok
}

// todoItemState resolves a checkbox's rendered state from its checked flag and the
// KNOVTODO: placeholder (if any) PreprocessTodoStates left in the following text node.
func todoItemState(checkbox *extast.TaskCheckBox, source []byte) string {
	placeholder := ""
	if t, ok := checkbox.NextSibling().(*ast.Text); ok {
		placeholder = todoPlaceholderPrefix(t, source)
	}
	switch {
	case checkbox.IsChecked && placeholder == TodoCancelledPlaceholder:
		return "cancelled"
	case !checkbox.IsChecked && placeholder == TodoWaitingPlaceholder:
		return "waiting"
	case checkbox.IsChecked:
		return "done"
	default:
		return "open"
	}
}

// todoPlaceholderPrefix returns the KNOVTODO placeholder t's text starts with, if any.
func todoPlaceholderPrefix(t *ast.Text, source []byte) string {
	val := t.Segment.Value(source)
	for _, p := range [...]string{TodoCancelledPlaceholder, TodoWaitingPlaceholder} {
		if bytes.HasPrefix(val, []byte(p)) {
			return p
		}
	}
	return ""
}

// stripTodoPlaceholder trims a leading KNOVTODO: placeholder off the text node right
// after checkbox, so it never reaches the renderer.
func stripTodoPlaceholder(checkbox *extast.TaskCheckBox, source []byte) {
	t, ok := checkbox.NextSibling().(*ast.Text)
	if !ok {
		return
	}
	if p := todoPlaceholderPrefix(t, source); p != "" {
		t.Segment = text.NewSegment(t.Segment.Start+len(p), t.Segment.Stop)
	}
}

// wrapTrailingTodoDate splits a trailing " (YYYY-MM-DD)" stamp off block's raw source text
// and appends a todoDateNode covering it, so renderTodoDate can wrap it in a styled span
// instead of leaving it as plain text.
//
// It locates the date in block's raw source line rather than in block.LastChild()'s text
// segment because goldmark's inline scanner breaks plain text into a new ast.Text node at
// every inline-trigger byte it sees - including a plain "-" (checked, and discarded, by the
// typographer extension's en/em-dash parser) - so "(2024-01-01)" alone commonly ends up
// fragmented across three or four sibling Text nodes instead of one.
func wrapTrailingTodoDate(block ast.Node, source []byte) {
	lines := block.Lines()
	if lines == nil || lines.Len() == 0 {
		return
	}
	tail := lines.At(lines.Len() - 1)
	// trailing whitespace left in the source after the date (easy for an editor to leave
	// behind) would otherwise defeat TodoDateRe's end-of-line anchor
	raw := strings.TrimRight(string(tail.Value(source)), " \t\r")
	loc := TodoDateRe.FindStringSubmatchIndex(raw)
	if loc == nil {
		return
	}
	date := raw[loc[2]:loc[3]]
	// loc[0] is the start of the \s* the regexp also swallows - keep that whitespace as
	// plain text (matches the space rendered before the pre-AST regex version's span) and
	// only replace from the opening paren on
	dateStart := tail.Start + loc[0] + strings.IndexByte(raw[loc[0]:loc[1]], '(')

	// plan first, mutate second: walk back read-only to find every trailing node covering
	// [dateStart, end). If a non-text node is in the way, bail out untouched rather than
	// leaving the date's original text in place *and* appending a second, styled copy of
	// it - some node types (e.g. emphasis) don't expose a source Segment to check, so
	// there's no way to safely resolve the overlap.
	var toRemove []ast.Node
	var truncate *ast.Text
	for c := block.LastChild(); c != nil; c = c.PreviousSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			return
		}
		if t.Segment.Stop <= dateStart {
			break
		}
		if t.Segment.Start >= dateStart {
			toRemove = append(toRemove, c)
			continue
		}
		truncate = t
		break
	}

	for _, c := range toRemove {
		block.RemoveChild(block, c)
	}
	if truncate != nil {
		truncate.Segment = text.NewSegment(truncate.Segment.Start, dateStart)
	}
	block.AppendChild(block, newTodoDateNode(date))
}

// kindTodoDate is the NodeKind for todoDateNode.
var kindTodoDate = ast.NewNodeKind("KnovTodoDate")

// todoDateNode is a custom inline node standing in for a todo item's trailing date stamp,
// inserted by wrapTrailingTodoDate and rendered by renderTodoDate.
type todoDateNode struct {
	ast.BaseInline
	Date string
}

func newTodoDateNode(date string) *todoDateNode {
	return &todoDateNode{Date: date}
}

func (n *todoDateNode) Kind() ast.NodeKind { return kindTodoDate }

func (n *todoDateNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Date": n.Date}, nil)
}

// renderTodoDate writes the de-emphasized date span and its clear button for a todo
// item's date stamp (see wrapTrailingTodoDate).
func (r *knovNodeRenderer) renderTodoDate(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*todoDateNode)
	fmt.Fprintf(w, `<span class="todo-date">(%s)</span><button type="button" class="todo-date-clear">&times;</button>`, n.Date)
	return ast.WalkContinue, nil
}

// todoCheckboxLineRe matches a GFM checkbox list item prefix, capturing the marker char.
var todoCheckboxLineRe = regexp.MustCompile(`^([ \t]*)[-*+] \[([ xX\-Oo])\] `)

// todoMarkerCycle is the open -> done -> cancelled -> waiting -> open cycle order.
var todoMarkerCycle = []byte{' ', 'X', '-', 'O'}

// TodoDateRe matches a trailing " (YYYY-MM-DD)" date stamp on a todo line/item, as
// appended by StampTodoDate, so it can be split back out or replaced instead of
// accumulating. Exported so the list/todo editor (render.ParseMarkdownToListItems)
// shares the same format instead of keeping its own copy.
var TodoDateRe = regexp.MustCompile(`\s*\((\d{4}-\d{2}-\d{2})\)\r?$`)

// SplitTodoDate splits a trailing " (YYYY-MM-DD)" stamp off text, returning the
// remaining text and the date (empty if none present).
func SplitTodoDate(text string) (string, string) {
	loc := TodoDateRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return text, ""
	}
	return text[:loc[0]], text[loc[2]:loc[3]]
}

// TodayStamp returns today's date in the app's configured timezone, formatted as the
// canonical "YYYY-MM-DD" stamp used by StampTodoDate/TodoDateRe. This is the single
// source of truth for "today" across every todo-editing surface (raw editor, rendered
// file view, list/todo editor) so they can never disagree with each other or with the
// browser's local clock/timezone.
func TodayStamp() string {
	return time.Now().In(configmanager.GetTimezone()).Format("2006-01-02")
}

// StampTodoDate replaces any trailing date stamp on text with today's date, so repeated
// toggles never accumulate more than the latest one. Also returns the date applied.
func StampTodoDate(text string) (string, string) {
	text, _ = SplitTodoDate(text)
	date := TodayStamp()
	return text + " (" + date + ")", date
}

func todoMarkerIndex(marker byte) int {
	switch marker {
	case 'x', 'X':
		return 1
	case '-':
		return 2
	case 'o', 'O':
		return 3
	default:
		return 0
	}
}

// CycleTodoStateAtLine advances the checkbox state on the given 0-indexed line
// (open -> done -> cancelled -> waiting -> open) and hands the new state down to all
// nested descendant checkboxes, mirroring the todo editor's cascade behavior. Returns
// the updated content and the date stamp applied to the line (empty if the
// "stamp date on toggle" setting is off).
func CycleTodoStateAtLine(content []byte, line int) ([]byte, string, error) {
	lines := strings.Split(string(content), "\n")
	if line < 0 || line >= len(lines) {
		return nil, "", fmt.Errorf("line %d out of range", line)
	}

	loc := todoCheckboxLineRe.FindStringSubmatchIndex(lines[line])
	if loc == nil {
		return nil, "", fmt.Errorf("line %d is not a todo item", line)
	}

	indent := loc[3] - loc[2]
	markerStart, markerEnd := loc[4], loc[5]
	next := todoMarkerCycle[(todoMarkerIndex(lines[line][markerStart])+1)%len(todoMarkerCycle)]
	lines[line] = lines[line][:markerStart] + string(next) + lines[line][markerEnd:]

	date := ""
	if configmanager.TodoStampDate.Get() {
		lines[line], date = StampTodoDate(lines[line])
	}

	// cascade to nested descendants: deeper-indented checkbox lines immediately following,
	// stopping at the first line back at or above the original indentation.
	for i := line + 1; i < len(lines); i++ {
		childLoc := todoCheckboxLineRe.FindStringSubmatchIndex(lines[i])
		if childLoc == nil {
			if strings.TrimSpace(lines[i]) == "" {
				continue
			}
			break
		}
		childIndent := childLoc[3] - childLoc[2]
		if childIndent <= indent {
			break
		}
		childMarkerStart, childMarkerEnd := childLoc[4], childLoc[5]
		lines[i] = lines[i][:childMarkerStart] + string(next) + lines[i][childMarkerEnd:]
	}

	return []byte(strings.Join(lines, "\n")), date, nil
}

// ClearTodoDateAtLine removes any trailing date stamp from the checkbox on the given
// 0-indexed line without changing its state. Returns the updated content.
func ClearTodoDateAtLine(content []byte, line int) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	if line < 0 || line >= len(lines) {
		return nil, fmt.Errorf("line %d out of range", line)
	}
	if todoCheckboxLineRe.FindStringSubmatchIndex(lines[line]) == nil {
		return nil, fmt.Errorf("line %d is not a todo item", line)
	}
	lines[line], _ = SplitTodoDate(lines[line])
	return []byte(strings.Join(lines, "\n")), nil
}

// TodoCancelledPlaceholder and TodoWaitingPlaceholder mark the non-GFM todo
// states after PreprocessTodoStates rewrites them into a valid checkbox plus
// a leading text marker. Consumers that walk the parsed AST directly (e.g.
// pdfexport) can detect these markers to recover the real state.
const (
	TodoCancelledPlaceholder = "KNOVTODO:cancelled "
	TodoWaitingPlaceholder   = "KNOVTODO:waiting "
)

// PreprocessTodoStates rewrites non-GFM todo states ([-] cancelled, [O] waiting)
// into standard GFM task items with a placeholder so goldmark parses them as list items.
// The placeholder is resolved back into the real state by renderListItem/renderTaskCheckBox
// at render time, or detected directly by other consumers walking the AST (e.g. pdfexport).
func PreprocessTodoStates(content []byte) []byte {
	s := string(content)
	// replace - [-] / * [-] / + [-] with <marker> [x] KNOVTODO:cancelled
	s = regexp.MustCompile(`(?m)^([ \t]*)([-*+]) \[-\] `).ReplaceAllString(s, "$1$2 [x] "+TodoCancelledPlaceholder)
	// replace - [O] / - [o] (and *, +) with <marker> [ ] KNOVTODO:waiting
	s = regexp.MustCompile(`(?mi)^([ \t]*)([-*+]) \[O\] `).ReplaceAllString(s, "$1$2 [ ] "+TodoWaitingPlaceholder)
	return []byte(s)
}
