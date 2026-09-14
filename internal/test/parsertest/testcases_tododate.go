package parsertest

import (
	"fmt"
	"strings"

	"knov/internal/parser"
	"knov/internal/test"
)

// caseTodoDateNoDuplication covers wrapTrailingTodoDate (internal/parser/parser_todo.go),
// exercised end-to-end through MarkdownHandler.Render since it operates on the parsed AST,
// not on a string it's handed directly. Each case's trailing " (YYYY-MM-DD)" stamp must end
// up wrapped in exactly one "todo-date" span - never left as leftover plain text *and*
// duplicated into a styled span, which is the failure mode a broken overlap-resolution walk
// would silently produce instead of hitting the documented safe bail-out.
func caseTodoDateNoDuplication() test.CaseResult {
	name := "todo-date-no-duplication"

	type tc struct {
		label, in string
	}
	tcs := []tc{
		{"single-line", "- [ ] Buy milk (2024-01-15)"},
		{"cancelled-placeholder", "- [-] Task done (2024-01-15)"},
		{"waiting-placeholder", "- [O] Task waiting (2024-01-15)"},
		{"hyphenated-word-before-date", "- [ ] pre-order item - final (2024-01-15)"},
		{"multi-line-wrapped", "- [ ] first line\n  second line (2024-01-15)"},
	}

	const wantSpan = `<span class="todo-date">(2024-01-15)</span>`

	var failures []string
	for _, c := range tcs {
		out, err := parser.NewMarkdownHandler().Render([]byte(c.in), parser.PathlessRender)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: Render error: %v", c.label, err))
			continue
		}
		got := string(out)
		if n := strings.Count(got, "2024-01-15"); n != 1 {
			failures = append(failures, fmt.Sprintf("%s: date appears %d times, want 1: %s", c.label, n, got))
			continue
		}
		if !strings.Contains(got, wantSpan) {
			failures = append(failures, fmt.Sprintf("%s: date not wrapped in %s: %s", c.label, wantSpan, got))
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: "every case's trailing date stamp is wrapped in exactly one todo-date span",
		Actual:   fmt.Sprintf("%d/%d ok", len(tcs)-len(failures), len(tcs)),
		Success:  len(failures) == 0,
	}
	if len(failures) != 0 {
		cr.Error = fmt.Sprintf("%v", failures)
	}
	return cr
}
