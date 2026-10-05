package configmanager

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestValidateKanbanBoards(t *testing.T) {
	for _, bad := range []string{"a", "projects/work:", "projects/work: ", ":Name", "/:Name",
		"../x:X", "a/../../x:X", "..:X", ".:X", "a/../b:X", "a//b:X", "docs/x:X", "media/x:X", "files/x:X", `C:\x:X`, "c:/x:X"} {
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

func TestValidateAutoCreateTags(t *testing.T) {
	for _, bad := range []string{`C:\x:tag`, `C:\x`, "c:/x:tag", "projects:"} {
		if err := ValidateAutoCreateTags([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := ValidateAutoCreateTags([]string{"todo", "projects/work:work"}); err != nil {
		t.Errorf("valid tags rejected: %v", err)
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
		[]string{"projects/work:Work", "projects-work:Other", "/personal/todo/:Todo", "bad", "x:", `team\docs:Team`},
		[]string{"personal/todo/"},
	)
	if len(boards) != 4 {
		t.Fatalf("got %d boards, want 4 (malformed entries skipped): %+v", len(boards), boards)
	}
	want := []KanbanBoard{
		{FolderPath: "projects/work", DisplayName: "Work", Slug: "projects-work"},
		{FolderPath: "projects-work", DisplayName: "Other", Slug: "projects-work-1"},
		{FolderPath: "personal/todo", DisplayName: "Todo", Slug: "personal-todo", FolderSync: true},
		{FolderPath: "team/docs", DisplayName: "Team", Slug: "team-docs"},
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

func TestRenameKanbanStatus(t *testing.T) {
	settings := []*StringSliceSetting{KanbanStatuses, KanbanColumns, KanbanAncestorAllowedStatus, KanbanCardStyles, KanbanTagColors, AutoCreateTags, HideFilesByTag}
	for _, s := range settings {
		defer s.store(s.Get())
	}
	defer KanbanArchiveStatus.store(KanbanArchiveStatus.Get())

	tag := KanbanStatusTag("inbox")
	errs := BulkSetFromForm(map[string][]string{
		"kanbanStatuses":              {"inbox,done"},
		"kanbanColumns":               {"inbox"},
		"kanbanArchiveStatus":         {"inbox"},
		"kanbanAncestorAllowedStatus": {"inbox,done"},
		"kanbanCardStyles":            {"inbox:italic,done:deleted"},
		"kanbanTagColors":             {tag + ":red,urgent:orange"},
		"autoCreateTags":              {tag + ",projects:" + tag + ",starred"},
		"hideFilesByTag":              {strings.ToUpper(tag) + "::kanban," + tag + "*,private"},
	})
	if len(errs) > 0 {
		t.Fatalf("setup failed: %v", errs)
	}

	for _, bad := range [][2]string{{"missing", "x"}, {"inbox", "inbox"}, {"inbox", "done"}, {"inbox", "bad name"}} {
		if err := RenameKanbanStatus(bad[0], bad[1]); err == nil {
			t.Errorf("rename %q -> %q accepted", bad[0], bad[1])
		}
	}

	if err := RenameKanbanStatus("inbox", "todo"); err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	newTag := KanbanStatusTag("todo")
	want := map[string][]string{
		"statuses":   {"todo", "done"},
		"columns":    {"todo"},
		"ancestor":   {"todo", "done"},
		"cardStyles": {"todo:italic", "done:deleted"},
		"tagColors":  {newTag + ":red", "urgent:orange"},
		"autoCreate": {newTag, "projects:" + newTag, "starred"},
		"hide":       {newTag + "::kanban", tag + "*", "private"},
	}
	got := map[string][]string{
		"statuses":   KanbanStatuses.Get(),
		"columns":    KanbanColumns.Get(),
		"ancestor":   KanbanAncestorAllowedStatus.Get(),
		"cardStyles": KanbanCardStyles.Get(),
		"tagColors":  KanbanTagColors.Get(),
		"autoCreate": AutoCreateTags.Get(),
		"hide":       HideFilesByTag.Get(),
	}
	for k := range want {
		if !slices.Equal(got[k], want[k]) {
			t.Errorf("%s = %v, want %v", k, got[k], want[k])
		}
	}
	if a := KanbanArchiveStatus.Get(); a != "todo" {
		t.Errorf("archive status = %q, want todo", a)
	}

	// the settings step runs last, so once it ran the old name is unknown
	if err := RenameKanbanStatus("inbox", "todo"); err == nil {
		t.Error("second rename of the already renamed status accepted")
	}
}
