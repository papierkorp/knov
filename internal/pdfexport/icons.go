package pdfexport

// iconFontFamily is the fpdf font family name the embedded Font Awesome
// Solid TrueType build is registered under (see SetIconFont).
const iconFontFamily = "FontAwesome"

// Font Awesome Solid glyph codepoints used for task-list status icons,
// matching the icons the app's HTML view uses for the same states (see
// parser_todo.go). Written as \u escapes rather than literal characters:
// these are Private Use Area codepoints with no visible glyph in most
// editors/fonts, so an embedded literal is effectively invisible, unreviewable
// in a diff, and at risk of mangling by any tool that isn't UTF-8-careful.
const (
	faCircle      = "\uf111" // fa-circle
	faCircleCheck = "\uf058" // fa-circle-check
	faCircleXmark = "\uf057" // fa-circle-xmark
	faClock       = "\uf017" // fa-clock
)

//nolint:gochecknoglobals
var iconFontData []byte

// SetIconFont registers the TrueType (glyf-outline) bytes of the Font Awesome
// Solid font used to draw task-list status icons in exported PDFs. It must be
// a TrueType build, not the CFF/OTTO one Font Awesome ships by default — fpdf
// only supports TrueType outlines. Call once at startup; if never called,
// task checkboxes fall back to plain-text marks ("[ ]", "[x]", "[-]", "[O]").
func SetIconFont(data []byte) {
	iconFontData = data
}

// taskState is the resolved state of a markdown task-list checkbox, including
// the two non-GFM states (cancelled, waiting) the app's editor supports on
// top of plain open/done (see parser.PreprocessTodoStates).
type taskState int

const (
	taskStateOpen taskState = iota
	taskStateDone
	taskStateCancelled
	taskStateWaiting
)

// taskIconGlyphAndColor returns the Font Awesome glyph and RGB color for
// state, matching themes/builtin/css/components.css's todo-state-* colors
// (light-theme values, since PDFs are always rendered on a white page).
func taskIconGlyphAndColor(state taskState) (string, [3]int) {
	switch state {
	case taskStateDone:
		return faCircleCheck, [3]int{22, 163, 74} // --success
	case taskStateCancelled:
		return faCircleXmark, [3]int{135, 141, 151} // --text-secondary @ ~0.6 opacity over white
	case taskStateWaiting:
		return faClock, [3]int{217, 119, 6} // --warning
	default:
		return faCircle, [3]int{175, 179, 185} // --text-secondary @ ~0.4 opacity over white
	}
}

// taskFallbackMark returns the plain-text marker used when no icon font was
// registered via SetIconFont.
func taskFallbackMark(state taskState) string {
	switch state {
	case taskStateDone:
		return "[x]"
	case taskStateCancelled:
		return "[-]"
	case taskStateWaiting:
		return "[O]"
	default:
		return "[ ]"
	}
}

// taskTextStyle folds a resolved task state into the text style used for the
// rest of the list item, mirroring components.css's li.todo-* rules: done and
// cancelled get a muted color plus strikethrough, waiting gets italics, open
// is unchanged.
func taskTextStyle(base style, state taskState) style {
	st := base
	switch state {
	case taskStateDone:
		st.strike = true
		st.textColor = [3]int{55, 65, 81} // --text-secondary
	case taskStateCancelled:
		st.strike = true
		st.textColor = [3]int{145, 150, 158} // --text-secondary, further muted
	case taskStateWaiting:
		st.italic = true
	}
	return st
}
