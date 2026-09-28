package configmanager

import (
	"strconv"
	"testing"
)

func TestValidateKanbanBoards(t *testing.T) {
	for _, bad := range []string{"a", "projects/work:", "projects/work: ", ":Name", "/:Name",
		"../x:X", "a/../../x:X", "..:X", ".:X", "a/../b:X", "a//b:X", "docs/x:X", "media/x:X", "files/x:X"} {
		if err := ValidateKanbanBoards([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := ValidateKanbanBoards([]string{"projects/work:A", "/projects/work/:B"}); err == nil {
		t.Error("duplicate board folder accepted")
	}
	if err := validateKanbanNames([]string{"inbox", "inbox"}); err == nil {
		t.Error("duplicate status accepted")
	}
	// a leading / is trimmed, so /personal/todo/ is the docs folder personal/todo, not an absolute path
	if err := ValidateKanbanBoards([]string{"projects/work:Work Board", "/personal/todo/:Todo"}); err != nil {
		t.Errorf("valid boards rejected: %v", err)
	}
}

func TestValidateKanbanTagColors(t *testing.T) {
	for _, bad := range []string{`urgent:red;x:y`, `urgent:"`, `urgent:red" onclick="x`, `urgent:`, `:red`, `urgent`, `urgent:url(x)`} {
		if err := ValidateKanbanTagColors([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := ValidateKanbanTagColors([]string{"urgent:red", "done:#2a2", "todo:var(--warning)"}); err != nil {
		t.Errorf("valid colors rejected: %v", err)
	}
}

func TestValidateKanbanCardStyles(t *testing.T) {
	if err := ValidateKanbanCardStyles([]string{"archive:bold"}); err == nil {
		t.Error("unknown style accepted")
	}
	if err := ValidateKanbanCardStyles([]string{"archive:deleted", "blocked:italic"}); err != nil {
		t.Errorf("valid styles rejected: %v", err)
	}
}

func TestParseKanbanBoards(t *testing.T) {
	boards := parseKanbanBoards(
		[]string{"projects/work:Work", "projects-work:Other", "/personal/todo/:Todo", "bad", "x:"},
		[]string{"personal/todo/"},
	)
	if len(boards) != 3 {
		t.Fatalf("got %d boards, want 3 (malformed entries skipped): %+v", len(boards), boards)
	}
	want := []KanbanBoard{
		{FolderPath: "projects/work", DisplayName: "Work", Slug: "projects-work"},
		{FolderPath: "projects-work", DisplayName: "Other", Slug: "projects-work-1"},
		{FolderPath: "personal/todo", DisplayName: "Todo", Slug: "personal-todo", FolderSync: true},
	}
	for i, w := range want {
		if boards[i] != w {
			t.Errorf("board %d = %+v, want %+v", i, boards[i], w)
		}
	}
}

// BulkSetFromForm is all-or-nothing: one invalid value keeps every other key unchanged too.
func TestBulkSetAllOrNothing(t *testing.T) {
	pageSize := GetSetting("pageSize").(*IntSetting)
	original := pageSize.Get()
	next := 50
	if original == next {
		next = 60
	}

	errs := BulkSetFromForm(map[string][]string{
		"pageSize":       {strconv.Itoa(next)},
		"kanbanStatuses": {"bad status!"},
	})

	if len(errs) == 0 {
		t.Fatal("invalid kanbanStatuses was not rejected")
	}
	if pageSize.Get() != original {
		t.Errorf("pageSize = %d after a rejected bulk update, want unchanged %d", pageSize.Get(), original)
	}
}
