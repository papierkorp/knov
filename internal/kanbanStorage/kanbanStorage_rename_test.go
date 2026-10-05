package kanbanStorage

import "testing"

func TestRenameStatus(t *testing.T) {
	jsonStorage, err := newJSONStorageAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sqliteStorage, err := newSQLiteStorageAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqliteStorage.db.Close() })

	for _, s := range []KanbanStorage{jsonStorage, sqliteStorage} {
		t.Run(s.GetBackendType(), func(t *testing.T) {
			for _, e := range [][2]string{{"", "inbox"}, {"inbox", "done"}, {"done", "inbox"}, {"inbox", "inbox"}, {"done", "blocked"}} {
				if err := s.LogEvent("a.md", "board", e[0], e[1]); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.RenameStatus("inbox", "todo"); err != nil {
				t.Fatal(err)
			}
			events, err := s.GetEvents("", "", nil, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := map[[2]string]bool{}
			for _, e := range events {
				got[[2]string{e.FromStatus, e.ToStatus}] = true
			}
			for _, want := range [][2]string{{"", "todo"}, {"todo", "done"}, {"done", "todo"}, {"todo", "todo"}, {"done", "blocked"}} {
				if !got[want] {
					t.Errorf("missing event %v in %v", want, got)
				}
			}
			if len(events) != 5 {
				t.Errorf("got %d events, want 5", len(events))
			}
		})
	}
}
