package parser

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
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
		fmt.Fprintf(w, `<span class="todo-state todo-state-open" data-line="%d"><i class="fa-regular fa-circle"></i></span> `, line)
	}
	return ast.WalkContinue, nil
}

// todoCheckboxLineRe matches a GFM checkbox list item prefix, capturing the marker char.
var todoCheckboxLineRe = regexp.MustCompile(`^([ \t]*)- \[([ xX\-Oo])\] `)

// todoMarkerCycle is the open -> done -> cancelled -> waiting -> open cycle order.
var todoMarkerCycle = []byte{' ', 'X', '-', 'O'}

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
// the updated content.
func CycleTodoStateAtLine(content []byte, line int) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	if line < 0 || line >= len(lines) {
		return nil, fmt.Errorf("line %d out of range", line)
	}

	loc := todoCheckboxLineRe.FindStringSubmatchIndex(lines[line])
	if loc == nil {
		return nil, fmt.Errorf("line %d is not a todo item", line)
	}

	indent := loc[3] - loc[2]
	markerStart, markerEnd := loc[4], loc[5]
	next := todoMarkerCycle[(todoMarkerIndex(lines[line][markerStart])+1)%len(todoMarkerCycle)]
	lines[line] = lines[line][:markerStart] + string(next) + lines[line][markerEnd:]

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

	return []byte(strings.Join(lines, "\n")), nil
}

// preprocessTodoStates rewrites non-GFM todo states ([-] cancelled, [O] waiting)
// into standard GFM task items with a placeholder so goldmark parses them as list items.
// The placeholders are resolved in postprocessTodoStates.
func (h *MarkdownHandler) preprocessTodoStates(content []byte) []byte {
	s := string(content)
	// replace - [-] with - [x] KNOVTODO:cancelled
	s = regexp.MustCompile(`(?m)^([ \t]*)- \[-\] `).ReplaceAllString(s, "$1- [x] KNOVTODO:cancelled ")
	// replace - [O] / - [o] with - [ ] KNOVTODO:waiting
	s = regexp.MustCompile(`(?mi)^([ \t]*)- \[O\] `).ReplaceAllString(s, "$1- [ ] KNOVTODO:waiting ")
	return []byte(s)
}

// postprocessTodoStates replaces KNOVTODO placeholders in rendered HTML with
// proper todo-state icons and adds state classes to their parent <li>.
func (h *MarkdownHandler) postprocessTodoStates(html string) string {
	// cancelled: was rendered as checked [x] with KNOVTODO:cancelled placeholder
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-done" (data-line="\d+")><i class="fa-solid fa-circle-check"></i></span> KNOVTODO:cancelled ([^<]*)`,
	).ReplaceAllString(html,
		`<li class="todo-cancelled"><span class="todo-state todo-state-cancelled" $1><i class="fa-solid fa-circle-xmark"></i></span> $2`,
	)
	// waiting: was rendered as unchecked [ ] with KNOVTODO:waiting placeholder
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-open" (data-line="\d+")><i class="fa-regular fa-circle"></i></span> KNOVTODO:waiting ([^<]*)`,
	).ReplaceAllString(html,
		`<li class="todo-waiting"><span class="todo-state todo-state-waiting" $1><i class="fa-regular fa-clock"></i></span> $2`,
	)
	// add state classes to remaining open/done items
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-done" `,
	).ReplaceAllString(html,
		`<li class="todo-done"><span class="todo-state todo-state-done" `,
	)
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-open" `,
	).ReplaceAllString(html,
		`<li class="todo-open"><span class="todo-state todo-state-open" `,
	)
	return html
}
