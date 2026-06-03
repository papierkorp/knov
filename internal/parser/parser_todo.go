package parser

import (
	"fmt"
	"regexp"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
)

// renderTaskCheckBox renders GFM [ ] / [x] checkboxes as styled todo-state icons.
func (r *knovNodeRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.TaskCheckBox)
	if n.IsChecked {
		fmt.Fprintf(w, `<span class="todo-state todo-state-done"><i class="fa-solid fa-circle-check"></i></span> `)
	} else {
		fmt.Fprintf(w, `<span class="todo-state todo-state-open"><i class="fa-regular fa-circle"></i></span> `)
	}
	return ast.WalkContinue, nil
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
		`<li><span class="todo-state todo-state-done"><i class="fa-solid fa-circle-check"></i></span> KNOVTODO:cancelled ([^<]*)`,
	).ReplaceAllString(html,
		`<li class="todo-cancelled"><span class="todo-state todo-state-cancelled"><i class="fa-solid fa-circle-xmark"></i></span> $1`,
	)
	// waiting: was rendered as unchecked [ ] with KNOVTODO:waiting placeholder
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-open"><i class="fa-regular fa-circle"></i></span> KNOVTODO:waiting ([^<]*)`,
	).ReplaceAllString(html,
		`<li class="todo-waiting"><span class="todo-state todo-state-waiting"><i class="fa-regular fa-clock"></i></span> $1`,
	)
	// add state classes to remaining open/done items
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-done">`,
	).ReplaceAllString(html,
		`<li class="todo-done"><span class="todo-state todo-state-done">`,
	)
	html = regexp.MustCompile(
		`<li><span class="todo-state todo-state-open">`,
	).ReplaceAllString(html,
		`<li class="todo-open"><span class="todo-state todo-state-open">`,
	)
	return html
}
