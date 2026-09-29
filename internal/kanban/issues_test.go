package kanban

import (
	"os"
	"slices"
	"testing"

	"knov/internal/configStorage"
	"knov/internal/configmanager"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-kanban-test")
	if err != nil {
		panic(err)
	}
	if err := configStorage.Init("json", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestFileIssues(t *testing.T) {
	for s, v := range map[configmanager.StorableSetting]string{
		configmanager.KanbanStatuses:   "todo,done",
		configmanager.KanbanBoards:     "sync:Sync,plain:Plain",
		configmanager.KanbanFolderSync: "sync",
	} {
		if err := configmanager.SetSetting(s, v); err != nil {
			t.Fatalf("failed to set %s: %v", v, err)
		}
	}
	prefix := configmanager.GetKanbanPrefix()
	todo, done, gone := prefix+"-status-todo", prefix+"-status-done", prefix+"-status-gone"

	for _, tc := range []struct {
		name, dir string
		tags      []string
		kinds     []IssueKind
		fix       string
	}{
		{"ok", "plain", []string{"a", todo}, nil, ""},
		{"no kanban tags", "other", []string{"a"}, nil, ""},
		{"multiple", "plain", []string{gone, todo, done}, []IssueKind{IssueMultipleStatus}, "todo"},
		{"multiple first valid", "plain", []string{done, todo}, []IssueKind{IssueMultipleStatus}, "done"},
		{"multiple duplicate", "plain", []string{todo, todo}, []IssueKind{IssueMultipleStatus}, "todo"},
		{"multiple none valid", "plain", []string{gone, prefix + "-status-x"}, []IssueKind{IssueMultipleStatus}, ""},
		{"unknown status", "plain", []string{gone}, []IssueKind{IssueUnknownStatus}, ""},
		{"no board", "other", []string{todo}, []IssueKind{IssueNoBoard}, ""},
		{"subfolder of board", "plain/sub", []string{todo}, nil, ""},
		{"folder mismatch", "sync/done", []string{todo}, []IssueKind{IssueFolderMismatch}, "done"},
		{"folder without tag", "sync/done", nil, []IssueKind{IssueFolderMismatch}, "done"},
		{"folder duplicate", "sync/done", []string{done, done}, []IssueKind{IssueFolderMismatch}, "done"},
		{"folder ok", "sync/done", []string{done}, nil, ""},
		{"folder not a status", "sync/other", []string{todo}, nil, ""},
		{"folder nested below status", "sync/done/sub", []string{todo}, nil, ""},
		{"sync board root", "sync", []string{todo}, nil, ""},
		{"no foldersync", "plain/done", []string{todo}, nil, ""},
		{"unknown tag", "other", []string{prefix + "-foo"}, []IssueKind{IssueUnknownTag}, ""},
		{"unknown tag and status", "plain", []string{prefix + "-foo", todo, done}, []IssueKind{IssueUnknownTag, IssueMultipleStatus}, "todo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := fileIssues("x.md", tc.dir, tc.tags)
			var kinds []IssueKind
			fix := ""
			for _, i := range issues {
				kinds = append(kinds, i.Kind)
				if i.Fix != "" {
					fix = i.Fix
				}
			}
			if !slices.Equal(kinds, tc.kinds) || fix != tc.fix {
				t.Errorf("got %+v, want kinds %v with fix %q", issues, tc.kinds, tc.fix)
			}
		})
	}
}
