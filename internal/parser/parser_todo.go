package parser

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
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

// renderTaskCheckBox renders GFM [ ] / [x] checkboxes as styled, clickable todo-state icons.
// data-line records the 0-indexed source line so the view can toggle state in place.
func (r *knovNodeRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.TaskCheckBox)
	line := lineNumberForNode(node, source)
	if n.IsChecked {
		fmt.Fprintf(w, `<span class="todo-state todo-state-done" data-line="%d"><i class="fa-solid fa-circle-check"></i></span> `, line)
	} else {
		fmt.Fprintf(w, `<span class="todo-state todo-state-open" data-line="%d"><i class="fa-solid fa-circle"></i></span> `, line)
	}
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

// patterns used by postprocessTodoStates to resolve KNOVTODO placeholders and tag
// <li> elements with todo-state classes in the rendered HTML. Compiled once at package
// init instead of per-render.
var (
	todoCancelledHTMLRe = regexp.MustCompile(
		`<li><span class="todo-state todo-state-done" (data-line="\d+")><i class="fa-solid fa-circle-check"></i></span> KNOVTODO:cancelled ([^<]*)`,
	)
	todoWaitingHTMLRe = regexp.MustCompile(
		`<li><span class="todo-state todo-state-open" (data-line="\d+")><i class="fa-solid fa-circle"></i></span> KNOVTODO:waiting ([^<]*)`,
	)
	todoDoneHTMLRe = regexp.MustCompile(`<li><span class="todo-state todo-state-done" `)
	todoOpenHTMLRe = regexp.MustCompile(`<li><span class="todo-state todo-state-open" `)
	// wraps a trailing "(YYYY-MM-DD)" date stamp in a low-emphasis span for styling - scoped
	// to <li> elements already tagged with a todo-state class above, so it can never mistake
	// coincidental trailing text on a plain (non-todo) list item for a stamp
	todoDateHTMLRe = regexp.MustCompile(`(<li class="todo-(?:open|done|cancelled|waiting)">.*?)(\(\d{4}-\d{2}-\d{2}\))(\s*(?:</li>|<ul))`)
)

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
// The placeholders are resolved in postprocessTodoStates (HTML) or detected directly
// by other consumers walking the AST (e.g. pdfexport).
func PreprocessTodoStates(content []byte) []byte {
	s := string(content)
	// replace - [-] / * [-] / + [-] with <marker> [x] KNOVTODO:cancelled
	s = regexp.MustCompile(`(?m)^([ \t]*)([-*+]) \[-\] `).ReplaceAllString(s, "$1$2 [x] "+TodoCancelledPlaceholder)
	// replace - [O] / - [o] (and *, +) with <marker> [ ] KNOVTODO:waiting
	s = regexp.MustCompile(`(?mi)^([ \t]*)([-*+]) \[O\] `).ReplaceAllString(s, "$1$2 [ ] "+TodoWaitingPlaceholder)
	return []byte(s)
}

// postprocessTodoStates replaces KNOVTODO placeholders in rendered HTML with
// proper todo-state icons and adds state classes to their parent <li>. Every
// replacement is <li>/<span>/<i> only, so it can never reintroduce a live
// <h1-6> after the renderer has assigned heading ids.
func (h *MarkdownHandler) postprocessTodoStates(html string) string {
	// cancelled: was rendered as checked [x] with KNOVTODO:cancelled placeholder
	html = todoCancelledHTMLRe.ReplaceAllString(html,
		`<li class="todo-cancelled"><span class="todo-state todo-state-cancelled" $1><i class="fa-solid fa-circle-xmark"></i></span> $2`,
	)
	// waiting: was rendered as unchecked [ ] with KNOVTODO:waiting placeholder
	html = todoWaitingHTMLRe.ReplaceAllString(html,
		`<li class="todo-waiting"><span class="todo-state todo-state-waiting" $1><i class="fa-solid fa-clock"></i></span> $2`,
	)
	// add state classes to remaining open/done items
	html = todoDoneHTMLRe.ReplaceAllString(html, `<li class="todo-done"><span class="todo-state todo-state-done" `)
	html = todoOpenHTMLRe.ReplaceAllString(html, `<li class="todo-open"><span class="todo-state todo-state-open" `)
	html = todoDateHTMLRe.ReplaceAllString(html,
		`$1<span class="todo-date">$2</span><button type="button" class="todo-date-clear">&times;</button>$3`,
	)
	return html
}
